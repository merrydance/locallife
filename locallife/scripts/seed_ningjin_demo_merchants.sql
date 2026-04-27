-- 宁晋示例商户 seed
--
-- 用途：为宁晋县范围准备 3 家可展示的演示商户、证照、门头图和菜品。
-- 设计目标：
-- 1. 可重复执行；
-- 2. 不依赖外部地理编码服务；
-- 3. 门头图沿用 merchant_applications.storefront_images 的旧公开路径；
-- 4. 证照图和菜品图走 media_assets，方便前台正常解析 CDN URL。
--
-- 你需要按下面这些 object_key 上传图片到 OSS：
--
-- 店铺 1：宁晋县饺好小吃店
--   门头：uploads/merchants/ningjin-jiaohao-snack/storefront/cover.jpg
--   营业执照：uploads/public/merchants/ningjin-jiaohao-snack/licenses/business-license.jpg
--   食品证：uploads/public/merchants/ningjin-jiaohao-snack/licenses/food-permit.jpg
--   菜品图：
--     uploads/public/merchants/ningjin-jiaohao-snack/dishes/dish-1.jpg
--     uploads/public/merchants/ningjin-jiaohao-snack/dishes/dish-2.jpg
--     uploads/public/merchants/ningjin-jiaohao-snack/dishes/dish-3.jpg
--     uploads/public/merchants/ningjin-jiaohao-snack/dishes/dish-4.jpg
--
-- 店铺 2：宁晋县周鹏饭店
--   门头：uploads/merchants/ningjin-zhoupeng-restaurant/storefront/cover.jpg
--   营业执照：uploads/public/merchants/ningjin-zhoupeng-restaurant/licenses/business-license.jpg
--   食品证：uploads/public/merchants/ningjin-zhoupeng-restaurant/licenses/food-permit.jpg
--   菜品图：
--     uploads/public/merchants/ningjin-zhoupeng-restaurant/dishes/dish-1.jpg
--     uploads/public/merchants/ningjin-zhoupeng-restaurant/dishes/dish-2.jpg
--     uploads/public/merchants/ningjin-zhoupeng-restaurant/dishes/dish-3.jpg
--     uploads/public/merchants/ningjin-zhoupeng-restaurant/dishes/dish-4.jpg
--
-- 店铺 3：宁晋县奇岩饭店
--   门头：uploads/merchants/ningjin-qiyan-restaurant/storefront/cover.jpg
--   营业执照：uploads/public/merchants/ningjin-qiyan-restaurant/licenses/business-license.jpg
--   食品证：uploads/public/merchants/ningjin-qiyan-restaurant/licenses/food-permit.jpg
--   菜品图：
--     uploads/public/merchants/ningjin-qiyan-restaurant/dishes/dish-1.jpg
--     uploads/public/merchants/ningjin-qiyan-restaurant/dishes/dish-2.jpg
--     uploads/public/merchants/ningjin-qiyan-restaurant/dishes/dish-3.jpg
--     uploads/public/merchants/ningjin-qiyan-restaurant/dishes/dish-4.jpg

DO $$
DECLARE
    ningjin_region_id BIGINT;
    ningjin_base_lat NUMERIC(10, 7);
    ningjin_base_lng NUMERIC(10, 7);
    shared_category_id BIGINT;

    store_owner_id BIGINT;
    store_app_id BIGINT;
    store_merchant_id BIGINT;
    store_business_license_asset_id BIGINT;
    store_food_permit_asset_id BIGINT;
    store_dish_asset_id BIGINT;
    store_dish_id BIGINT;

    store_name TEXT;
    store_slug TEXT;
    store_full_address TEXT;
    store_contact_phone TEXT;
    store_owner_name TEXT;
    store_openid TEXT;
    store_license_no TEXT;
    store_food_permit_no TEXT;
    store_lat NUMERIC(10, 7);
    store_lng NUMERIC(10, 7);
    store_storefront_key TEXT;
    store_application_payload JSONB;

    dish_name TEXT;
    dish_description TEXT;
    dish_price BIGINT;
    dish_member_price BIGINT;
    dish_sort_order SMALLINT;
    dish_prepare_time SMALLINT;
    dish_key TEXT;

    active_ledger_id BIGINT;
BEGIN
    SELECT r.id,
           COALESCE(r.latitude, 37.6175000),
           COALESCE(r.longitude, 114.9195000)
    INTO ningjin_region_id, ningjin_base_lat, ningjin_base_lng
    FROM regions r
    WHERE r.name IN ('宁晋县', '宁晋')
      AND COALESCE(r.status, 'active') = 'active'
    ORDER BY CASE WHEN r.name = '宁晋县' THEN 0 ELSE 1 END, r.id DESC
    LIMIT 1;

    IF ningjin_region_id IS NULL THEN
        RAISE EXCEPTION 'seed_ningjin_demo_merchants: 未找到宁晋县 region，请先确认 regions 表已导入宁晋区县数据';
    END IF;

    INSERT INTO dish_categories (name)
    VALUES ('招牌推荐')
    ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
    RETURNING id INTO shared_category_id;

    -- ==================== 店铺 1：宁晋县饺好小吃店 ====================
    store_name := '宁晋县饺好小吃店';
    store_slug := 'ningjin-jiaohao-snack';
    store_owner_name := '饺好小吃店老板';
    store_openid := 'seed_ningjin_jiaohao_owner';
    store_full_address := '河北省邢台市宁晋县吉祥路133号';
    store_contact_phone := '15030990031';
    store_license_no := '91130528SEEDJH001';
    store_food_permit_no := 'JY21305280000031';
    store_lat := ningjin_base_lat + 0.0012;
    store_lng := ningjin_base_lng + 0.0018;
    store_storefront_key := 'uploads/merchants/ningjin-jiaohao-snack/storefront/cover.jpg';

    INSERT INTO users (wechat_openid, wechat_unionid, full_name, phone, avatar_url)
    VALUES (store_openid, NULL, store_owner_name, NULL, NULL)
    ON CONFLICT (wechat_openid) DO UPDATE
    SET full_name = EXCLUDED.full_name
    RETURNING id INTO store_owner_id;

    INSERT INTO media_assets (
        object_key, visibility, media_category, mime_type, file_size,
        checksum_sha256, upload_status, moderation_status, uploaded_by, source_client
    )
    VALUES (
        'uploads/public/merchants/ningjin-jiaohao-snack/licenses/business-license.jpg',
        'public', 'business_license', 'image/jpeg', 1024,
        md5('uploads/public/merchants/ningjin-jiaohao-snack/licenses/business-license.jpg') || md5('uploads/public/merchants/ningjin-jiaohao-snack/licenses/business-license.jpg'),
        'confirmed', 'approved', store_owner_id, 'server'
    )
    ON CONFLICT (object_key) DO UPDATE SET
        visibility = EXCLUDED.visibility,
        media_category = EXCLUDED.media_category,
        mime_type = EXCLUDED.mime_type,
        file_size = EXCLUDED.file_size,
        checksum_sha256 = EXCLUDED.checksum_sha256,
        upload_status = EXCLUDED.upload_status,
        moderation_status = EXCLUDED.moderation_status,
        uploaded_by = EXCLUDED.uploaded_by,
        source_client = EXCLUDED.source_client,
        deleted_at = NULL,
        updated_at = now()
    RETURNING id INTO store_business_license_asset_id;

    INSERT INTO media_assets (
        object_key, visibility, media_category, mime_type, file_size,
        checksum_sha256, upload_status, moderation_status, uploaded_by, source_client
    )
    VALUES (
        'uploads/public/merchants/ningjin-jiaohao-snack/licenses/food-permit.jpg',
        'public', 'food_permit', 'image/jpeg', 1024,
        md5('uploads/public/merchants/ningjin-jiaohao-snack/licenses/food-permit.jpg') || md5('uploads/public/merchants/ningjin-jiaohao-snack/licenses/food-permit.jpg'),
        'confirmed', 'approved', store_owner_id, 'server'
    )
    ON CONFLICT (object_key) DO UPDATE SET
        visibility = EXCLUDED.visibility,
        media_category = EXCLUDED.media_category,
        mime_type = EXCLUDED.mime_type,
        file_size = EXCLUDED.file_size,
        checksum_sha256 = EXCLUDED.checksum_sha256,
        upload_status = EXCLUDED.upload_status,
        moderation_status = EXCLUDED.moderation_status,
        uploaded_by = EXCLUDED.uploaded_by,
        source_client = EXCLUDED.source_client,
        deleted_at = NULL,
        updated_at = now()
    RETURNING id INTO store_food_permit_asset_id;

    SELECT ma.id
    INTO store_app_id
    FROM merchant_applications ma
    WHERE ma.user_id = store_owner_id
      AND ma.merchant_name = store_name
    ORDER BY ma.id DESC
    LIMIT 1;

    IF store_app_id IS NULL THEN
        INSERT INTO merchant_applications (
            user_id, merchant_name, business_license_number, legal_person_name,
            legal_person_id_number, contact_phone, business_address, business_scope,
            status, longitude, latitude, region_id, storefront_images,
            business_license_media_asset_id, food_permit_media_asset_id
        )
        VALUES (
            store_owner_id, store_name, store_license_no, '张三',
            '132229198801010031', store_contact_phone, store_full_address, '小吃服务',
            'approved', store_lng, store_lat, ningjin_region_id, jsonb_build_array(store_storefront_key),
            store_business_license_asset_id, store_food_permit_asset_id
        )
        RETURNING id INTO store_app_id;
    ELSE
        UPDATE merchant_applications
        SET merchant_name = store_name,
            business_license_number = store_license_no,
            legal_person_name = '张三',
            legal_person_id_number = '132229198801010031',
            contact_phone = store_contact_phone,
            business_address = store_full_address,
            business_scope = '小吃服务',
            status = 'approved',
            longitude = store_lng,
            latitude = store_lat,
            region_id = ningjin_region_id,
            storefront_images = jsonb_build_array(store_storefront_key),
            business_license_media_asset_id = store_business_license_asset_id,
            food_permit_media_asset_id = store_food_permit_asset_id,
            updated_at = now()
        WHERE id = store_app_id;
    END IF;

    store_application_payload := jsonb_build_object(
        'seed', 'ningjin-demo',
        'storefront_images', jsonb_build_array(store_storefront_key),
        'business_license_media_asset_id', store_business_license_asset_id,
        'food_permit_media_asset_id', store_food_permit_asset_id
    );

    SELECT m.id
    INTO store_merchant_id
    FROM merchants m
    WHERE m.owner_user_id = store_owner_id
      AND m.name = store_name
      AND m.deleted_at IS NULL
    ORDER BY m.id DESC
    LIMIT 1;

    IF store_merchant_id IS NULL THEN
        INSERT INTO merchants (
            owner_user_id, name, description, phone, address,
            latitude, longitude, status, application_data, region_id
        )
        VALUES (
            store_owner_id, store_name, '宁晋演示店铺，请勿下单', store_contact_phone, store_full_address,
            store_lat, store_lng, 'active', store_application_payload, ningjin_region_id
        )
        RETURNING id INTO store_merchant_id;
    ELSE
        UPDATE merchants
        SET name = store_name,
            description = '宁晋演示店铺，请勿下单',
            phone = store_contact_phone,
            address = store_full_address,
            latitude = store_lat,
            longitude = store_lng,
            status = 'active',
            application_data = store_application_payload,
            region_id = ningjin_region_id,
            updated_at = now(),
            deleted_at = NULL
        WHERE id = store_merchant_id;
    END IF;

    INSERT INTO user_roles (user_id, role, status, related_entity_id)
    VALUES (store_owner_id, 'merchant', 'active', store_merchant_id)
    ON CONFLICT (user_id, role) DO UPDATE SET
        status = EXCLUDED.status,
        related_entity_id = EXCLUDED.related_entity_id;

    INSERT INTO user_roles (user_id, role, status, related_entity_id)
    VALUES (store_owner_id, 'customer', 'active', NULL)
    ON CONFLICT (user_id, role) DO UPDATE SET
        status = EXCLUDED.status;

    INSERT INTO merchant_dish_categories (merchant_id, category_id, sort_order)
    VALUES (store_merchant_id, shared_category_id, 1)
    ON CONFLICT (merchant_id, category_id) DO UPDATE SET sort_order = EXCLUDED.sort_order;

    SELECT cl.id
    INTO active_ledger_id
    FROM credential_ledgers cl
    WHERE cl.merchant_id = store_merchant_id
      AND cl.document_type = 'business_license'
      AND cl.active = true
    ORDER BY cl.id DESC
    LIMIT 1;

    IF active_ledger_id IS NULL THEN
        INSERT INTO credential_ledgers (
            subject_type, merchant_id, document_type, merchant_application_id,
            review_run_id, media_asset_id, normalized_payload, expires_at,
            active, activated_at
        )
        VALUES (
            'merchant', store_merchant_id, 'business_license', store_app_id,
            NULL, store_business_license_asset_id,
            jsonb_build_object('company_name', store_name, 'license_no', store_license_no),
            '2030-12-31 23:59:59+08', true, now()
        );
    ELSE
        UPDATE credential_ledgers
        SET merchant_application_id = store_app_id,
            media_asset_id = store_business_license_asset_id,
            normalized_payload = jsonb_build_object('company_name', store_name, 'license_no', store_license_no),
            expires_at = '2030-12-31 23:59:59+08',
            deactivated_at = NULL,
            suspended_at = NULL,
            resumed_at = NULL,
            suspension_reason_code = NULL,
            updated_at = now()
        WHERE id = active_ledger_id;
    END IF;

    SELECT cl.id
    INTO active_ledger_id
    FROM credential_ledgers cl
    WHERE cl.merchant_id = store_merchant_id
      AND cl.document_type = 'food_permit'
      AND cl.active = true
    ORDER BY cl.id DESC
    LIMIT 1;

    IF active_ledger_id IS NULL THEN
        INSERT INTO credential_ledgers (
            subject_type, merchant_id, document_type, merchant_application_id,
            review_run_id, media_asset_id, normalized_payload, expires_at,
            active, activated_at
        )
        VALUES (
            'merchant', store_merchant_id, 'food_permit', store_app_id,
            NULL, store_food_permit_asset_id,
            jsonb_build_object('company_name', store_name, 'permit_no', store_food_permit_no),
            '2030-12-31 23:59:59+08', true, now()
        );
    ELSE
        UPDATE credential_ledgers
        SET merchant_application_id = store_app_id,
            media_asset_id = store_food_permit_asset_id,
            normalized_payload = jsonb_build_object('company_name', store_name, 'permit_no', store_food_permit_no),
            expires_at = '2030-12-31 23:59:59+08',
            deactivated_at = NULL,
            suspended_at = NULL,
            resumed_at = NULL,
            suspension_reason_code = NULL,
            updated_at = now()
        WHERE id = active_ledger_id;
    END IF;

    FOR dish_name, dish_description, dish_price, dish_member_price, dish_sort_order, dish_prepare_time, dish_key IN
        SELECT *
        FROM (
            VALUES
                ('三鲜水饺', '招牌现包三鲜水饺', 2600::BIGINT, 2300::BIGINT, 1::SMALLINT, 12::SMALLINT, 'uploads/public/merchants/ningjin-jiaohao-snack/dishes/dish-1.jpg'::TEXT),
                ('猪肉大葱水饺', '经典猪肉大葱口味', 2400::BIGINT, 2100::BIGINT, 2::SMALLINT, 12::SMALLINT, 'uploads/public/merchants/ningjin-jiaohao-snack/dishes/dish-2.jpg'::TEXT),
                ('酸辣汤', '现熬酸辣汤', 800::BIGINT, 700::BIGINT, 3::SMALLINT, 6::SMALLINT, 'uploads/public/merchants/ningjin-jiaohao-snack/dishes/dish-3.jpg'::TEXT),
                ('凉拌黄瓜', '清爽开胃小凉菜', 1200::BIGINT, 1000::BIGINT, 4::SMALLINT, 5::SMALLINT, 'uploads/public/merchants/ningjin-jiaohao-snack/dishes/dish-4.jpg'::TEXT)
        ) AS dishes(name, description, price, member_price, sort_order, prepare_time, object_key)
    LOOP
        INSERT INTO media_assets (
            object_key, visibility, media_category, mime_type, file_size,
            checksum_sha256, upload_status, moderation_status, uploaded_by, source_client
        )
        VALUES (
            dish_key, 'public', 'dish', 'image/jpeg', 1024,
            md5(dish_key) || md5(dish_key), 'confirmed', 'approved', store_owner_id, 'server'
        )
        ON CONFLICT (object_key) DO UPDATE SET
            visibility = EXCLUDED.visibility,
            media_category = EXCLUDED.media_category,
            mime_type = EXCLUDED.mime_type,
            file_size = EXCLUDED.file_size,
            checksum_sha256 = EXCLUDED.checksum_sha256,
            upload_status = EXCLUDED.upload_status,
            moderation_status = EXCLUDED.moderation_status,
            uploaded_by = EXCLUDED.uploaded_by,
            source_client = EXCLUDED.source_client,
            deleted_at = NULL,
            updated_at = now()
        RETURNING id INTO store_dish_asset_id;

        SELECT d.id
        INTO store_dish_id
        FROM dishes d
        WHERE d.merchant_id = store_merchant_id
          AND d.name = dish_name
          AND d.deleted_at IS NULL
        ORDER BY d.id DESC
        LIMIT 1;

        IF store_dish_id IS NULL THEN
            INSERT INTO dishes (
                merchant_id, category_id, name, description, price, member_price,
                is_available, is_online, sort_order, created_at, updated_at,
                prepare_time, image_media_asset_id, is_packaging
            )
            VALUES (
                store_merchant_id, shared_category_id, dish_name, dish_description, dish_price, dish_member_price,
                true, true, dish_sort_order, now(), now(),
                dish_prepare_time, store_dish_asset_id, false
            );
        ELSE
            UPDATE dishes
            SET category_id = shared_category_id,
                description = dish_description,
                price = dish_price,
                member_price = dish_member_price,
                is_available = true,
                is_online = true,
                sort_order = dish_sort_order,
                prepare_time = dish_prepare_time,
                image_media_asset_id = store_dish_asset_id,
                is_packaging = false,
                updated_at = now(),
                deleted_at = NULL
            WHERE id = store_dish_id;
        END IF;
    END LOOP;

    -- ==================== 店铺 2：宁晋县周鹏饭店 ====================
    store_name := '宁晋县周鹏饭店';
    store_slug := 'ningjin-zhoupeng-restaurant';
    store_owner_name := '周鹏饭店老板';
    store_openid := 'seed_ningjin_zhoupeng_owner';
    store_full_address := '河北省邢台市宁晋县吉祥路与晶龙街交叉口东侧';
    store_contact_phone := '15030990032';
    store_license_no := '91130528SEEDZP002';
    store_food_permit_no := 'JY21305280000032';
    store_lat := ningjin_base_lat + 0.0004;
    store_lng := ningjin_base_lng - 0.0009;
    store_storefront_key := 'uploads/merchants/ningjin-zhoupeng-restaurant/storefront/cover.jpg';

    INSERT INTO users (wechat_openid, wechat_unionid, full_name, phone, avatar_url)
    VALUES (store_openid, NULL, store_owner_name, NULL, NULL)
    ON CONFLICT (wechat_openid) DO UPDATE
    SET full_name = EXCLUDED.full_name
    RETURNING id INTO store_owner_id;

    INSERT INTO media_assets (
        object_key, visibility, media_category, mime_type, file_size,
        checksum_sha256, upload_status, moderation_status, uploaded_by, source_client
    )
    VALUES (
        'uploads/public/merchants/ningjin-zhoupeng-restaurant/licenses/business-license.jpg',
        'public', 'business_license', 'image/jpeg', 1024,
        md5('uploads/public/merchants/ningjin-zhoupeng-restaurant/licenses/business-license.jpg') || md5('uploads/public/merchants/ningjin-zhoupeng-restaurant/licenses/business-license.jpg'),
        'confirmed', 'approved', store_owner_id, 'server'
    )
    ON CONFLICT (object_key) DO UPDATE SET
        visibility = EXCLUDED.visibility,
        media_category = EXCLUDED.media_category,
        mime_type = EXCLUDED.mime_type,
        file_size = EXCLUDED.file_size,
        checksum_sha256 = EXCLUDED.checksum_sha256,
        upload_status = EXCLUDED.upload_status,
        moderation_status = EXCLUDED.moderation_status,
        uploaded_by = EXCLUDED.uploaded_by,
        source_client = EXCLUDED.source_client,
        deleted_at = NULL,
        updated_at = now()
    RETURNING id INTO store_business_license_asset_id;

    INSERT INTO media_assets (
        object_key, visibility, media_category, mime_type, file_size,
        checksum_sha256, upload_status, moderation_status, uploaded_by, source_client
    )
    VALUES (
        'uploads/public/merchants/ningjin-zhoupeng-restaurant/licenses/food-permit.jpg',
        'public', 'food_permit', 'image/jpeg', 1024,
        md5('uploads/public/merchants/ningjin-zhoupeng-restaurant/licenses/food-permit.jpg') || md5('uploads/public/merchants/ningjin-zhoupeng-restaurant/licenses/food-permit.jpg'),
        'confirmed', 'approved', store_owner_id, 'server'
    )
    ON CONFLICT (object_key) DO UPDATE SET
        visibility = EXCLUDED.visibility,
        media_category = EXCLUDED.media_category,
        mime_type = EXCLUDED.mime_type,
        file_size = EXCLUDED.file_size,
        checksum_sha256 = EXCLUDED.checksum_sha256,
        upload_status = EXCLUDED.upload_status,
        moderation_status = EXCLUDED.moderation_status,
        uploaded_by = EXCLUDED.uploaded_by,
        source_client = EXCLUDED.source_client,
        deleted_at = NULL,
        updated_at = now()
    RETURNING id INTO store_food_permit_asset_id;

    SELECT ma.id
    INTO store_app_id
    FROM merchant_applications ma
    WHERE ma.user_id = store_owner_id
      AND ma.merchant_name = store_name
    ORDER BY ma.id DESC
    LIMIT 1;

    IF store_app_id IS NULL THEN
        INSERT INTO merchant_applications (
            user_id, merchant_name, business_license_number, legal_person_name,
            legal_person_id_number, contact_phone, business_address, business_scope,
            status, longitude, latitude, region_id, storefront_images,
            business_license_media_asset_id, food_permit_media_asset_id
        )
        VALUES (
            store_owner_id, store_name, store_license_no, '李四',
            '132229198901010032', store_contact_phone, store_full_address, '热食类食品制售',
            'approved', store_lng, store_lat, ningjin_region_id, jsonb_build_array(store_storefront_key),
            store_business_license_asset_id, store_food_permit_asset_id
        )
        RETURNING id INTO store_app_id;
    ELSE
        UPDATE merchant_applications
        SET merchant_name = store_name,
            business_license_number = store_license_no,
            legal_person_name = '李四',
            legal_person_id_number = '132229198901010032',
            contact_phone = store_contact_phone,
            business_address = store_full_address,
            business_scope = '热食类食品制售',
            status = 'approved',
            longitude = store_lng,
            latitude = store_lat,
            region_id = ningjin_region_id,
            storefront_images = jsonb_build_array(store_storefront_key),
            business_license_media_asset_id = store_business_license_asset_id,
            food_permit_media_asset_id = store_food_permit_asset_id,
            updated_at = now()
        WHERE id = store_app_id;
    END IF;

    store_application_payload := jsonb_build_object(
        'seed', 'ningjin-demo',
        'storefront_images', jsonb_build_array(store_storefront_key),
        'business_license_media_asset_id', store_business_license_asset_id,
        'food_permit_media_asset_id', store_food_permit_asset_id
    );

    SELECT m.id
    INTO store_merchant_id
    FROM merchants m
    WHERE m.owner_user_id = store_owner_id
      AND m.name = store_name
      AND m.deleted_at IS NULL
    ORDER BY m.id DESC
    LIMIT 1;

    IF store_merchant_id IS NULL THEN
        INSERT INTO merchants (
            owner_user_id, name, description, phone, address,
            latitude, longitude, status, application_data, region_id
        )
        VALUES (
            store_owner_id, store_name, '宁晋演示店铺，请勿下单', store_contact_phone, store_full_address,
            store_lat, store_lng, 'active', store_application_payload, ningjin_region_id
        )
        RETURNING id INTO store_merchant_id;
    ELSE
        UPDATE merchants
        SET name = store_name,
            description = '宁晋演示店铺，请勿下单',
            phone = store_contact_phone,
            address = store_full_address,
            latitude = store_lat,
            longitude = store_lng,
            status = 'active',
            application_data = store_application_payload,
            region_id = ningjin_region_id,
            updated_at = now(),
            deleted_at = NULL
        WHERE id = store_merchant_id;
    END IF;

    INSERT INTO user_roles (user_id, role, status, related_entity_id)
    VALUES (store_owner_id, 'merchant', 'active', store_merchant_id)
    ON CONFLICT (user_id, role) DO UPDATE SET
        status = EXCLUDED.status,
        related_entity_id = EXCLUDED.related_entity_id;

    INSERT INTO user_roles (user_id, role, status, related_entity_id)
    VALUES (store_owner_id, 'customer', 'active', NULL)
    ON CONFLICT (user_id, role) DO UPDATE SET
        status = EXCLUDED.status;

    INSERT INTO merchant_dish_categories (merchant_id, category_id, sort_order)
    VALUES (store_merchant_id, shared_category_id, 1)
    ON CONFLICT (merchant_id, category_id) DO UPDATE SET sort_order = EXCLUDED.sort_order;

    SELECT cl.id
    INTO active_ledger_id
    FROM credential_ledgers cl
    WHERE cl.merchant_id = store_merchant_id
      AND cl.document_type = 'business_license'
      AND cl.active = true
    ORDER BY cl.id DESC
    LIMIT 1;

    IF active_ledger_id IS NULL THEN
        INSERT INTO credential_ledgers (
            subject_type, merchant_id, document_type, merchant_application_id,
            review_run_id, media_asset_id, normalized_payload, expires_at,
            active, activated_at
        )
        VALUES (
            'merchant', store_merchant_id, 'business_license', store_app_id,
            NULL, store_business_license_asset_id,
            jsonb_build_object('company_name', store_name, 'license_no', store_license_no),
            '2030-12-31 23:59:59+08', true, now()
        );
    ELSE
        UPDATE credential_ledgers
        SET merchant_application_id = store_app_id,
            media_asset_id = store_business_license_asset_id,
            normalized_payload = jsonb_build_object('company_name', store_name, 'license_no', store_license_no),
            expires_at = '2030-12-31 23:59:59+08',
            deactivated_at = NULL,
            suspended_at = NULL,
            resumed_at = NULL,
            suspension_reason_code = NULL,
            updated_at = now()
        WHERE id = active_ledger_id;
    END IF;

    SELECT cl.id
    INTO active_ledger_id
    FROM credential_ledgers cl
    WHERE cl.merchant_id = store_merchant_id
      AND cl.document_type = 'food_permit'
      AND cl.active = true
    ORDER BY cl.id DESC
    LIMIT 1;

    IF active_ledger_id IS NULL THEN
        INSERT INTO credential_ledgers (
            subject_type, merchant_id, document_type, merchant_application_id,
            review_run_id, media_asset_id, normalized_payload, expires_at,
            active, activated_at
        )
        VALUES (
            'merchant', store_merchant_id, 'food_permit', store_app_id,
            NULL, store_food_permit_asset_id,
            jsonb_build_object('company_name', store_name, 'permit_no', store_food_permit_no),
            '2030-12-31 23:59:59+08', true, now()
        );
    ELSE
        UPDATE credential_ledgers
        SET merchant_application_id = store_app_id,
            media_asset_id = store_food_permit_asset_id,
            normalized_payload = jsonb_build_object('company_name', store_name, 'permit_no', store_food_permit_no),
            expires_at = '2030-12-31 23:59:59+08',
            deactivated_at = NULL,
            suspended_at = NULL,
            resumed_at = NULL,
            suspension_reason_code = NULL,
            updated_at = now()
        WHERE id = active_ledger_id;
    END IF;

    FOR dish_name, dish_description, dish_price, dish_member_price, dish_sort_order, dish_prepare_time, dish_key IN
        SELECT *
        FROM (
            VALUES
                ('尖椒肉丝盖饭', '招牌下饭盖饭', 2600::BIGINT, 2300::BIGINT, 1::SMALLINT, 10::SMALLINT, 'uploads/public/merchants/ningjin-zhoupeng-restaurant/dishes/dish-1.jpg'::TEXT),
                ('宫保鸡丁盖饭', '经典宫保鸡丁现炒', 2800::BIGINT, 2500::BIGINT, 2::SMALLINT, 12::SMALLINT, 'uploads/public/merchants/ningjin-zhoupeng-restaurant/dishes/dish-2.jpg'::TEXT),
                ('西红柿鸡蛋面', '家常热汤面', 1800::BIGINT, 1600::BIGINT, 3::SMALLINT, 8::SMALLINT, 'uploads/public/merchants/ningjin-zhoupeng-restaurant/dishes/dish-3.jpg'::TEXT),
                ('紫菜蛋花汤', '清爽配餐汤品', 700::BIGINT, 600::BIGINT, 4::SMALLINT, 5::SMALLINT, 'uploads/public/merchants/ningjin-zhoupeng-restaurant/dishes/dish-4.jpg'::TEXT)
        ) AS dishes(name, description, price, member_price, sort_order, prepare_time, object_key)
    LOOP
        INSERT INTO media_assets (
            object_key, visibility, media_category, mime_type, file_size,
            checksum_sha256, upload_status, moderation_status, uploaded_by, source_client
        )
        VALUES (
            dish_key, 'public', 'dish', 'image/jpeg', 1024,
            md5(dish_key) || md5(dish_key), 'confirmed', 'approved', store_owner_id, 'server'
        )
        ON CONFLICT (object_key) DO UPDATE SET
            visibility = EXCLUDED.visibility,
            media_category = EXCLUDED.media_category,
            mime_type = EXCLUDED.mime_type,
            file_size = EXCLUDED.file_size,
            checksum_sha256 = EXCLUDED.checksum_sha256,
            upload_status = EXCLUDED.upload_status,
            moderation_status = EXCLUDED.moderation_status,
            uploaded_by = EXCLUDED.uploaded_by,
            source_client = EXCLUDED.source_client,
            deleted_at = NULL,
            updated_at = now()
        RETURNING id INTO store_dish_asset_id;

        SELECT d.id
        INTO store_dish_id
        FROM dishes d
        WHERE d.merchant_id = store_merchant_id
          AND d.name = dish_name
          AND d.deleted_at IS NULL
        ORDER BY d.id DESC
        LIMIT 1;

        IF store_dish_id IS NULL THEN
            INSERT INTO dishes (
                merchant_id, category_id, name, description, price, member_price,
                is_available, is_online, sort_order, created_at, updated_at,
                prepare_time, image_media_asset_id, is_packaging
            )
            VALUES (
                store_merchant_id, shared_category_id, dish_name, dish_description, dish_price, dish_member_price,
                true, true, dish_sort_order, now(), now(),
                dish_prepare_time, store_dish_asset_id, false
            );
        ELSE
            UPDATE dishes
            SET category_id = shared_category_id,
                description = dish_description,
                price = dish_price,
                member_price = dish_member_price,
                is_available = true,
                is_online = true,
                sort_order = dish_sort_order,
                prepare_time = dish_prepare_time,
                image_media_asset_id = store_dish_asset_id,
                is_packaging = false,
                updated_at = now(),
                deleted_at = NULL
            WHERE id = store_dish_id;
        END IF;
    END LOOP;

    -- ==================== 店铺 3：宁晋县奇岩饭店 ====================
    store_name := '宁晋县奇岩饭店';
    store_slug := 'ningjin-qiyan-restaurant';
    store_owner_name := '奇岩饭店老板';
    store_openid := 'seed_ningjin_qiyan_owner';
    store_full_address := '河北省邢台市宁晋县天宝西街';
    store_contact_phone := '15030990033';
    store_license_no := '91130528SEEDQY003';
    store_food_permit_no := 'JY21305280000033';
    store_lat := ningjin_base_lat - 0.0011;
    store_lng := ningjin_base_lng + 0.0005;
    store_storefront_key := 'uploads/merchants/ningjin-qiyan-restaurant/storefront/cover.jpg';

    INSERT INTO users (wechat_openid, wechat_unionid, full_name, phone, avatar_url)
    VALUES (store_openid, NULL, store_owner_name, NULL, NULL)
    ON CONFLICT (wechat_openid) DO UPDATE
    SET full_name = EXCLUDED.full_name
    RETURNING id INTO store_owner_id;

    INSERT INTO media_assets (
        object_key, visibility, media_category, mime_type, file_size,
        checksum_sha256, upload_status, moderation_status, uploaded_by, source_client
    )
    VALUES (
        'uploads/public/merchants/ningjin-qiyan-restaurant/licenses/business-license.jpg',
        'public', 'business_license', 'image/jpeg', 1024,
        md5('uploads/public/merchants/ningjin-qiyan-restaurant/licenses/business-license.jpg') || md5('uploads/public/merchants/ningjin-qiyan-restaurant/licenses/business-license.jpg'),
        'confirmed', 'approved', store_owner_id, 'server'
    )
    ON CONFLICT (object_key) DO UPDATE SET
        visibility = EXCLUDED.visibility,
        media_category = EXCLUDED.media_category,
        mime_type = EXCLUDED.mime_type,
        file_size = EXCLUDED.file_size,
        checksum_sha256 = EXCLUDED.checksum_sha256,
        upload_status = EXCLUDED.upload_status,
        moderation_status = EXCLUDED.moderation_status,
        uploaded_by = EXCLUDED.uploaded_by,
        source_client = EXCLUDED.source_client,
        deleted_at = NULL,
        updated_at = now()
    RETURNING id INTO store_business_license_asset_id;

    INSERT INTO media_assets (
        object_key, visibility, media_category, mime_type, file_size,
        checksum_sha256, upload_status, moderation_status, uploaded_by, source_client
    )
    VALUES (
        'uploads/public/merchants/ningjin-qiyan-restaurant/licenses/food-permit.jpg',
        'public', 'food_permit', 'image/jpeg', 1024,
        md5('uploads/public/merchants/ningjin-qiyan-restaurant/licenses/food-permit.jpg') || md5('uploads/public/merchants/ningjin-qiyan-restaurant/licenses/food-permit.jpg'),
        'confirmed', 'approved', store_owner_id, 'server'
    )
    ON CONFLICT (object_key) DO UPDATE SET
        visibility = EXCLUDED.visibility,
        media_category = EXCLUDED.media_category,
        mime_type = EXCLUDED.mime_type,
        file_size = EXCLUDED.file_size,
        checksum_sha256 = EXCLUDED.checksum_sha256,
        upload_status = EXCLUDED.upload_status,
        moderation_status = EXCLUDED.moderation_status,
        uploaded_by = EXCLUDED.uploaded_by,
        source_client = EXCLUDED.source_client,
        deleted_at = NULL,
        updated_at = now()
    RETURNING id INTO store_food_permit_asset_id;

    SELECT ma.id
    INTO store_app_id
    FROM merchant_applications ma
    WHERE ma.user_id = store_owner_id
      AND ma.merchant_name = store_name
    ORDER BY ma.id DESC
    LIMIT 1;

    IF store_app_id IS NULL THEN
        INSERT INTO merchant_applications (
            user_id, merchant_name, business_license_number, legal_person_name,
            legal_person_id_number, contact_phone, business_address, business_scope,
            status, longitude, latitude, region_id, storefront_images,
            business_license_media_asset_id, food_permit_media_asset_id
        )
        VALUES (
            store_owner_id, store_name, store_license_no, '王五',
            '132229199001010033', store_contact_phone, store_full_address, '热食类食品制售',
            'approved', store_lng, store_lat, ningjin_region_id, jsonb_build_array(store_storefront_key),
            store_business_license_asset_id, store_food_permit_asset_id
        )
        RETURNING id INTO store_app_id;
    ELSE
        UPDATE merchant_applications
        SET merchant_name = store_name,
            business_license_number = store_license_no,
            legal_person_name = '王五',
            legal_person_id_number = '132229199001010033',
            contact_phone = store_contact_phone,
            business_address = store_full_address,
            business_scope = '热食类食品制售',
            status = 'approved',
            longitude = store_lng,
            latitude = store_lat,
            region_id = ningjin_region_id,
            storefront_images = jsonb_build_array(store_storefront_key),
            business_license_media_asset_id = store_business_license_asset_id,
            food_permit_media_asset_id = store_food_permit_asset_id,
            updated_at = now()
        WHERE id = store_app_id;
    END IF;

    store_application_payload := jsonb_build_object(
        'seed', 'ningjin-demo',
        'storefront_images', jsonb_build_array(store_storefront_key),
        'business_license_media_asset_id', store_business_license_asset_id,
        'food_permit_media_asset_id', store_food_permit_asset_id
    );

    SELECT m.id
    INTO store_merchant_id
    FROM merchants m
    WHERE m.owner_user_id = store_owner_id
      AND m.name = store_name
      AND m.deleted_at IS NULL
    ORDER BY m.id DESC
    LIMIT 1;

    IF store_merchant_id IS NULL THEN
        INSERT INTO merchants (
            owner_user_id, name, description, phone, address,
            latitude, longitude, status, application_data, region_id
        )
        VALUES (
            store_owner_id, store_name, '宁晋演示店铺，请勿下单', store_contact_phone, store_full_address,
            store_lat, store_lng, 'active', store_application_payload, ningjin_region_id
        )
        RETURNING id INTO store_merchant_id;
    ELSE
        UPDATE merchants
        SET name = store_name,
            description = '宁晋演示店铺，请勿下单',
            phone = store_contact_phone,
            address = store_full_address,
            latitude = store_lat,
            longitude = store_lng,
            status = 'active',
            application_data = store_application_payload,
            region_id = ningjin_region_id,
            updated_at = now(),
            deleted_at = NULL
        WHERE id = store_merchant_id;
    END IF;

    INSERT INTO user_roles (user_id, role, status, related_entity_id)
    VALUES (store_owner_id, 'merchant', 'active', store_merchant_id)
    ON CONFLICT (user_id, role) DO UPDATE SET
        status = EXCLUDED.status,
        related_entity_id = EXCLUDED.related_entity_id;

    INSERT INTO user_roles (user_id, role, status, related_entity_id)
    VALUES (store_owner_id, 'customer', 'active', NULL)
    ON CONFLICT (user_id, role) DO UPDATE SET
        status = EXCLUDED.status;

    INSERT INTO merchant_dish_categories (merchant_id, category_id, sort_order)
    VALUES (store_merchant_id, shared_category_id, 1)
    ON CONFLICT (merchant_id, category_id) DO UPDATE SET sort_order = EXCLUDED.sort_order;

    SELECT cl.id
    INTO active_ledger_id
    FROM credential_ledgers cl
    WHERE cl.merchant_id = store_merchant_id
      AND cl.document_type = 'business_license'
      AND cl.active = true
    ORDER BY cl.id DESC
    LIMIT 1;

    IF active_ledger_id IS NULL THEN
        INSERT INTO credential_ledgers (
            subject_type, merchant_id, document_type, merchant_application_id,
            review_run_id, media_asset_id, normalized_payload, expires_at,
            active, activated_at
        )
        VALUES (
            'merchant', store_merchant_id, 'business_license', store_app_id,
            NULL, store_business_license_asset_id,
            jsonb_build_object('company_name', store_name, 'license_no', store_license_no),
            '2030-12-31 23:59:59+08', true, now()
        );
    ELSE
        UPDATE credential_ledgers
        SET merchant_application_id = store_app_id,
            media_asset_id = store_business_license_asset_id,
            normalized_payload = jsonb_build_object('company_name', store_name, 'license_no', store_license_no),
            expires_at = '2030-12-31 23:59:59+08',
            deactivated_at = NULL,
            suspended_at = NULL,
            resumed_at = NULL,
            suspension_reason_code = NULL,
            updated_at = now()
        WHERE id = active_ledger_id;
    END IF;

    SELECT cl.id
    INTO active_ledger_id
    FROM credential_ledgers cl
    WHERE cl.merchant_id = store_merchant_id
      AND cl.document_type = 'food_permit'
      AND cl.active = true
    ORDER BY cl.id DESC
    LIMIT 1;

    IF active_ledger_id IS NULL THEN
        INSERT INTO credential_ledgers (
            subject_type, merchant_id, document_type, merchant_application_id,
            review_run_id, media_asset_id, normalized_payload, expires_at,
            active, activated_at
        )
        VALUES (
            'merchant', store_merchant_id, 'food_permit', store_app_id,
            NULL, store_food_permit_asset_id,
            jsonb_build_object('company_name', store_name, 'permit_no', store_food_permit_no),
            '2030-12-31 23:59:59+08', true, now()
        );
    ELSE
        UPDATE credential_ledgers
        SET merchant_application_id = store_app_id,
            media_asset_id = store_food_permit_asset_id,
            normalized_payload = jsonb_build_object('company_name', store_name, 'permit_no', store_food_permit_no),
            expires_at = '2030-12-31 23:59:59+08',
            deactivated_at = NULL,
            suspended_at = NULL,
            resumed_at = NULL,
            suspension_reason_code = NULL,
            updated_at = now()
        WHERE id = active_ledger_id;
    END IF;

    FOR dish_name, dish_description, dish_price, dish_member_price, dish_sort_order, dish_prepare_time, dish_key IN
        SELECT *
        FROM (
            VALUES
                ('红烧茄子盖饭', '下饭招牌盖饭', 2400::BIGINT, 2100::BIGINT, 1::SMALLINT, 10::SMALLINT, 'uploads/public/merchants/ningjin-qiyan-restaurant/dishes/dish-1.jpg'::TEXT),
                ('土豆牛肉盖饭', '酱香浓郁热卖盖饭', 3000::BIGINT, 2700::BIGINT, 2::SMALLINT, 12::SMALLINT, 'uploads/public/merchants/ningjin-qiyan-restaurant/dishes/dish-2.jpg'::TEXT),
                ('香菇青菜面', '清爽热汤面', 1800::BIGINT, 1600::BIGINT, 3::SMALLINT, 8::SMALLINT, 'uploads/public/merchants/ningjin-qiyan-restaurant/dishes/dish-3.jpg'::TEXT),
                ('冰糖雪梨', '清甜解腻饮品', 900::BIGINT, 800::BIGINT, 4::SMALLINT, 4::SMALLINT, 'uploads/public/merchants/ningjin-qiyan-restaurant/dishes/dish-4.jpg'::TEXT)
        ) AS dishes(name, description, price, member_price, sort_order, prepare_time, object_key)
    LOOP
        INSERT INTO media_assets (
            object_key, visibility, media_category, mime_type, file_size,
            checksum_sha256, upload_status, moderation_status, uploaded_by, source_client
        )
        VALUES (
            dish_key, 'public', 'dish', 'image/jpeg', 1024,
            md5(dish_key) || md5(dish_key), 'confirmed', 'approved', store_owner_id, 'server'
        )
        ON CONFLICT (object_key) DO UPDATE SET
            visibility = EXCLUDED.visibility,
            media_category = EXCLUDED.media_category,
            mime_type = EXCLUDED.mime_type,
            file_size = EXCLUDED.file_size,
            checksum_sha256 = EXCLUDED.checksum_sha256,
            upload_status = EXCLUDED.upload_status,
            moderation_status = EXCLUDED.moderation_status,
            uploaded_by = EXCLUDED.uploaded_by,
            source_client = EXCLUDED.source_client,
            deleted_at = NULL,
            updated_at = now()
        RETURNING id INTO store_dish_asset_id;

        SELECT d.id
        INTO store_dish_id
        FROM dishes d
        WHERE d.merchant_id = store_merchant_id
          AND d.name = dish_name
          AND d.deleted_at IS NULL
        ORDER BY d.id DESC
        LIMIT 1;

        IF store_dish_id IS NULL THEN
            INSERT INTO dishes (
                merchant_id, category_id, name, description, price, member_price,
                is_available, is_online, sort_order, created_at, updated_at,
                prepare_time, image_media_asset_id, is_packaging
            )
            VALUES (
                store_merchant_id, shared_category_id, dish_name, dish_description, dish_price, dish_member_price,
                true, true, dish_sort_order, now(), now(),
                dish_prepare_time, store_dish_asset_id, false
            );
        ELSE
            UPDATE dishes
            SET category_id = shared_category_id,
                description = dish_description,
                price = dish_price,
                member_price = dish_member_price,
                is_available = true,
                is_online = true,
                sort_order = dish_sort_order,
                prepare_time = dish_prepare_time,
                image_media_asset_id = store_dish_asset_id,
                is_packaging = false,
                updated_at = now(),
                deleted_at = NULL
            WHERE id = store_dish_id;
        END IF;
    END LOOP;
END
$$ LANGUAGE plpgsql;