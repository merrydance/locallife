-- 辅助函数：清洗身份证号（去空格转大写）
CREATE OR REPLACE FUNCTION public.clean_id_number(val text)
RETURNS text AS $$
BEGIN
    RETURN upper(trim(val));
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- 辅助函数：解析身份证有效期末尾日期 (格式: 20200101-20300101 或 20300101)
CREATE OR REPLACE FUNCTION public.parse_id_card_expire_date(val text)
RETURNS date AS $$
DECLARE
    clean_val text;
BEGIN
    IF val IS NULL OR val = '' THEN RETURN NULL; END IF;
    IF val ~ '长期|永久' THEN RETURN '9999-12-31'::date; END IF;
    
    -- 取最后 8 位
    clean_val := substring(val from length(val)-7);
    IF clean_val ~ '^\d{8}$' THEN
        RETURN to_date(clean_val, 'YYYYMMDD');
    END IF;
    RETURN NULL;
EXCEPTION WHEN OTHERS THEN
    RETURN NULL;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- 核心函数：提交骑手申请并执行自动规则引擎
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
    
    -- 规则 A: 完整性
    IF id_ocr IS NULL OR id_ocr->>'status' != 'done' OR health_ocr IS NULL OR health_ocr->>'status' != 'done' THEN
        is_approved := false;
        reject_reason := '证照识别尚未完成或不全，请确认识别成功后再提交';
    END IF;

    -- 规则 B: 身份证有效期
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

        IF rider_name = '' OR health_name = '' OR rider_name != health_name THEN
            is_approved := false;
            reject_reason := '健康证姓名与身份证不匹配';
        ELSIF rider_id_num = '' OR health_id_num = '' OR rider_id_num != health_id_num THEN
            is_approved := false;
            reject_reason := '健康证身份证号与身份证不匹配';
        END IF;
    END IF;

    -- 规则 D: 健康证有效期 (剩余需 >7 天)
    IF is_approved THEN
        -- 使用之前创建的 is_date_valid (假设它在 public 模式)
        -- 注意：is_date_valid 目前仅返回 boolean，我们需要校验是否 > CURRENT_DATE + 7
        -- 这里直接手写一个针对健康证日期的逻辑
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
        UPDATE public.rider_applications SET status = 'approved', updated_at = now(), submitted_at = now() WHERE id = app_id;
        
        -- 创建/更新骑手记录 (事务性)
        INSERT INTO public.riders (
            user_id, real_name, id_card_no, phone, status
        ) VALUES (
            app.user_id, rider_name, rider_id_num, app.phone, 'approved'
        )
        ON CONFLICT (user_id) DO UPDATE SET
            real_name = EXCLUDED.real_name,
            id_card_no = EXCLUDED.id_card_no,
            phone = EXCLUDED.phone,
            status = 'approved'
        RETURNING id INTO new_rider_id;

        -- 赋予/激活骑手角色
        INSERT INTO public.user_roles (user_id, role, status, related_entity_id)
        VALUES (app.user_id, 'rider', 'active', new_rider_id)
        ON CONFLICT (user_id, role) DO UPDATE SET status = 'active', related_entity_id = EXCLUDED.related_entity_id;

        RETURN jsonb_build_object('success', true, 'status', 'approved');
    ELSE
        -- 拒绝
        UPDATE public.rider_applications SET status = 'rejected', reject_reason = reject_reason, updated_at = now(), submitted_at = now() WHERE id = app_id;
        RETURN jsonb_build_object('success', true, 'status', 'rejected', 'reason', reject_reason);
    END IF;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;


-- 5. Webhook 触发器 (自动触发 OCR)
CREATE OR REPLACE FUNCTION public.tr_on_rider_application_image_update()
RETURNS TRIGGER AS $$
BEGIN
  -- 身份证正面
  IF NEW.id_card_front_url IS DISTINCT FROM OLD.id_card_front_url AND NEW.id_card_front_url IS NOT NULL THEN
    PERFORM net.http_post(
      url := current_setting('app.settings.edge_function_url') || '/ocr-service',
      headers := jsonb_build_object('Content-Type', 'application/json', 'Authorization', 'Bearer ' || current_setting('app.settings.service_role_key')),
      body := jsonb_build_object(
        'application_id', NEW.id,
        'image_url', NEW.id_card_front_url,
        'type', 'id_card',
        'side', 'Front',
        'target_table', 'rider_applications'
      )
    );
  END IF;

  -- 身份证背面
  IF NEW.id_card_back_url IS DISTINCT FROM OLD.id_card_back_url AND NEW.id_card_back_url IS NOT NULL THEN
    PERFORM net.http_post(
      url := current_setting('app.settings.edge_function_url') || '/ocr-service',
      headers := jsonb_build_object('Content-Type', 'application/json', 'Authorization', 'Bearer ' || current_setting('app.settings.service_role_key')),
      body := jsonb_build_object(
        'application_id', NEW.id,
        'image_url', NEW.id_card_back_url,
        'type', 'id_card',
        'side', 'Back',
        'target_table', 'rider_applications'
      )
    );
  END IF;

  -- 健康证
  IF NEW.health_cert_url IS DISTINCT FROM OLD.health_cert_url AND NEW.health_cert_url IS NOT NULL THEN
    PERFORM net.http_post(
      url := current_setting('app.settings.edge_function_url') || '/ocr-service',
      headers := jsonb_build_object('Content-Type', 'application/json', 'Authorization', 'Bearer ' || current_setting('app.settings.service_role_key')),
      body := jsonb_build_object(
        'application_id', NEW.id,
        'image_url', NEW.health_cert_url,
        'type', 'health_cert',
        'target_table', 'rider_applications'
      )
    );
  END IF;

  RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 绑定触发器到 rider_applications 表
DROP TRIGGER IF EXISTS tr_rider_application_ocr ON public.rider_applications;
CREATE TRIGGER tr_rider_application_ocr
AFTER UPDATE ON public.rider_applications
FOR EACH ROW
WHEN (
    NEW.id_card_front_url IS DISTINCT FROM OLD.id_card_front_url OR
    NEW.id_card_back_url IS DISTINCT FROM OLD.id_card_back_url OR
    NEW.health_cert_url IS DISTINCT FROM OLD.health_cert_url
)
EXECUTE FUNCTION public.tr_on_rider_application_image_update();
