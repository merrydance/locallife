-- Create search_combos RPC
CREATE OR REPLACE FUNCTION public.search_combos(
    p_keyword TEXT DEFAULT NULL,
    p_user_lat FLOAT8 DEFAULT NULL,
    p_user_lng FLOAT8 DEFAULT NULL,
    p_page_id INTEGER DEFAULT 1,
    p_page_size INTEGER DEFAULT 10
)
RETURNS TABLE (
    id UUID,
    merchant_id UUID,
    name TEXT,
    description TEXT,
    combo_price BIGINT,
    original_price BIGINT,
    image_url TEXT,
    is_online BOOLEAN,
    merchant_name TEXT,
    merchant_logo TEXT,
    merchant_is_open BOOLEAN,
    distance FLOAT8,
    estimated_delivery_fee JSONB,
    monthly_sales BIGINT,
    total_count BIGINT
) 
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
DECLARE
    v_offset INTEGER;
BEGIN
    v_offset := (p_page_id - 1) * p_page_size;

    RETURN QUERY
    WITH combos_with_dist AS (
        SELECT 
            c.*,
            m.name AS m_name,
            m.logo_url AS m_logo,
            m.is_open AS m_is_open,
            m.latitude AS m_lat,
            m.longitude AS m_lng,
            m.region_id AS m_region_id,
            CASE 
                WHEN p_user_lat IS NOT NULL AND p_user_lng IS NOT NULL 
                THEN public.earth_distance(
                    ll_to_earth(p_user_lat, p_user_lng),
                    ll_to_earth(m.latitude::FLOAT8, m.longitude::FLOAT8)
                )
                ELSE 0 
            END as calculated_distance
        FROM public.combos c
        JOIN public.merchants m ON c.merchant_id = m.id
        WHERE c.deleted_at IS NULL
          AND c.is_online = true
          AND (p_keyword IS NULL OR c.name ILIKE '%' || p_keyword || '%')
    ),
    counted_combos AS (
        SELECT COUNT(*) as total FROM combos_with_dist
    )
    SELECT 
        cwd.id,
        cwd.merchant_id,
        cwd.name,
        cwd.description,
        cwd.combo_price,
        cwd.original_price,
        cwd.image_url,
        cwd.is_online,
        cwd.m_name,
        cwd.m_logo,
        cwd.m_is_open,
        cwd.calculated_distance,
        public.calculate_delivery_fee(
            cwd.m_region_id,
            cwd.merchant_id,
            cwd.calculated_distance::NUMERIC,
            COALESCE(cwd.combo_price, 0)
        ) as delivery_fee,
        0::BIGINT as monthly_sales, -- Mocked
        cc.total
    FROM combos_with_dist cwd, counted_combos cc
    ORDER BY cwd.calculated_distance ASC, cwd.id ASC
    LIMIT p_page_size
    OFFSET v_offset;
END;
$$;

GRANT EXECUTE ON FUNCTION public.search_combos TO anon, authenticated;
