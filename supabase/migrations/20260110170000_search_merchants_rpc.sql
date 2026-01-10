CREATE OR REPLACE FUNCTION public.search_merchants(
    p_keyword TEXT DEFAULT NULL,
    p_user_lat NUMERIC DEFAULT NULL,
    p_user_lng NUMERIC DEFAULT NULL,
    p_page_id INTEGER DEFAULT 1,
    p_page_size INTEGER DEFAULT 10
)
RETURNS TABLE (
    id UUID,
    name TEXT,
    description TEXT,
    address TEXT,
    phone TEXT,
    logo_url TEXT,
    latitude NUMERIC,
    longitude NUMERIC,
    status TEXT,
    region_id UUID,
    is_open BOOLEAN,
    distance FLOAT,
    estimated_delivery_fee JSONB,
    total_count BIGINT
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_offset INTEGER := (p_page_id - 1) * p_page_size;
    v_total_count BIGINT;
BEGIN
    SELECT COUNT(*) INTO v_total_count
    FROM public.merchants m
    WHERE (p_keyword IS NULL OR m.name ILIKE '%' || p_keyword || '%' OR m.description ILIKE '%' || p_keyword || '%')
      AND m.status = 'active'
      AND m.deleted_at IS NULL;

    RETURN QUERY
    WITH merchants_with_dist AS (
        SELECT 
            m.*,
            CASE 
                WHEN p_user_lat IS NOT NULL AND p_user_lng IS NOT NULL 
                THEN public.earth_distance(public.ll_to_earth(p_user_lat::FLOAT8, p_user_lng::FLOAT8), public.ll_to_earth(m.latitude::FLOAT8, m.longitude::FLOAT8))
                ELSE 0.0
            END as calculated_distance
        FROM public.merchants m
        WHERE (p_keyword IS NULL OR m.name ILIKE '%' || p_keyword || '%' OR m.description ILIKE '%' || p_keyword || '%')
          AND m.status = 'active'
          AND m.deleted_at IS NULL
    ),
    ordered_merchants AS (
        SELECT * FROM merchants_with_dist
        ORDER BY 
            calculated_distance ASC,
            created_at DESC
        LIMIT p_page_size
        OFFSET v_offset
    )
    SELECT 
        om.id,
        om.name,
        om.description,
        om.address,
        om.phone,
        om.logo_url,
        om.latitude,
        om.longitude,
        om.status,
        om.region_id,
        om.is_open,
        om.calculated_distance as distance,
        public.calculate_delivery_fee(om.region_id, om.id, om.calculated_distance::INTEGER, 0) as estimated_delivery_fee,
        v_total_count
    FROM ordered_merchants om;
END;
$$;
