-- Update search_dishes RPC with multi-dimensional sorting
CREATE OR REPLACE FUNCTION public.search_dishes(
    p_keyword TEXT DEFAULT NULL,
    p_tag_id UUID DEFAULT NULL,
    p_user_lat NUMERIC DEFAULT NULL,
    p_user_lng NUMERIC DEFAULT NULL,
    p_page_id INTEGER DEFAULT 1,
    p_page_size INTEGER DEFAULT 10
)
RETURNS TABLE (
    id UUID,
    merchant_id UUID,
    name TEXT,
    description TEXT,
    image_url TEXT,
    price BIGINT,
    member_price BIGINT,
    is_available BOOLEAN,
    is_online BOOLEAN,
    monthly_sales INTEGER,
    repurchase_rate NUMERIC,
    merchant_name TEXT,
    merchant_logo TEXT,
    merchant_is_open BOOLEAN,
    distance FLOAT,
    estimated_delivery_time INTEGER,
    estimated_delivery_fee JSONB,
    total_count BIGINT
) 
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
DECLARE
    v_offset INTEGER := (p_page_id - 1) * p_page_size;
    v_total_count BIGINT;
BEGIN
    -- 1. Get total count
    SELECT COUNT(*) INTO v_total_count
    FROM public.dishes d
    JOIN public.merchants m ON d.merchant_id = m.id
    WHERE (p_keyword IS NULL OR d.name ILIKE '%' || p_keyword || '%' OR m.name ILIKE '%' || p_keyword || '%')
      AND (p_tag_id IS NULL OR EXISTS (SELECT 1 FROM public.dish_tags dt WHERE dt.dish_id = d.id AND dt.tag_id = p_tag_id))
      AND d.is_online = TRUE 
      AND d.deleted_at IS NULL
      AND m.status = 'active'
      AND m.deleted_at IS NULL;

    RETURN QUERY
    WITH dishes_with_dist AS (
        SELECT 
            d.*,
            m.name as m_name,
            m.logo_url as m_logo,
            m.is_open as m_is_open,
            m.latitude as m_lat,
            m.longitude as m_lng,
            m.region_id as m_region_id,
            CASE 
                WHEN p_user_lat IS NOT NULL AND p_user_lng IS NOT NULL 
                THEN public.earth_distance(public.ll_to_earth(p_user_lat::FLOAT8, p_user_lng::FLOAT8), public.ll_to_earth(m.latitude::FLOAT8, m.longitude::FLOAT8))
                ELSE 0.0
            END as calculated_distance
        FROM public.dishes d
        JOIN public.merchants m ON d.merchant_id = m.id
        WHERE (p_keyword IS NULL OR d.name ILIKE '%' || p_keyword || '%' OR m.name ILIKE '%' || p_keyword || '%')
          AND (p_tag_id IS NULL OR EXISTS (SELECT 1 FROM public.dish_tags dt WHERE dt.dish_id = d.id AND dt.tag_id = p_tag_id))
          AND d.is_online = TRUE 
          AND d.deleted_at IS NULL
          AND m.status = 'active'
          AND m.deleted_at IS NULL
    )
    SELECT 
        d.id,
        d.merchant_id,
        d.name,
        d.description,
        d.image_url,
        d.price,
        d.member_price,
        d.is_available,
        d.is_online,
        d.monthly_sales,
        d.repurchase_rate,
        d.m_name,
        d.m_logo,
        d.m_is_open,
        d.calculated_distance as distance,
        (d.prepare_time * 60 + (d.calculated_distance / 3.33))::INTEGER as estimated_delivery_time,
        public.calculate_delivery_fee(d.m_region_id, d.merchant_id, d.calculated_distance::INTEGER, 0) as estimated_delivery_fee,
        v_total_count
    FROM dishes_with_dist d
    ORDER BY 
        d.m_is_open DESC,               -- 1. 营业中优先
        d.calculated_distance ASC,      -- 2. 距离优先
        d.monthly_sales DESC,           -- 3. 销量优先
        d.price ASC                     -- 4. 价格优先
    LIMIT p_page_size
    OFFSET v_offset;
END;
$$;

GRANT EXECUTE ON FUNCTION public.search_dishes TO anon, authenticated;
