-- Add missing columns to rider_applications
ALTER TABLE public.rider_applications 
ADD COLUMN IF NOT EXISTS gender text,
ADD COLUMN IF NOT EXISTS hometown text,
ADD COLUMN IF NOT EXISTS current_address text,
ADD COLUMN IF NOT EXISTS id_card_number text,
ADD COLUMN IF NOT EXISTS id_card_validity text,
ADD COLUMN IF NOT EXISTS address text,
ADD COLUMN IF NOT EXISTS address_detail text,
ADD COLUMN IF NOT EXISTS latitude numeric(10,7),
ADD COLUMN IF NOT EXISTS longitude numeric(10,7),
ADD COLUMN IF NOT EXISTS vehicle_type text,
ADD COLUMN IF NOT EXISTS available_time text;

-- Add missing columns to riders
ALTER TABLE public.riders
ADD COLUMN IF NOT EXISTS gender text,
ADD COLUMN IF NOT EXISTS hometown text,
ADD COLUMN IF NOT EXISTS current_address text,
ADD COLUMN IF NOT EXISTS vehicle_type text,
ADD COLUMN IF NOT EXISTS available_time text;

-- Update submit_rider_application RPC to handle new fields
CREATE OR REPLACE FUNCTION public.submit_rider_application(app_id uuid)
RETURNS jsonb AS $$
DECLARE
    app public.rider_applications%ROWTYPE;
    id_ocr jsonb;
    health_ocr jsonb;
    is_approved boolean := true;
    reject_reason text := '';
    rider_name text;
    rider_id_num text;
    health_name text;
    health_id_num text;
    expire_date date;
    new_rider_id uuid;
BEGIN
    -- 1. 获取申请单
    SELECT * INTO app FROM public.rider_applications WHERE id = app_id;
    IF NOT FOUND THEN RETURN jsonb_build_object('success', false, 'error', '申请不存在'); END IF;
    
    -- 2. 获取 OCR 数据
    id_ocr := app.id_card_ocr;
    health_ocr := app.health_cert_ocr;

    -- 3. 自动审核逻辑
    
    -- 规则 A: 完整性 (基础图片 + OCR 数据)
    IF app.id_card_front_url IS NULL OR app.id_card_back_url IS NULL OR app.health_cert_url IS NULL THEN
        is_approved := false;
        reject_reason := '证照图片不全';
    ELSIF id_ocr IS NULL OR id_ocr->>'status' != 'done' OR health_ocr IS NULL OR health_ocr->>'status' != 'done' THEN
        is_approved := false;
        reject_reason := '证照识别尚未完成或不全，请确认识别成功后再提交';
    END IF;

    -- 规则 B: 身份证有效期 (优先使用 OCR 数据，因为它是原始凭证)
    IF is_approved THEN
        expire_date := public.parse_id_card_expire_date(id_ocr->>'valid_end');
        IF expire_date IS NULL OR expire_date < CURRENT_DATE THEN
            is_approved := false;
            reject_reason := '身份证已过期或有效期无法识别';
        END IF;
    END IF;

    -- 规则 C: 健康证一致性 (姓名+证号)
    IF is_approved THEN
        rider_name := public.clean_id_number(id_ocr->>'name');
        rider_id_num := public.clean_id_number(id_ocr->>'id_number');
        health_name := public.clean_id_number(health_ocr->>'name');
        health_id_num := public.clean_id_number(health_ocr->>'id_number');

        -- 允许用户修正后的 real_name 与 OCR 匹配 (如果 OCR 姓名在 real_name 中或相等)
        -- 但为了严格，通常要求 OCR 必须正确。这里我们暂时保持原有的 OCR 强校验。
        IF rider_name = '' OR health_name = '' OR rider_name != health_name THEN
            is_approved := false;
            reject_reason := '健康证姓名与身份证不匹配';
        ELSIF rider_id_num = '' OR health_id_num = '' OR rider_id_num != health_id_num THEN
            is_approved := false;
            reject_reason := '健康证身份证号与身份证不匹配';
        END IF;
    END IF;

    -- 规则 D: 健康证有效期
    IF is_approved THEN
        DECLARE
            h_parsed_date date;
        BEGIN
            IF health_ocr->>'valid_end' ~ '长期|永久' THEN
                -- OK
            ELSE
                h_parsed_date := (regexp_replace(health_ocr->>'valid_end', '(\d{4})年(\d{1,2})月(\d{1,2})日', '\1-\2-\3'))::date;
                IF h_parsed_date < (CURRENT_DATE + 7) THEN
                    is_approved := false;
                    reject_reason := '健康证有效期需超过今日7天';
                END IF;
            END IF;
        EXCEPTION WHEN OTHERS THEN
            is_approved := false;
            reject_reason := '健康证有效期识别失败';
        END;
    END IF;

    -- 4. 处理结果
    IF is_approved THEN
        -- 更新状态
        UPDATE public.rider_applications SET 
            status = 'approved', 
            updated_at = now(), 
            submitted_at = now() 
        WHERE id = app_id;
        
        -- 创建/更新骑手记录
        INSERT INTO public.riders (
            user_id, 
            real_name, 
            id_card_no, 
            phone, 
            status,
            gender,
            hometown,
            current_address,
            vehicle_type,
            available_time,
            current_longitude,
            current_latitude,
            application_id
        ) VALUES (
            app.user_id, 
            COALESCE(app.real_name, rider_name), 
            COALESCE(app.id_card_number, rider_id_num), 
            app.phone, 
            'approved',
            app.gender,
            app.hometown,
            app.current_address,
            app.vehicle_type,
            app.available_time,
            app.longitude,
            app.latitude,
            app.id
        )
        ON CONFLICT (user_id) DO UPDATE SET
            real_name = EXCLUDED.real_name,
            id_card_no = EXCLUDED.id_card_no,
            phone = EXCLUDED.phone,
            status = 'approved',
            gender = EXCLUDED.gender,
            hometown = EXCLUDED.hometown,
            current_address = EXCLUDED.current_address,
            vehicle_type = EXCLUDED.vehicle_type,
            available_time = EXCLUDED.available_time,
            current_longitude = EXCLUDED.current_longitude,
            current_latitude = EXCLUDED.current_latitude,
            application_id = EXCLUDED.application_id,
            updated_at = now()
        RETURNING id INTO new_rider_id;

        -- 赋予/激活骑手角色
        INSERT INTO public.user_roles (user_id, role, status, related_entity_id)
        VALUES (app.user_id, 'rider', 'active', new_rider_id)
        ON CONFLICT (user_id, role) DO UPDATE SET 
            status = 'active', 
            related_entity_id = EXCLUDED.related_entity_id;

        RETURN jsonb_build_object('success', true, 'status', 'approved');
    ELSE
        -- 拒绝
        UPDATE public.rider_applications SET 
            status = 'rejected', 
            reject_reason = reject_reason, 
            updated_at = now(), 
            submitted_at = now() 
        WHERE id = app_id;
        RETURN jsonb_build_object('success', true, 'status', 'rejected', 'reason', reject_reason);
    END IF;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;
