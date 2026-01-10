-- 辅助函数：提取地址中的路名/关键字（简化版地址匹配基础）
CREATE OR REPLACE FUNCTION public.extract_address_keywords(addr text)
RETURNS text[] AS $$
DECLARE
    keywords text[];
BEGIN
    -- 提取常见的路、街、道等
    SELECT array_agg(m[1]) INTO keywords
    FROM (
        SELECT regexp_matches(addr, '([^省市区县镇乡村街道]+(?:路|街|道|巷|弄|大街|大道|胡同))', 'g') as m
    ) t;
    RETURN keywords;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- 辅助函数：检查日期是否在有效期内 (格式: 2025年12月31日 或 长期)
CREATE OR REPLACE FUNCTION public.is_date_valid(date_str text)
RETURNS boolean AS $$
DECLARE
    parsed_date date;
BEGIN
    IF date_str IS NULL OR date_str = '' THEN RETURN false; END IF;
    IF date_str ~ '长期|永久' THEN RETURN true; END IF;
    
    -- 转换 2025年12月31日 为 2025-12-31
    BEGIN
        parsed_date := (regexp_replace(date_str, '(\d{4})年(\d{1,2})月(\d{1,2})日', '\1-\2-\3'))::date;
        RETURN parsed_date > CURRENT_DATE;
    EXCEPTION WHEN OTHERS THEN
        RETURN false;
    END;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- 辅助函数：尝试为经纬度自动匹配 RegionID
CREATE OR REPLACE FUNCTION public.match_region_id(lat float8, lon float8)
RETURNS bigint AS $$
DECLARE
    matched_id bigint;
BEGIN
    -- 寻找最近的区域（此处假设 regions 表有位置信息，或者简单根据坐标点关联）
    -- 作为一个示例逻辑，这里直接从现有商户中寻找最近的区域 ID
    SELECT region_id INTO matched_id
    FROM public.merchants
    ORDER BY (latitude - lat)^2 + (longitude - lon)^2 ASC
    LIMIT 1;
    RETURN matched_id;
END;
$$ LANGUAGE plpgsql;

-- 核心函数：提交商户入驻申请并执行自动审核
CREATE OR REPLACE FUNCTION public.submit_merchant_application(app_id bigint)
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
    new_merchant_id bigint;
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
        
        -- 创建/更新商户记录 (事务性)
        app_data := jsonb_build_object(
            'business_license_number', app.business_license_number,
            'legal_person_name', app.legal_person_name,
            'legal_person_id_number', app.legal_person_id_number,
            'business_license_image_url', app.business_license_image_url
        );

        INSERT INTO public.merchants (
            owner_user_id, name, phone, address, latitude, longitude, region_id, status, application_data
        ) VALUES (
            app.user_id, app.merchant_name, app.contact_phone, app.business_address, 
            app.latitude, app.longitude, app.region_id, 'approved', app_data
        )
        ON CONFLICT (owner_user_id) DO UPDATE SET
            name = EXCLUDED.name,
            phone = EXCLUDED.phone,
            address = EXCLUDED.address,
            latitude = EXCLUDED.latitude,
            longitude = EXCLUDED.longitude,
            region_id = EXCLUDED.region_id,
            status = 'approved',
            application_data = EXCLUDED.application_data
        RETURNING id INTO new_merchant_id;

        -- 赋予商户角色
        INSERT INTO public.user_roles (user_id, role, status, related_entity_id)
        VALUES (app.user_id, 'merchant', 'active', new_merchant_id)
        ON CONFLICT (user_id, role) DO UPDATE SET status = 'active', related_entity_id = EXCLUDED.related_entity_id;

        RETURN jsonb_build_object('success', true, 'status', 'approved');
    ELSE
        -- 修改状态为 rejected
        UPDATE public.merchant_applications SET status = 'rejected', reject_reason = reject_reason, updated_at = now() WHERE id = app_id;
        RETURN jsonb_build_object('success', true, 'status', 'rejected', 'reason', reject_reason);
    END IF;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;
