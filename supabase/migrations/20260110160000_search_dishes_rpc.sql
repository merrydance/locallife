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
    ),
    ordered_dishes AS (
        SELECT * FROM dishes_with_dist
        ORDER BY 
            calculated_distance ASC,
            sort_order ASC,
            created_at DESC
        LIMIT p_page_size
        OFFSET v_offset
    )
    SELECT 
        od.id,
        od.merchant_id,
        od.name,
        od.description,
        od.image_url,
        od.price,
        od.member_price,
        od.is_available,
        od.is_online,
        od.monthly_sales,
        od.repurchase_rate,
        od.m_name,
        od.m_logo,
        od.m_is_open,
        od.calculated_distance as distance,
        (od.prepare_time * 60 + (od.calculated_distance / 3.33))::INTEGER as estimated_delivery_time,
        public.calculate_delivery_fee(od.m_region_id, od.merchant_id, od.calculated_distance::INTEGER, 0) as estimated_delivery_fee,
        v_total_count
    FROM ordered_dishes od;
END;
$$;
