-- 1. 为商户申请表增加角色字段
ALTER TABLE public.merchant_applications ADD COLUMN IF NOT EXISTS onboarding_role text DEFAULT 'owner';
COMMENT ON COLUMN public.merchant_applications.onboarding_role IS '入驻时的角色：owner (店主), manager (店长)';

-- 2. 弃用并删除 merchant_bosses 表（合并到 merchant_staff）
DROP TABLE IF EXISTS public.merchant_bosses CASCADE;

-- 3. 确保 merchants 表允许一个 owner_user_id 有多个店铺（移除唯一索引）
-- 注意：我在之前的交互中已经手动执行过，但在迁移文件中包含它是为了环境一致性
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'merchants_owner_user_id_key' 
        AND conrelid = 'public.merchants'::regclass
    ) THEN
        ALTER TABLE public.merchants DROP CONSTRAINT merchants_owner_user_id_key;
    END IF;
END $$;

-- 4. 更新核心函数：提交商户入驻申请并执行自动审核
CREATE OR REPLACE FUNCTION public.submit_merchant_application(app_id uuid)
RETURNS jsonb AS $$
DECLARE
    app public.merchant_applications%ROWTYPE;
    license_ocr jsonb;
    food_ocr jsonb;
    id_front_ocr jsonb;
    id_back_ocr jsonb;
    reject_reason text := '';
    is_approved boolean := true;
    keywords text[];
    license_addr_keywords text[];
    common_keywords text[];
    app_data jsonb;
    new_merchant_id uuid;
BEGIN
    -- 1. 获取申请单
    SELECT * INTO app FROM public.merchant_applications WHERE id = app_id;
    IF NOT FOUND THEN RETURN jsonb_build_object('success', false, 'error', '申请不存在'); END IF;
    
    -- 2. 解析 OCR 数据
    license_ocr := app.business_license_ocr;
    food_ocr := app.food_permit_ocr;
    id_front_ocr := app.id_card_front_ocr;
    id_back_ocr := app.id_card_back_ocr;

    -- 3. 基础规则校验 (借鉴 Go 后端逻辑)
    
    -- 规则 A: OCR 完整性
    IF license_ocr->>'status' != 'done' OR food_ocr->>'status' != 'done' OR 
       id_front_ocr->>'status' != 'done' OR id_back_ocr->>'status' != 'done' THEN
        is_approved := false;
        reject_reason := '证照识别尚未完成，请稍后再试';
    END IF;

    -- 规则 B: 有效期校验
    IF is_approved AND NOT public.is_date_valid(license_ocr->>'valid_period') THEN
        is_approved := false;
        reject_reason := '营业执照已过期';
    END IF;
    IF is_approved AND NOT public.is_date_valid(food_ocr->>'valid_to') THEN
        is_approved := false;
        reject_reason := '食品经营许可证已过期';
    END IF;
    IF is_approved AND NOT public.is_date_valid(id_back_ocr->>'valid_date') THEN
        is_approved := false;
        reject_reason := '法人身份证已过期';
    END IF;

    -- 规则 C: 企业名称一致性 (营业执照 vs 食品许可证)
    IF is_approved AND (license_ocr->>'enterprise_name' != food_ocr->>'company_name') THEN
        is_approved := false;
        reject_reason := '食品经营许可证企业名称与营业执照不一致';
    END IF;

    -- 规则 D: 法人一致性 (营业执照 vs 身份证)
    IF is_approved AND (license_ocr->>'legal_representative' != id_front_ocr->>'name') THEN
        is_approved := false;
        reject_reason := '身份证姓名与营业执照法人不一致';
    END IF;

    -- 规则 E: 地址匹配 (路名匹配)
    IF is_approved THEN
        license_addr_keywords := public.extract_address_keywords(license_ocr->>'address');
        common_keywords := ARRAY(
            SELECT unnest(license_addr_keywords) 
            INTERSECT 
            SELECT unnest(public.extract_address_keywords(app.business_address))
        );
        IF array_length(common_keywords, 1) IS NULL THEN
            is_approved := false;
            reject_reason := '营业执照地址与商户地址不匹配';
        END IF;
    END IF;

    -- 4. 处理审核结果
    IF is_approved THEN
        -- 修改状态为 approved
        UPDATE public.merchant_applications SET status = 'approved', updated_at = now() WHERE id = app_id;
        
        -- 分解上传数据回填到 merchants 表
        app_data := jsonb_build_object(
            'business_license_number', app.business_license_number,
            'legal_person_name', app.legal_person_name,
            'legal_person_id_number', app.legal_person_id_number,
            'business_license_image_url', app.business_license_image_url
        );

        -- 如果已经设置了 target_merchant_id (更新已有商户)，则执行 UPDATE，否则执行 INSERT
        IF app.target_merchant_id IS NOT NULL THEN
            UPDATE public.merchants SET
                name = app.merchant_name,
                phone = app.contact_phone,
                address = app.business_address,
                latitude = app.latitude,
                longitude = app.longitude,
                region_id = app.region_id,
                status = 'approved',
                application_data = app_data,
                updated_at = now()
            WHERE id = app.target_merchant_id
            RETURNING id INTO new_merchant_id;
        ELSE
            -- 创建新商户（支持多店，不使用 ON CONFLICT）
            INSERT INTO public.merchants (
                owner_user_id, name, phone, address, latitude, longitude, region_id, status, application_data
            ) VALUES (
                app.user_id, app.merchant_name, app.contact_phone, app.business_address, 
                app.latitude, app.longitude, app.region_id, 'approved', app_data
            )
            RETURNING id INTO new_merchant_id;
        END IF;

        -- 赋予平台商户角色 (通用入口)
        INSERT INTO public.user_roles (user_id, role, status, related_entity_id)
        VALUES (app.user_id, 'merchant', 'active', new_merchant_id)
        ON CONFLICT (user_id, role) DO UPDATE SET status = 'active', related_entity_id = EXCLUDED.related_entity_id;

        -- 绑定店铺具体成员角色 (RBAC)
        INSERT INTO public.merchant_staff (merchant_id, user_id, role, status)
        VALUES (new_merchant_id, app.user_id, app.onboarding_role, 'active')
        ON CONFLICT (merchant_id, user_id) DO UPDATE SET role = EXCLUDED.role, status = 'active';

        RETURN jsonb_build_object('success', true, 'status', 'approved', 'merchant_id', new_merchant_id);
    ELSE
        -- 修改状态为 rejected
        UPDATE public.merchant_applications SET status = 'rejected', reject_reason = reject_reason, updated_at = now() WHERE id = app_id;
        RETURN jsonb_build_object('success', true, 'status', 'rejected', 'reason', reject_reason);
    END IF;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;
