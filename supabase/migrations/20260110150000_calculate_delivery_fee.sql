CREATE OR REPLACE FUNCTION public.calculate_delivery_fee(
    p_region_id UUID,
    p_merchant_id UUID,
    p_distance INTEGER, -- in meters
    p_order_amount BIGINT -- in cents
)
RETURNS JSONB
LANGUAGE plpgsql
AS $$
DECLARE
    v_config RECORD;
    v_weather RECORD;
    v_peak_coeff NUMERIC(3,2) := 1.00;
    v_weather_coeff NUMERIC(3,2) := 1.00;
    v_base_fee BIGINT;
    v_distance_fee BIGINT := 0;
    v_value_fee BIGINT := 0;
    v_subtotal BIGINT;
    v_promo_discount BIGINT := 0;
    v_final_fee BIGINT;
    v_delivery_suspended BOOLEAN := FALSE;
    v_suspend_reason TEXT := '';
    v_now TIMESTAMP WITH TIME ZONE := NOW();
    v_current_day SMALLINT := EXTRACT(DOW FROM NOW()); -- 0=Sunday
BEGIN
    -- 1. Get Base Configuration
    SELECT * INTO v_config 
    FROM public.delivery_fee_configs 
    WHERE region_id = p_region_id AND is_active = TRUE;

    IF NOT FOUND THEN
        -- Default Fallback
        v_base_fee := 500;
        v_subtotal := 500;
    ELSE
        v_base_fee := v_config.base_fee;

        -- 2. Calculate Distance Fee
        IF p_distance > v_config.base_distance THEN
            v_distance_fee := ((p_distance - v_config.base_distance)::NUMERIC / 1000.0 * v_config.extra_fee_per_km)::BIGINT;
        END IF;

        -- 3. Calculate Value Fee
        v_value_fee := (p_order_amount::NUMERIC * v_config.value_ratio)::BIGINT;

        -- 4. Get Weather Coefficient (Latest within 30 mins)
        SELECT * INTO v_weather
        FROM public.weather_coefficients
        WHERE region_id = p_region_id
          AND recorded_at > (v_now - INTERVAL '30 minutes')
        ORDER BY recorded_at DESC
        LIMIT 1;

        IF FOUND THEN
            v_weather_coeff := v_weather.final_coefficient;
            IF v_weather.delivery_suspended THEN
                v_delivery_suspended := TRUE;
                v_suspend_reason := 'extreme weather warning';
            END IF;
        END IF;

        -- 5. Get Peak Hour Coefficient
        SELECT MAX(coefficient) INTO v_peak_coeff
        FROM public.peak_hour_configs
        WHERE region_id = p_region_id
          AND is_active = TRUE
          AND v_current_day = ANY(days_of_week)
          AND (
            (start_time <= end_time AND v_now::TIME >= start_time AND v_now::TIME < end_time) OR
            (start_time > end_time AND (v_now::TIME >= start_time OR v_now::TIME < end_time))
          );

        IF v_peak_coeff IS NULL THEN
            v_peak_coeff := 1.00;
        END IF;

        -- 6. Apply Coefficients
        v_subtotal := ((v_base_fee + v_distance_fee + v_value_fee)::NUMERIC * v_weather_coeff * v_peak_coeff)::BIGINT;

        -- 7. Apply Min/Max Constraints
        IF v_config.max_fee IS NOT NULL AND v_subtotal > v_config.max_fee THEN
            v_subtotal := v_config.max_fee;
        END IF;
        IF v_subtotal < v_config.min_fee THEN
            v_subtotal := v_config.min_fee;
        END IF;
    END IF;

    -- 8. Merchant Promotions
    SELECT MAX(discount_amount) INTO v_promo_discount
    FROM public.merchant_delivery_promotions
    WHERE merchant_id = p_merchant_id
      AND is_active = TRUE
      AND p_order_amount >= min_order_amount
      AND v_now >= valid_from
      AND v_now <= valid_until;

    IF v_promo_discount IS NULL THEN
        v_promo_discount := 0;
    END IF;

    -- 9. Final Calculation
    v_final_fee := v_subtotal - v_promo_discount;
    IF v_final_fee < 0 THEN
        v_final_fee := 0;
    END IF;

    RETURN jsonb_build_object(
        'base_fee', v_base_fee,
        'distance_fee', v_distance_fee,
        'value_fee', v_value_fee,
        'weather_coefficient', v_weather_coeff,
        'peak_hour_coefficient', v_peak_coeff,
        'subtotal_fee', v_subtotal,
        'promotion_discount', v_promo_discount,
        'final_fee', v_final_fee,
        'delivery_suspended', v_delivery_suspended,
        'suspend_reason', CASE WHEN v_suspend_reason = '' THEN NULL ELSE v_suspend_reason END
    );
END;
$$;
