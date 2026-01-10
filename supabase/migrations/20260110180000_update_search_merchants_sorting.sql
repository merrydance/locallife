-- Update search_merchants RPC with multi-dimensional sorting
CREATE OR REPLACE FUNCTION public.search_merchants(
    p_keyword TEXT DEFAULT NULL,
    p_user_lat FLOAT8 DEFAULT NULL,
    p_user_lng FLOAT8 DEFAULT NULL,
    p_page_id INTEGER DEFAULT 1,
    p_page_size INTEGER DEFAULT 10
)
RETURNS TABLE (
    id UUID,
    name TEXT,
    description TEXT,
    logo_url TEXT,
    phone TEXT,
    address TEXT,
    latitude NUMERIC,
    longitude NUMERIC,
    status TEXT,
    region_id UUID,
    is_open BOOLEAN,
    distance FLOAT8,
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
    FROM public.merchants m
    WHERE m.deleted_at IS NULL
      AND m.status = 'active'
      AND (p_keyword IS NULL OR m.name ILIKE '%' || p_keyword || '%' OR m.description ILIKE '%' || p_keyword || '%');

    RETURN QUERY
    WITH merchants_with_dist AS (
        SELECT 
            m.*,
            CASE 
                WHEN p_user_lat IS NOT NULL AND p_user_lng IS NOT NULL 
                THEN public.earth_distance(
                    public.ll_to_earth(p_user_lat, p_user_lng),
                    public.ll_to_earth(m.latitude::FLOAT8, m.longitude::FLOAT8)
                )
                ELSE 0 
            END as calculated_distance
        FROM public.merchants m
        WHERE m.deleted_at IS NULL
          AND m.status = 'active'
          AND (p_keyword IS NULL OR m.name ILIKE '%' || p_keyword || '%' OR m.description ILIKE '%' || p_keyword || '%')
    )
    SELECT 
        mwd.id,
        mwd.name,
        mwd.description,
        mwd.logo_url,
        mwd.phone,
        mwd.address,
        mwd.latitude,
        mwd.longitude,
        mwd.status,
        mwd.region_id,
        mwd.is_open,
        mwd.calculated_distance as distance,
        public.calculate_delivery_fee(
            mwd.region_id,
            mwd.id,
            mwd.calculated_distance::INTEGER,
            0
        ) as delivery_fee,
        v_total_count
    FROM merchants_with_dist mwd
    ORDER BY 
        mwd.is_open DESC,               -- 1. 营业中优先
        mwd.calculated_distance ASC,    -- 2. 距离优先
        mwd.id ASC                      -- 3. 稳定排序
    LIMIT p_page_size
    OFFSET v_offset;
END;
$$;

GRANT EXECUTE ON FUNCTION public.search_merchants TO anon, authenticated;
