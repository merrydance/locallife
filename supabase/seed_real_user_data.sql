-- Bind business data to the real user ec27a8c3-df13-4409-80e4-20a82da215ab
DO $$
DECLARE
    v_user_id UUID := 'ec27a8c3-df13-4409-80e4-20a82da215ab';
    v_region_id UUID := gen_random_uuid();
    v_merchant_id UUID := gen_random_uuid();
    v_tag_id UUID := gen_random_uuid();
    v_dish_id_1 UUID := gen_random_uuid();
    v_dish_id_2 UUID := gen_random_uuid();
BEGIN
    -- 1. Create Region (Pingxiang)
    INSERT INTO public.regions (id, code, name, level, longitude, latitude)
    VALUES (v_region_id, '130532', '平乡县', 3, 115.03008, 37.063771);

    -- 2. Create Delivery Fee Config
    INSERT INTO public.delivery_fee_configs (region_id, base_fee, base_distance, extra_fee_per_km, min_fee, is_active)
    VALUES (v_region_id, 300, 3000, 100, 300, true);

    -- 3. Create Tags
    INSERT INTO public.tags (id, name, type, status)
    VALUES (v_tag_id, '招牌', 'dish', 'active');

    -- 4. Create Merchant (Associated with real user)
    INSERT INTO public.merchants (id, owner_user_id, name, latitude, longitude, status, region_id, is_open, phone, address, description)
    VALUES (v_merchant_id, v_user_id, '快捷美食店', 37.063771, 115.03008, 'active', v_region_id, true, '13812345678', '平乡县中心路88号', '您的私人深夜食堂');

    -- 5. Create Dishes
    INSERT INTO public.dishes (id, merchant_id, name, price, is_available, is_online, prepare_time, image_url, description)
    VALUES (v_dish_id_1, v_merchant_id, '至尊红烧肉', 4800, true, true, 15, 'https://img.example.com/dish1.jpg', '精选五花肉，肥而不腻');

    INSERT INTO public.dishes (id, merchant_id, name, price, is_available, is_online, prepare_time, image_url, description)
    VALUES (v_dish_id_2, v_merchant_id, '清香排骨汤', 3200, true, true, 12, 'https://img.example.com/dish2.jpg', '文火慢炖，营养健康');

    -- 6. Tag the dishes
    INSERT INTO public.dish_tags (dish_id, tag_id) VALUES (v_dish_id_1, v_tag_id);
    INSERT INTO public.dish_tags (dish_id, tag_id) VALUES (v_dish_id_2, v_tag_id);

    RAISE NOTICE 'Seed data created successfully for user %', v_user_id;
END $$;
