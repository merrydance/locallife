--
-- PostgreSQL database dump
--

CREATE EXTENSION IF NOT EXISTS "pgcrypto";


\restrict 6nWZASry3GkjxglF8eh70qww1Fj4X0V0Od1kp2Ngj5fEFyO6iSB6EnDvWdlDlfN

-- Dumped from database version 17.6 (Debian 17.6-0+deb13u1)
-- Dumped by pg_dump version 17.6 (Debian 17.6-0+deb13u1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: cube; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS cube WITH SCHEMA public;


--
-- Name: EXTENSION cube; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION cube IS 'data type for multidimensional cubes';


--
-- Name: earthdistance; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS earthdistance WITH SCHEMA public;


--
-- Name: EXTENSION earthdistance; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION earthdistance IS 'calculate great-circle distances on the surface of the Earth';


--
-- Name: update_updated_at_column(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.update_updated_at_column() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: appeals; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.appeals (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    claim_id uuid NOT NULL,
    appellant_type text NOT NULL,
    appellant_id uuid NOT NULL,
    reason text NOT NULL,
    evidence_urls text[],
    status text DEFAULT 'pending'::text NOT NULL,
    reviewer_id uuid,
    review_notes text,
    reviewed_at timestamp with time zone,
    compensation_amount bigint,
    compensated_at timestamp with time zone,
    region_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT appeals_appellant_type_check CHECK ((appellant_type = ANY (ARRAY['merchant'::text, 'rider'::text]))),
    CONSTRAINT appeals_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'approved'::text, 'rejected'::text])))
);


--
-- Name: TABLE appeals; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.appeals IS '申诉表 - 商户/骑手对索赔的申诉';


--
-- Name: COLUMN appeals.appellant_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.appeals.appellant_type IS '申诉人类型：merchant=商户, rider=骑手';


--
-- Name: COLUMN appeals.appellant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.appeals.appellant_id IS '申诉人ID（商户ID或骑手ID）';


--
-- Name: COLUMN appeals.compensation_amount; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.appeals.compensation_amount IS '补偿金额（申诉成功时平台垫付给申诉人）';


--
-- Name: COLUMN appeals.region_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.appeals.region_id IS '关联区域（用于运营商按区域过滤）';


--
-- Name: appeals_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.appeals_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: browse_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.browse_history (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    target_type text NOT NULL,
    target_id uuid NOT NULL,
    last_viewed_at timestamp with time zone DEFAULT now() NOT NULL,
    view_count integer DEFAULT 1 NOT NULL,
    CONSTRAINT browse_history_target_type_check CHECK ((target_type = ANY (ARRAY['merchant'::text, 'dish'::text])))
);


--
-- Name: TABLE browse_history; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.browse_history IS '用户浏览历史';


--
-- Name: COLUMN browse_history.target_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.browse_history.target_type IS '浏览目标类型: merchant(商户), dish(菜品)';


--
-- Name: COLUMN browse_history.view_count; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.browse_history.view_count IS '浏览次数';


--
-- Name: browse_history_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.browse_history_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: cart_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cart_items (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    cart_id uuid NOT NULL,
    dish_id uuid,
    combo_id uuid,
    quantity smallint DEFAULT 1 NOT NULL,
    customizations jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT cart_items_dish_or_combo_check CHECK ((((dish_id IS NOT NULL) AND (combo_id IS NULL)) OR ((dish_id IS NULL) AND (combo_id IS NOT NULL))))
);


--
-- Name: TABLE cart_items; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.cart_items IS '购物车商品表';


--
-- Name: COLUMN cart_items.cart_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.cart_items.cart_id IS '购物车ID';


--
-- Name: COLUMN cart_items.dish_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.cart_items.dish_id IS '菜品ID，与combo_id二选一';


--
-- Name: COLUMN cart_items.combo_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.cart_items.combo_id IS '套餐ID，与dish_id二选一';


--
-- Name: COLUMN cart_items.quantity; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.cart_items.quantity IS '数量';


--
-- Name: COLUMN cart_items.customizations; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.cart_items.customizations IS '定制选项，如 [{"name":"辣度","value":"微辣","extra_price":0}]';


--
-- Name: cart_items_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.cart_items_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: carts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.carts (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    merchant_id uuid NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    order_type text DEFAULT 'takeout'::text NOT NULL,
    table_id uuid,
    reservation_id uuid
);


--
-- Name: TABLE carts; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.carts IS '购物车主表，每个用户每个商户一个购物车';


--
-- Name: COLUMN carts.user_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.carts.user_id IS '用户ID';


--
-- Name: COLUMN carts.merchant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.carts.merchant_id IS '商户ID';


--
-- Name: COLUMN carts.order_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.carts.order_type IS '订单类型：takeout=外卖, dine_in=堂食, reservation=预订';


--
-- Name: COLUMN carts.table_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.carts.table_id IS '桌台ID（仅堂食有效）';


--
-- Name: COLUMN carts.reservation_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.carts.reservation_id IS '预订ID（仅预订有效）';


--
-- Name: carts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.carts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: claims; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.claims (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    order_id uuid NOT NULL,
    user_id uuid NOT NULL,
    claim_type text NOT NULL,
    description text NOT NULL,
    evidence_urls text[],
    claim_amount bigint NOT NULL,
    approved_amount bigint,
    status text DEFAULT 'pending'::text NOT NULL,
    approval_type text,
    trust_score_snapshot smallint,
    is_malicious boolean DEFAULT false NOT NULL,
    lookback_result jsonb,
    auto_approval_reason text,
    rejection_reason text,
    reviewer_id uuid,
    review_notes text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    reviewed_at timestamp with time zone,
    paid_at timestamp with time zone,
    CONSTRAINT claims_approval_type_check CHECK ((approval_type = ANY (ARRAY['instant'::text, 'auto'::text, 'manual'::text]))),
    CONSTRAINT claims_claim_type_check CHECK ((claim_type = ANY (ARRAY['foreign-object'::text, 'damage'::text, 'delay'::text, 'quality'::text, 'missing-item'::text, 'other'::text]))),
    CONSTRAINT claims_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'auto-approved'::text, 'manual-review'::text, 'approved'::text, 'rejected'::text])))
);


--
-- Name: TABLE claims; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.claims IS '索赔记录表 - 信用驱动免证索赔';


--
-- Name: COLUMN claims.approval_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.claims.approval_type IS 'instant=秒赔(>=750分+<=50元), auto=回溯通过, manual=人工审核';


--
-- Name: COLUMN claims.trust_score_snapshot; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.claims.trust_score_snapshot IS '用户提交索赔时的信用分快照（决策依据）';


--
-- Name: COLUMN claims.lookback_result; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.claims.lookback_result IS '回溯检查：最近5笔订单（30天→90天→1年）的索赔历史';


--
-- Name: claims_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.claims_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: cloud_printers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cloud_printers (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    merchant_id uuid NOT NULL,
    printer_name text NOT NULL,
    printer_sn text NOT NULL,
    printer_key text NOT NULL,
    printer_type text NOT NULL,
    print_takeout boolean DEFAULT true NOT NULL,
    print_dine_in boolean DEFAULT true NOT NULL,
    print_reservation boolean DEFAULT true NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone,
    CONSTRAINT cloud_printers_printer_type_check CHECK ((printer_type = ANY (ARRAY['feieyun'::text, 'yilianyun'::text, 'other'::text])))
);


--
-- Name: cloud_printers_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.cloud_printers_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: combined_payment_orders; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.combined_payment_orders (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    combine_out_trade_no text NOT NULL,
    total_amount bigint NOT NULL,
    prepay_id text,
    transaction_id text,
    status text DEFAULT 'pending'::text NOT NULL,
    paid_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone,
    CONSTRAINT combined_payment_orders_amount_check CHECK ((total_amount > 0)),
    CONSTRAINT combined_payment_orders_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'paid'::text, 'failed'::text, 'closed'::text])))
);


--
-- Name: TABLE combined_payment_orders; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.combined_payment_orders IS '合单支付主表，支持多商户一次支付';


--
-- Name: COLUMN combined_payment_orders.combine_out_trade_no; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.combined_payment_orders.combine_out_trade_no IS '合单商户订单号，微信合单支付接口使用';


--
-- Name: COLUMN combined_payment_orders.total_amount; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.combined_payment_orders.total_amount IS '合计支付金额（分），所有子订单之和';


--
-- Name: combined_payment_orders_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.combined_payment_orders_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: combined_payment_sub_orders; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.combined_payment_sub_orders (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    combined_payment_id uuid NOT NULL,
    order_id uuid NOT NULL,
    merchant_id uuid NOT NULL,
    sub_mchid text NOT NULL,
    amount bigint NOT NULL,
    out_trade_no text NOT NULL,
    description text NOT NULL,
    profit_sharing_status text DEFAULT 'pending'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT combined_payment_sub_orders_amount_check CHECK ((amount > 0)),
    CONSTRAINT combined_payment_sub_orders_profit_sharing_status_check CHECK ((profit_sharing_status = ANY (ARRAY['pending'::text, 'processing'::text, 'finished'::text, 'failed'::text])))
);


--
-- Name: TABLE combined_payment_sub_orders; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.combined_payment_sub_orders IS '合单支付子订单表，每个商户一个子单';


--
-- Name: COLUMN combined_payment_sub_orders.sub_mchid; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.combined_payment_sub_orders.sub_mchid IS '微信子商户号';


--
-- Name: COLUMN combined_payment_sub_orders.out_trade_no; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.combined_payment_sub_orders.out_trade_no IS '子单商户订单号，对应微信的sub_out_trade_no';


--
-- Name: combined_payment_sub_orders_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.combined_payment_sub_orders_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: combo_dishes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.combo_dishes (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    combo_id uuid NOT NULL,
    dish_id uuid NOT NULL,
    quantity smallint DEFAULT 1 NOT NULL
);


--
-- Name: combo_dishes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.combo_dishes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: combo_sets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.combo_sets (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    merchant_id uuid NOT NULL,
    name text NOT NULL,
    description text,
    image_url text,
    original_price bigint NOT NULL,
    combo_price bigint NOT NULL,
    is_online boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone
);


--
-- Name: combo_sets_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.combo_sets_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: combo_tags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.combo_tags (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    combo_id uuid NOT NULL,
    tag_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: combo_tags_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.combo_tags_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: daily_inventory; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.daily_inventory (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    merchant_id uuid NOT NULL,
    dish_id uuid NOT NULL,
    date date NOT NULL,
    total_quantity integer DEFAULT '-1'::integer NOT NULL,
    sold_quantity integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone
);


--
-- Name: daily_inventory_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.daily_inventory_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: deliveries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.deliveries (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    order_id uuid NOT NULL,
    rider_id uuid,
    pickup_address text NOT NULL,
    pickup_longitude numeric(10,7) NOT NULL,
    pickup_latitude numeric(10,7) NOT NULL,
    pickup_contact character varying(50),
    pickup_phone character varying(20),
    picked_at timestamp with time zone,
    delivery_address text NOT NULL,
    delivery_longitude numeric(10,7) NOT NULL,
    delivery_latitude numeric(10,7) NOT NULL,
    delivery_contact character varying(50),
    delivery_phone character varying(20),
    delivered_at timestamp with time zone,
    distance integer NOT NULL,
    delivery_fee bigint NOT NULL,
    rider_earnings bigint DEFAULT 0 NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    estimated_pickup_at timestamp with time zone,
    estimated_delivery_at timestamp with time zone,
    is_damaged boolean DEFAULT false NOT NULL,
    is_delayed boolean DEFAULT false NOT NULL,
    damage_amount bigint DEFAULT 0 NOT NULL,
    damage_reason text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    assigned_at timestamp with time zone,
    completed_at timestamp with time zone,
    CONSTRAINT deliveries_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'assigned'::character varying, 'picking'::character varying, 'picked'::character varying, 'delivering'::character varying, 'delivered'::character varying, 'completed'::character varying, 'cancelled'::character varying])::text[])))
);


--
-- Name: TABLE deliveries; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.deliveries IS '配送单表';


--
-- Name: COLUMN deliveries.distance; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.deliveries.distance IS '配送距离（米）';


--
-- Name: COLUMN deliveries.rider_earnings; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.deliveries.rider_earnings IS '骑手配送收益（分）';


--
-- Name: COLUMN deliveries.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.deliveries.status IS '状态：pending待分配/assigned已分配/picking取餐中/picked已取餐/delivering配送中/delivered已送达/completed已完成/cancelled已取消';


--
-- Name: deliveries_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.deliveries_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: delivery_fee_configs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.delivery_fee_configs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    region_id uuid NOT NULL,
    base_fee bigint NOT NULL,
    base_distance integer NOT NULL,
    extra_fee_per_km bigint NOT NULL,
    value_ratio numeric(5,4) DEFAULT 0.0100 NOT NULL,
    max_fee bigint,
    min_fee bigint DEFAULT 0 NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone
);


--
-- Name: TABLE delivery_fee_configs; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.delivery_fee_configs IS '运费配置表，按区县配置基础运费规则';


--
-- Name: COLUMN delivery_fee_configs.region_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.delivery_fee_configs.region_id IS '区县级别的region_id';


--
-- Name: COLUMN delivery_fee_configs.base_fee; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.delivery_fee_configs.base_fee IS '基础运费（分）';


--
-- Name: COLUMN delivery_fee_configs.base_distance; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.delivery_fee_configs.base_distance IS '基础距离（米），在此范围内收取基础运费';


--
-- Name: COLUMN delivery_fee_configs.extra_fee_per_km; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.delivery_fee_configs.extra_fee_per_km IS '超出基础距离后每公里加价（分）';


--
-- Name: COLUMN delivery_fee_configs.value_ratio; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.delivery_fee_configs.value_ratio IS '货值系数，如0.01表示1%';


--
-- Name: COLUMN delivery_fee_configs.max_fee; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.delivery_fee_configs.max_fee IS '最高运费上限（分），NULL表示不限';


--
-- Name: COLUMN delivery_fee_configs.min_fee; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.delivery_fee_configs.min_fee IS '最低运费（分）';


--
-- Name: delivery_fee_configs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.delivery_fee_configs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: delivery_pool; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.delivery_pool (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    order_id uuid NOT NULL,
    merchant_id uuid NOT NULL,
    pickup_longitude numeric(10,7) NOT NULL,
    pickup_latitude numeric(10,7) NOT NULL,
    delivery_longitude numeric(10,7) NOT NULL,
    delivery_latitude numeric(10,7) NOT NULL,
    distance integer NOT NULL,
    delivery_fee bigint NOT NULL,
    expected_pickup_at timestamp with time zone NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    priority integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE delivery_pool; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.delivery_pool IS '可接单订单池';


--
-- Name: COLUMN delivery_pool.distance; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.delivery_pool.distance IS '配送距离（米）';


--
-- Name: delivery_pool_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.delivery_pool_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: discount_rules; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discount_rules (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    merchant_id uuid NOT NULL,
    name text NOT NULL,
    description text,
    min_order_amount bigint NOT NULL,
    discount_amount bigint NOT NULL,
    can_stack_with_voucher boolean DEFAULT false NOT NULL,
    can_stack_with_membership boolean DEFAULT true NOT NULL,
    valid_from timestamp with time zone NOT NULL,
    valid_until timestamp with time zone NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    CONSTRAINT check_discount_amount_valid CHECK ((discount_amount < min_order_amount)),
    CONSTRAINT check_discount_valid_period CHECK ((valid_until > valid_from)),
    CONSTRAINT discount_rules_discount_amount_check CHECK ((discount_amount > 0)),
    CONSTRAINT discount_rules_min_order_amount_check CHECK ((min_order_amount > 0))
);


--
-- Name: TABLE discount_rules; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.discount_rules IS 'M10: 满减规则表';


--
-- Name: discount_rules_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discount_rules_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: dish_categories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dish_categories (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone
);


--
-- Name: dish_categories_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.dish_categories_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: dish_customization_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dish_customization_groups (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    dish_id uuid NOT NULL,
    name text NOT NULL,
    is_required boolean DEFAULT false NOT NULL,
    sort_order smallint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: dish_customization_groups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.dish_customization_groups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: dish_customization_options; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dish_customization_options (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    group_id uuid NOT NULL,
    tag_id uuid NOT NULL,
    extra_price bigint DEFAULT 0 NOT NULL,
    sort_order smallint DEFAULT 0 NOT NULL
);


--
-- Name: dish_customization_options_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.dish_customization_options_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: dish_ingredients; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dish_ingredients (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    dish_id uuid NOT NULL,
    ingredient_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: dish_ingredients_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.dish_ingredients_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: dish_tags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dish_tags (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    dish_id uuid NOT NULL,
    tag_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: dish_tags_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.dish_tags_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: dishes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dishes (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    merchant_id uuid NOT NULL,
    category_id uuid,
    name text NOT NULL,
    description text,
    image_url text,
    price bigint NOT NULL,
    member_price bigint,
    is_available boolean DEFAULT true NOT NULL,
    is_online boolean DEFAULT true NOT NULL,
    sort_order smallint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone,
    prepare_time smallint DEFAULT 10 NOT NULL,
    deleted_at timestamp with time zone,
    monthly_sales integer DEFAULT 0 NOT NULL,
    repurchase_rate numeric(5,4) DEFAULT 0 NOT NULL
);


--
-- Name: COLUMN dishes.prepare_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dishes.prepare_time IS '预估制作时间（分钟），默认10分钟';


--
-- Name: dishes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.dishes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: ecommerce_applyments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ecommerce_applyments (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    subject_type text NOT NULL,
    subject_id uuid NOT NULL,
    out_request_no text NOT NULL,
    applyment_id uuid,
    organization_type text NOT NULL,
    business_license_number text,
    business_license_copy text,
    merchant_name text NOT NULL,
    legal_person text NOT NULL,
    id_card_number text NOT NULL,
    id_card_name text NOT NULL,
    id_card_valid_time text NOT NULL,
    id_card_front_copy text NOT NULL,
    id_card_back_copy text NOT NULL,
    account_type text NOT NULL,
    account_bank text NOT NULL,
    bank_address_code text NOT NULL,
    bank_name text,
    account_number text NOT NULL,
    account_name text NOT NULL,
    contact_name text NOT NULL,
    contact_id_card_number text,
    mobile_phone text NOT NULL,
    contact_email text,
    merchant_shortname text NOT NULL,
    qualifications jsonb,
    business_addition_pics text[],
    business_addition_desc text,
    status text DEFAULT 'pending'::text NOT NULL,
    sign_url text,
    sign_state text,
    reject_reason text,
    sub_mch_id text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    submitted_at timestamp with time zone,
    audited_at timestamp with time zone,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT ecommerce_applyments_account_type_check CHECK ((account_type = ANY (ARRAY['ACCOUNT_TYPE_BUSINESS'::text, 'ACCOUNT_TYPE_PRIVATE'::text]))),
    CONSTRAINT ecommerce_applyments_org_type_check CHECK ((organization_type = ANY (ARRAY['2401'::text, '2500'::text, '2600'::text]))),
    CONSTRAINT ecommerce_applyments_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'submitted'::text, 'auditing'::text, 'rejected'::text, 'frozen'::text, 'to_be_signed'::text, 'signing'::text, 'rejected_sign'::text, 'finish'::text]))),
    CONSTRAINT ecommerce_applyments_subject_type_check CHECK ((subject_type = ANY (ARRAY['merchant'::text, 'rider'::text, 'operator'::text])))
);


--
-- Name: TABLE ecommerce_applyments; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.ecommerce_applyments IS '微信平台收付通二级商户进件申请';


--
-- Name: COLUMN ecommerce_applyments.subject_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ecommerce_applyments.subject_type IS '主体类型: merchant-商户, rider-骑手, operator-运营商';


--
-- Name: COLUMN ecommerce_applyments.organization_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ecommerce_applyments.organization_type IS '微信主体类型: 2401-小微商户(个人), 2500-个体工商户, 2600-企业';


--
-- Name: COLUMN ecommerce_applyments.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.ecommerce_applyments.status IS '进件状态: pending-待提交, submitted-已提交, auditing-审核中, rejected-已驳回, frozen-冻结, to_be_signed-待签约, signing-签约中, rejected_sign-签约失败, finish-完成';


--
-- Name: ecommerce_applyments_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ecommerce_applyments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: favorites; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.favorites (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    favorite_type text NOT NULL,
    merchant_id uuid,
    dish_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT favorites_target_check CHECK ((((favorite_type = 'merchant'::text) AND (merchant_id IS NOT NULL) AND (dish_id IS NULL)) OR ((favorite_type = 'dish'::text) AND (dish_id IS NOT NULL) AND (merchant_id IS NULL)))),
    CONSTRAINT favorites_type_check CHECK ((favorite_type = ANY (ARRAY['merchant'::text, 'dish'::text])))
);


--
-- Name: TABLE favorites; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.favorites IS '用户收藏表';


--
-- Name: COLUMN favorites.user_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.favorites.user_id IS '用户ID';


--
-- Name: COLUMN favorites.favorite_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.favorites.favorite_type IS '收藏类型：merchant=商户, dish=菜品';


--
-- Name: COLUMN favorites.merchant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.favorites.merchant_id IS '收藏的商户ID';


--
-- Name: COLUMN favorites.dish_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.favorites.dish_id IS '收藏的菜品ID';


--
-- Name: favorites_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.favorites_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: food_safety_incidents; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.food_safety_incidents (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    order_id uuid NOT NULL,
    merchant_id uuid NOT NULL,
    user_id uuid NOT NULL,
    incident_type text NOT NULL,
    description text NOT NULL,
    evidence_urls text[] NOT NULL,
    order_snapshot jsonb NOT NULL,
    merchant_snapshot jsonb NOT NULL,
    rider_snapshot jsonb,
    status text DEFAULT 'reported'::text NOT NULL,
    investigation_report text,
    resolution text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    resolved_at timestamp with time zone,
    CONSTRAINT food_safety_incidents_status_check CHECK ((status = ANY (ARRAY['reported'::text, 'investigating'::text, 'merchant-suspended'::text, 'resolved'::text])))
);


--
-- Name: TABLE food_safety_incidents; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.food_safety_incidents IS '食品安全事件表 - 唯一需要证据的场景';


--
-- Name: COLUMN food_safety_incidents.evidence_urls; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.food_safety_incidents.evidence_urls IS '食安必须有证据（照片、描述）';


--
-- Name: COLUMN food_safety_incidents.order_snapshot; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.food_safety_incidents.order_snapshot IS '订单完整快照（所有字段）用于事故溯源';


--
-- Name: COLUMN food_safety_incidents.merchant_snapshot; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.food_safety_incidents.merchant_snapshot IS '商户快照（菜单、当班员工）';


--
-- Name: COLUMN food_safety_incidents.rider_snapshot; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.food_safety_incidents.rider_snapshot IS '骑手快照（配送路线、时间）';


--
-- Name: COLUMN food_safety_incidents.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.food_safety_incidents.status IS 'merchant-suspended: 商户已熔断，需整改和人工审核';


--
-- Name: food_safety_incidents_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.food_safety_incidents_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: fraud_patterns; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.fraud_patterns (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    pattern_type text NOT NULL,
    related_user_ids bigint[] NOT NULL,
    related_order_ids bigint[],
    related_claim_ids bigint[],
    device_fingerprints text[],
    address_ids bigint[],
    ip_addresses text[],
    pattern_description text,
    match_count smallint DEFAULT 1 NOT NULL,
    is_confirmed boolean DEFAULT false NOT NULL,
    reviewer_id uuid,
    review_notes text,
    action_taken text,
    detected_at timestamp with time zone DEFAULT now() NOT NULL,
    reviewed_at timestamp with time zone,
    confirmed_at timestamp with time zone,
    CONSTRAINT fraud_patterns_pattern_type_check CHECK ((pattern_type = ANY (ARRAY['device-reuse'::text, 'address-cluster'::text, 'coordinated-claims'::text, 'payment-link'::text, 'time-anomaly'::text])))
);


--
-- Name: TABLE fraud_patterns; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.fraud_patterns IS '欺诈模式检测表 - 纯规则引擎（设备指纹+地址聚类+协同索赔）';


--
-- Name: COLUMN fraud_patterns.pattern_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.fraud_patterns.pattern_type IS 'device-reuse: 同设备多账号, address-cluster: 同地址多账号, coordinated-claims: 协同索赔';


--
-- Name: COLUMN fraud_patterns.match_count; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.fraud_patterns.match_count IS '匹配规则数量（同设备+同地址=2），>=2确认为欺诈';


--
-- Name: COLUMN fraud_patterns.is_confirmed; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.fraud_patterns.is_confirmed IS '确认后拉黑用户，平台返还商户/骑手损失';


--
-- Name: fraud_patterns_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.fraud_patterns_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: ingredients; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ingredients (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    is_system boolean DEFAULT false NOT NULL,
    category text,
    is_allergen boolean DEFAULT false NOT NULL,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: ingredients_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ingredients_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: membership_transactions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.membership_transactions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    membership_id uuid NOT NULL,
    type text NOT NULL,
    amount bigint NOT NULL,
    balance_after bigint NOT NULL,
    related_order_id uuid,
    recharge_rule_id uuid,
    notes text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT membership_transactions_balance_after_check CHECK ((balance_after >= 0)),
    CONSTRAINT membership_transactions_type_check CHECK ((type = ANY (ARRAY['recharge'::text, 'consume'::text, 'refund'::text, 'bonus'::text])))
);


--
-- Name: TABLE membership_transactions; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.membership_transactions IS 'M10: 会员交易流水表';


--
-- Name: membership_transactions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.membership_transactions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: merchant_applications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.merchant_applications (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    merchant_name text NOT NULL,
    business_license_number text NOT NULL,
    business_license_image_url text NOT NULL,
    legal_person_name text NOT NULL,
    legal_person_id_number text NOT NULL,
    legal_person_id_front_url text NOT NULL,
    legal_person_id_back_url text NOT NULL,
    contact_phone text NOT NULL,
    business_address text NOT NULL,
    business_scope text,
    status text DEFAULT 'draft'::text NOT NULL,
    reject_reason text,
    reviewed_by uuid,
    reviewed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    longitude numeric(10,7),
    latitude numeric(10,7),
    region_id uuid,
    food_permit_url text,
    food_permit_ocr jsonb,
    business_license_ocr jsonb,
    id_card_front_ocr jsonb,
    id_card_back_ocr jsonb,
    storefront_images jsonb,
    environment_images jsonb,
    CONSTRAINT merchant_applications_status_check CHECK ((status = ANY (ARRAY['draft'::text, 'submitted'::text, 'approved'::text, 'rejected'::text])))
);


--
-- Name: COLUMN merchant_applications.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_applications.status IS 'draft=草稿, submitted=已提交(待审核), approved=已通过, rejected=已拒绝';


--
-- Name: COLUMN merchant_applications.longitude; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_applications.longitude IS '商户位置经度，由前端地图选点提供';


--
-- Name: COLUMN merchant_applications.latitude; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_applications.latitude IS '商户位置纬度，由前端地图选点提供';


--
-- Name: COLUMN merchant_applications.region_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_applications.region_id IS '区域ID，根据商户定位自动确定';


--
-- Name: COLUMN merchant_applications.food_permit_url; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_applications.food_permit_url IS '食品经营许可证图片URL';


--
-- Name: COLUMN merchant_applications.food_permit_ocr; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_applications.food_permit_ocr IS '食品经营许可证OCR识别结果JSON';


--
-- Name: COLUMN merchant_applications.business_license_ocr; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_applications.business_license_ocr IS '营业执照OCR识别结果JSON';


--
-- Name: COLUMN merchant_applications.id_card_front_ocr; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_applications.id_card_front_ocr IS '身份证正面OCR识别结果JSON';


--
-- Name: COLUMN merchant_applications.id_card_back_ocr; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_applications.id_card_back_ocr IS '身份证背面OCR识别结果JSON';


--
-- Name: COLUMN merchant_applications.storefront_images; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_applications.storefront_images IS '门头照片URL数组 JSON，最多3张';


--
-- Name: COLUMN merchant_applications.environment_images; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_applications.environment_images IS '店内环境照片URL数组 JSON，最多5张';


--
-- Name: merchant_applications_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.merchant_applications_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: merchant_bosses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.merchant_bosses (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    merchant_id uuid NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone,
    CONSTRAINT merchant_bosses_status_check CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'disabled'::character varying])::text[])))
);


--
-- Name: TABLE merchant_bosses; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.merchant_bosses IS 'Boss 店铺认领关系表 - Boss 可以认领多个店铺，只有分析和员工管理权限';


--
-- Name: COLUMN merchant_bosses.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_bosses.status IS '状态: active=有效, disabled=已解除';


--
-- Name: merchant_bosses_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.merchant_bosses_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: merchant_business_hours; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.merchant_business_hours (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    merchant_id uuid NOT NULL,
    day_of_week integer NOT NULL,
    open_time time without time zone NOT NULL,
    close_time time without time zone NOT NULL,
    is_closed boolean DEFAULT false NOT NULL,
    special_date date,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: COLUMN merchant_business_hours.day_of_week; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_business_hours.day_of_week IS '0=Sunday, 1=Monday, ..., 6=Saturday';


--
-- Name: COLUMN merchant_business_hours.special_date; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_business_hours.special_date IS '特殊日期覆盖常规营业时间，如节假日';


--
-- Name: merchant_business_hours_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.merchant_business_hours_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: merchant_delivery_promotions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.merchant_delivery_promotions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    merchant_id uuid NOT NULL,
    name text NOT NULL,
    min_order_amount bigint NOT NULL,
    discount_amount bigint NOT NULL,
    valid_from timestamp with time zone NOT NULL,
    valid_until timestamp with time zone NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone
);


--
-- Name: TABLE merchant_delivery_promotions; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.merchant_delivery_promotions IS '商户运费满返促销表，门槛式阶梯取最优';


--
-- Name: COLUMN merchant_delivery_promotions.min_order_amount; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_delivery_promotions.min_order_amount IS '最低订单金额（分）';


--
-- Name: COLUMN merchant_delivery_promotions.discount_amount; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_delivery_promotions.discount_amount IS '减免金额（分）';


--
-- Name: merchant_delivery_promotions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.merchant_delivery_promotions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: merchant_dish_categories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.merchant_dish_categories (
    merchant_id uuid NOT NULL,
    category_id uuid NOT NULL,
    sort_order smallint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: merchant_membership_settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.merchant_membership_settings (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    merchant_id uuid NOT NULL,
    balance_usable_scenes text[] DEFAULT ARRAY['dine_in'::text, 'takeout'::text, 'reservation'::text] NOT NULL,
    bonus_usable_scenes text[] DEFAULT ARRAY['dine_in'::text] NOT NULL,
    allow_with_voucher boolean DEFAULT true NOT NULL,
    allow_with_discount boolean DEFAULT true NOT NULL,
    max_deduction_percent integer DEFAULT 100 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone,
    CONSTRAINT merchant_membership_settings_max_deduction_percent_check CHECK (((max_deduction_percent >= 1) AND (max_deduction_percent <= 100)))
);


--
-- Name: TABLE merchant_membership_settings; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.merchant_membership_settings IS '商户会员使用场景配置';


--
-- Name: COLUMN merchant_membership_settings.balance_usable_scenes; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_membership_settings.balance_usable_scenes IS '余额可用场景: dine_in(堂食), takeout(外卖), reservation(预定)';


--
-- Name: COLUMN merchant_membership_settings.bonus_usable_scenes; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_membership_settings.bonus_usable_scenes IS '赠送金额可用场景，可比余额更严格';


--
-- Name: COLUMN merchant_membership_settings.allow_with_voucher; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_membership_settings.allow_with_voucher IS '是否允许与优惠券叠加';


--
-- Name: COLUMN merchant_membership_settings.allow_with_discount; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_membership_settings.allow_with_discount IS '是否允许与满减叠加';


--
-- Name: COLUMN merchant_membership_settings.max_deduction_percent; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_membership_settings.max_deduction_percent IS '单笔最大抵扣比例(1-100)';


--
-- Name: merchant_membership_settings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.merchant_membership_settings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: merchant_memberships; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.merchant_memberships (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    merchant_id uuid NOT NULL,
    user_id uuid NOT NULL,
    balance bigint DEFAULT 0 NOT NULL,
    total_recharged bigint DEFAULT 0 NOT NULL,
    total_consumed bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone,
    CONSTRAINT merchant_memberships_balance_check CHECK ((balance >= 0)),
    CONSTRAINT merchant_memberships_total_consumed_check CHECK ((total_consumed >= 0)),
    CONSTRAINT merchant_memberships_total_recharged_check CHECK ((total_recharged >= 0))
);


--
-- Name: TABLE merchant_memberships; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.merchant_memberships IS 'M10: 商户会员账户表';


--
-- Name: merchant_memberships_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.merchant_memberships_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: merchant_payment_configs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.merchant_payment_configs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    merchant_id uuid NOT NULL,
    sub_mch_id text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE merchant_payment_configs; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.merchant_payment_configs IS '商户微信支付配置（平台收付通）';


--
-- Name: COLUMN merchant_payment_configs.sub_mch_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_payment_configs.sub_mch_id IS '微信平台收付通二级商户号';


--
-- Name: COLUMN merchant_payment_configs.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_payment_configs.status IS '配置状态：active-启用, suspended-暂停';


--
-- Name: merchant_payment_configs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.merchant_payment_configs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: merchant_profiles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.merchant_profiles (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    merchant_id uuid NOT NULL,
    trust_score smallint DEFAULT 850 NOT NULL,
    total_orders integer DEFAULT 0 NOT NULL,
    total_sales bigint DEFAULT 0 NOT NULL,
    completed_orders integer DEFAULT 0 NOT NULL,
    total_claims integer DEFAULT 0 NOT NULL,
    foreign_object_claims integer DEFAULT 0 NOT NULL,
    food_safety_incidents integer DEFAULT 0 NOT NULL,
    timeout_count integer DEFAULT 0 NOT NULL,
    refuse_order_count integer DEFAULT 0 NOT NULL,
    recent_7d_claims integer DEFAULT 0 NOT NULL,
    recent_7d_incidents integer DEFAULT 0 NOT NULL,
    recent_30d_claims integer DEFAULT 0 NOT NULL,
    recent_30d_incidents integer DEFAULT 0 NOT NULL,
    recent_30d_timeouts integer DEFAULT 0 NOT NULL,
    recent_90d_claims integer DEFAULT 0 NOT NULL,
    recent_90d_incidents integer DEFAULT 0 NOT NULL,
    is_suspended boolean DEFAULT false NOT NULL,
    suspend_reason text,
    suspended_at timestamp with time zone,
    suspend_until timestamp with time zone,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT merchant_profiles_trust_score_check CHECK (((trust_score >= 300) AND (trust_score <= 850)))
);


--
-- Name: TABLE merchant_profiles; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.merchant_profiles IS '商户信任画像表 - 信用分驱动食安熔断';


--
-- Name: COLUMN merchant_profiles.trust_score; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_profiles.trust_score IS '商户信任分，400以下停业整顿';


--
-- Name: COLUMN merchant_profiles.foreign_object_claims; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_profiles.foreign_object_claims IS '异物索赔：一周3次触发限流';


--
-- Name: COLUMN merchant_profiles.food_safety_incidents; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_profiles.food_safety_incidents IS '食安事件：需整改和人工审核';


--
-- Name: COLUMN merchant_profiles.is_suspended; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_profiles.is_suspended IS '是否已熔断（食安事件/信用分过低）';


--
-- Name: merchant_profiles_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.merchant_profiles_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: merchant_staff; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.merchant_staff (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    merchant_id uuid NOT NULL,
    user_id uuid NOT NULL,
    role character varying(20) NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    invited_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone,
    CONSTRAINT merchant_staff_role_check CHECK (((role)::text = ANY ((ARRAY['owner'::character varying, 'manager'::character varying, 'chef'::character varying, 'cashier'::character varying, 'pending'::character varying])::text[]))),
    CONSTRAINT merchant_staff_status_check CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'pending'::character varying, 'disabled'::character varying])::text[])))
);


--
-- Name: TABLE merchant_staff; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.merchant_staff IS '商户员工表 - 管理商户与用户的关联关系及角色';


--
-- Name: COLUMN merchant_staff.role; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_staff.role IS '员工角色: owner=店主, manager=店长, chef=厨师长, cashier=收银员, pending=待分配';


--
-- Name: COLUMN merchant_staff.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_staff.status IS '状态: active=启用, pending=待分配权限, disabled=禁用';


--
-- Name: COLUMN merchant_staff.invited_by; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchant_staff.invited_by IS '邀请人（店主ID）';


--
-- Name: merchant_staff_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.merchant_staff_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: merchant_tags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.merchant_tags (
    merchant_id uuid NOT NULL,
    tag_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: merchants; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.merchants (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    owner_user_id uuid NOT NULL,
    name text NOT NULL,
    description text,
    logo_url text,
    phone text NOT NULL,
    address text NOT NULL,
    latitude numeric(10,7),
    longitude numeric(10,7),
    status text DEFAULT 'pending'::text NOT NULL,
    application_data jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    region_id uuid NOT NULL,
    is_open boolean DEFAULT true NOT NULL,
    auto_close_at timestamp with time zone,
    deleted_at timestamp with time zone,
    pending_owner_bind boolean DEFAULT false,
    bind_code character varying(32),
    bind_code_expires_at timestamp with time zone,
    boss_bind_code character varying(32),
    boss_bind_code_expires_at timestamp with time zone,
    CONSTRAINT merchants_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'approved'::text, 'pending_bindbank'::text, 'bindbank_submitted'::text, 'active'::text, 'suspended'::text, 'rejected'::text])))
);


--
-- Name: COLUMN merchants.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchants.status IS 'pending, approved, rejected, suspended';


--
-- Name: COLUMN merchants.application_data; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchants.application_data IS 'JSON存储原始申请数据（营业执照信息等）';


--
-- Name: COLUMN merchants.version; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchants.version IS 'Optimistic locking version for concurrent updates';


--
-- Name: COLUMN merchants.region_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchants.region_id IS '商户所属区域ID，必填，用于多租户隔离和运营商管理';


--
-- Name: COLUMN merchants.is_open; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchants.is_open IS '商户营业状态: true=营业中, false=已打烊';


--
-- Name: COLUMN merchants.auto_close_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchants.auto_close_at IS '自动打烊时间（可选）';


--
-- Name: COLUMN merchants.pending_owner_bind; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchants.pending_owner_bind IS '是否等待老板绑定（店长代入驻时为true）';


--
-- Name: COLUMN merchants.bind_code; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchants.bind_code IS '老板绑定码';


--
-- Name: COLUMN merchants.bind_code_expires_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchants.bind_code_expires_at IS '绑定码过期时间';


--
-- Name: COLUMN merchants.boss_bind_code; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchants.boss_bind_code IS 'Boss 认领码';


--
-- Name: COLUMN merchants.boss_bind_code_expires_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.merchants.boss_bind_code_expires_at IS 'Boss 认领码过期时间';


--
-- Name: merchants_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.merchants_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: notifications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.notifications (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    type text NOT NULL,
    title text NOT NULL,
    content text NOT NULL,
    related_type text,
    related_id uuid,
    extra_data jsonb,
    is_read boolean DEFAULT false NOT NULL,
    read_at timestamp with time zone,
    is_pushed boolean DEFAULT false NOT NULL,
    pushed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone,
    CONSTRAINT notifications_related_type_check CHECK ((related_type = ANY (ARRAY['order'::text, 'payment'::text, 'delivery'::text, 'reservation'::text, 'merchant'::text]))),
    CONSTRAINT notifications_type_check CHECK ((type = ANY (ARRAY['order'::text, 'payment'::text, 'delivery'::text, 'system'::text, 'food_safety'::text])))
);


--
-- Name: TABLE notifications; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.notifications IS '用户通知表，支持WebSocket实时推送';


--
-- Name: COLUMN notifications.type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notifications.type IS '通知类型：order/payment/delivery/system/food_safety';


--
-- Name: COLUMN notifications.related_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notifications.related_type IS '关联实体类型，用于跳转';


--
-- Name: COLUMN notifications.extra_data; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notifications.extra_data IS '扩展数据JSON，用于前端渲染';


--
-- Name: COLUMN notifications.is_pushed; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notifications.is_pushed IS '是否已通过WebSocket推送';


--
-- Name: COLUMN notifications.expires_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.notifications.expires_at IS '过期时间，过期后可删除';


--
-- Name: notifications_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.notifications_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: operator_applications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.operator_applications (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    region_id uuid NOT NULL,
    name text,
    contact_name text,
    contact_phone text,
    business_license_url text,
    business_license_number text,
    business_license_ocr jsonb,
    legal_person_name text,
    legal_person_id_number text,
    id_card_front_url text,
    id_card_back_url text,
    id_card_front_ocr jsonb,
    id_card_back_ocr jsonb,
    requested_contract_years integer DEFAULT 1 NOT NULL,
    status text DEFAULT 'draft'::text NOT NULL,
    reject_reason text,
    reviewed_by uuid,
    reviewed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    submitted_at timestamp with time zone,
    CONSTRAINT operator_applications_status_check CHECK ((status = ANY (ARRAY['draft'::text, 'submitted'::text, 'approved'::text, 'rejected'::text])))
);


--
-- Name: TABLE operator_applications; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.operator_applications IS '运营商入驻申请表，支持草稿保存和人工审核';


--
-- Name: COLUMN operator_applications.region_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operator_applications.region_id IS '申请运营的区县ID，独占（一区一运营商）';


--
-- Name: COLUMN operator_applications.business_license_ocr; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operator_applications.business_license_ocr IS '营业执照OCR识别结果JSON';


--
-- Name: COLUMN operator_applications.id_card_front_ocr; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operator_applications.id_card_front_ocr IS '身份证正面OCR识别结果JSON';


--
-- Name: COLUMN operator_applications.id_card_back_ocr; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operator_applications.id_card_back_ocr IS '身份证背面OCR识别结果JSON';


--
-- Name: COLUMN operator_applications.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operator_applications.status IS 'draft=草稿, submitted=已提交待审核, approved=已通过, rejected=已拒绝';


--
-- Name: operator_applications_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.operator_applications_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: operator_regions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.operator_regions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    operator_id uuid NOT NULL,
    region_id uuid NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT operator_regions_status_check CHECK ((status = ANY (ARRAY['active'::text, 'suspended'::text])))
);


--
-- Name: TABLE operator_regions; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.operator_regions IS '运营商管理的区域列表，支持一个运营商管理多个区县';


--
-- Name: operator_regions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.operator_regions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: operators; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.operators (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    region_id uuid NOT NULL,
    name text NOT NULL,
    contact_name text NOT NULL,
    contact_phone text NOT NULL,
    wechat_mch_id text,
    commission_rate numeric(5,4) DEFAULT 0.0300 NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone,
    contract_start_date date,
    contract_end_date date,
    contract_years integer DEFAULT 1 NOT NULL,
    sub_mch_id text,
    CONSTRAINT operators_commission_rate_check CHECK (((commission_rate >= (0)::numeric) AND (commission_rate <= (1)::numeric))),
    CONSTRAINT operators_status_check CHECK ((status = ANY (ARRAY['active'::text, 'suspended'::text, 'expired'::text])))
);


--
-- Name: COLUMN operators.region_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operators.region_id IS '运营商的主要区域（已废弃UNIQUE约束，现通过operator_regions表管理多区域）';


--
-- Name: COLUMN operators.contract_start_date; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operators.contract_start_date IS '合同开始日期';


--
-- Name: COLUMN operators.contract_end_date; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operators.contract_end_date IS '合同到期日期';


--
-- Name: COLUMN operators.contract_years; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operators.contract_years IS '合同年限（1/2/3年等）';


--
-- Name: COLUMN operators.sub_mch_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operators.sub_mch_id IS '微信平台收付通二级商户号（开户成功后返回）';


--
-- Name: operators_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.operators_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: order_display_configs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.order_display_configs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    merchant_id uuid NOT NULL,
    enable_print boolean DEFAULT true NOT NULL,
    print_takeout boolean DEFAULT true NOT NULL,
    print_dine_in boolean DEFAULT true NOT NULL,
    print_reservation boolean DEFAULT true NOT NULL,
    enable_voice boolean DEFAULT false NOT NULL,
    voice_takeout boolean DEFAULT true NOT NULL,
    voice_dine_in boolean DEFAULT true NOT NULL,
    enable_kds boolean DEFAULT false NOT NULL,
    kds_url text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone
);


--
-- Name: order_display_configs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.order_display_configs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: order_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.order_items (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    order_id uuid NOT NULL,
    dish_id uuid,
    combo_id uuid,
    name text NOT NULL,
    unit_price bigint NOT NULL,
    quantity smallint NOT NULL,
    subtotal bigint NOT NULL,
    customizations jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT order_items_dish_or_combo CHECK ((((dish_id IS NOT NULL) AND (combo_id IS NULL)) OR ((dish_id IS NULL) AND (combo_id IS NOT NULL)))),
    CONSTRAINT order_items_quantity_check CHECK ((quantity > 0)),
    CONSTRAINT order_items_subtotal_check CHECK ((subtotal >= 0)),
    CONSTRAINT order_items_unit_price_check CHECK ((unit_price >= 0))
);


--
-- Name: order_items_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.order_items_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: order_status_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.order_status_logs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    order_id uuid NOT NULL,
    from_status text,
    to_status text NOT NULL,
    operator_id uuid,
    operator_type text,
    notes text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT order_status_logs_operator_type_check CHECK (((operator_type IS NULL) OR (operator_type = ANY (ARRAY['user'::text, 'merchant'::text, 'system'::text]))))
);


--
-- Name: order_status_logs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.order_status_logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: orders; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.orders (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    order_no text NOT NULL,
    user_id uuid NOT NULL,
    merchant_id uuid NOT NULL,
    order_type text NOT NULL,
    address_id uuid,
    delivery_fee bigint DEFAULT 0 NOT NULL,
    delivery_distance integer,
    table_id uuid,
    reservation_id uuid,
    subtotal bigint NOT NULL,
    discount_amount bigint DEFAULT 0 NOT NULL,
    delivery_fee_discount bigint DEFAULT 0 NOT NULL,
    total_amount bigint NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    payment_method text,
    paid_at timestamp with time zone,
    notes text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone,
    completed_at timestamp with time zone,
    cancelled_at timestamp with time zone,
    cancel_reason text,
    final_amount bigint DEFAULT 0,
    platform_commission bigint DEFAULT 0,
    user_voucher_id uuid,
    voucher_amount bigint DEFAULT 0 NOT NULL,
    balance_paid bigint DEFAULT 0 NOT NULL,
    membership_id uuid,
    CONSTRAINT orders_balance_paid_check CHECK ((balance_paid >= 0)),
    CONSTRAINT orders_final_amount_check CHECK ((final_amount >= 0)),
    CONSTRAINT orders_order_type_check CHECK ((order_type = ANY (ARRAY['takeout'::text, 'dine_in'::text, 'takeaway'::text, 'reservation'::text]))),
    CONSTRAINT orders_payment_method_check CHECK (((payment_method IS NULL) OR (payment_method = ANY (ARRAY['wechat'::text, 'balance'::text])))),
    CONSTRAINT orders_platform_commission_check CHECK ((platform_commission >= 0)),
    CONSTRAINT orders_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'paid'::text, 'preparing'::text, 'ready'::text, 'delivering'::text, 'completed'::text, 'cancelled'::text]))),
    CONSTRAINT orders_subtotal_check CHECK ((subtotal >= 0)),
    CONSTRAINT orders_total_amount_check CHECK ((total_amount >= 0)),
    CONSTRAINT orders_voucher_amount_check CHECK ((voucher_amount >= 0))
);


--
-- Name: COLUMN orders.user_voucher_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.orders.user_voucher_id IS '使用的用户优惠券ID';


--
-- Name: COLUMN orders.voucher_amount; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.orders.voucher_amount IS '优惠券抵扣金额(分)';


--
-- Name: COLUMN orders.balance_paid; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.orders.balance_paid IS '会员余额支付金额(分)';


--
-- Name: COLUMN orders.membership_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.orders.membership_id IS '使用的会员卡ID';


--
-- Name: orders_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.orders_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: payment_orders; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.payment_orders (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    order_id uuid,
    reservation_id uuid,
    user_id uuid NOT NULL,
    payment_type text NOT NULL,
    business_type text NOT NULL,
    amount bigint NOT NULL,
    out_trade_no text NOT NULL,
    transaction_id text,
    prepay_id text,
    status text DEFAULT 'pending'::text NOT NULL,
    paid_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone,
    attach text,
    combined_payment_id uuid,
    CONSTRAINT payment_orders_amount_check CHECK ((amount > 0)),
    CONSTRAINT payment_orders_business_type_check CHECK ((business_type = ANY (ARRAY['order'::text, 'deposit'::text, 'recharge'::text, 'reservation'::text]))),
    CONSTRAINT payment_orders_payment_type_check CHECK ((payment_type = ANY (ARRAY['miniprogram'::text, 'profit_sharing'::text]))),
    CONSTRAINT payment_orders_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'paid'::text, 'failed'::text, 'refunded'::text, 'closed'::text])))
);


--
-- Name: COLUMN payment_orders.combined_payment_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.payment_orders.combined_payment_id IS '关联的合单支付ID，单商户支付时为NULL';


--
-- Name: payment_orders_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.payment_orders_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: peak_hour_configs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.peak_hour_configs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    region_id uuid NOT NULL,
    name text NOT NULL,
    start_time time without time zone NOT NULL,
    end_time time without time zone NOT NULL,
    coefficient numeric(3,2) NOT NULL,
    days_of_week smallint[] NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone
);


--
-- Name: TABLE peak_hour_configs; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.peak_hour_configs IS '高峰/特殊时段配置表（午高峰、晚高峰、深夜配送等）';


--
-- Name: COLUMN peak_hour_configs.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.peak_hour_configs.name IS '配置名称，如：午高峰、晚高峰、深夜配送';


--
-- Name: COLUMN peak_hour_configs.end_time; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.peak_hour_configs.end_time IS '结束时间，可小于start_time表示跨天（如22:00-06:00）';


--
-- Name: COLUMN peak_hour_configs.coefficient; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.peak_hour_configs.coefficient IS '运费系数，如1.20表示加价20%';


--
-- Name: COLUMN peak_hour_configs.days_of_week; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.peak_hour_configs.days_of_week IS '生效的星期：1=周一...7=周日';


--
-- Name: peak_hour_configs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.peak_hour_configs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: print_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.print_logs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    order_id uuid NOT NULL,
    printer_id uuid NOT NULL,
    print_content text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    error_message text,
    printed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT print_logs_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'success'::text, 'failed'::text])))
);


--
-- Name: print_logs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.print_logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: profit_sharing_orders; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.profit_sharing_orders (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    payment_order_id uuid NOT NULL,
    merchant_id uuid NOT NULL,
    operator_id uuid,
    order_source text NOT NULL,
    total_amount bigint NOT NULL,
    platform_commission bigint DEFAULT 0 NOT NULL,
    operator_commission bigint DEFAULT 0 NOT NULL,
    merchant_amount bigint NOT NULL,
    out_order_no text NOT NULL,
    sharing_order_id text,
    status text DEFAULT 'pending'::text NOT NULL,
    finished_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    delivery_fee bigint DEFAULT 0 NOT NULL,
    rider_id uuid,
    rider_amount bigint DEFAULT 0 NOT NULL,
    distributable_amount bigint DEFAULT 0 NOT NULL,
    platform_rate integer DEFAULT 2 NOT NULL,
    operator_rate integer DEFAULT 3 NOT NULL,
    CONSTRAINT profit_sharing_orders_amount_check CHECK ((total_amount > 0)),
    CONSTRAINT profit_sharing_orders_commission_check CHECK (((platform_commission >= 0) AND (operator_commission >= 0) AND (merchant_amount >= 0))),
    CONSTRAINT profit_sharing_orders_order_source_check CHECK ((order_source = ANY (ARRAY['takeout'::text, 'dine_in'::text, 'takeaway'::text, 'reservation'::text]))),
    CONSTRAINT profit_sharing_orders_rider_amount_check CHECK (((rider_amount >= 0) AND ((rider_id IS NULL) OR (rider_amount = delivery_fee)))),
    CONSTRAINT profit_sharing_orders_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'processing'::text, 'finished'::text, 'failed'::text])))
);


--
-- Name: COLUMN profit_sharing_orders.delivery_fee; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.profit_sharing_orders.delivery_fee IS '配送费（分），外卖订单专用';


--
-- Name: COLUMN profit_sharing_orders.rider_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.profit_sharing_orders.rider_id IS '骑手ID，配送订单关联';


--
-- Name: COLUMN profit_sharing_orders.rider_amount; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.profit_sharing_orders.rider_amount IS '骑手分账金额（分），等于配送费';


--
-- Name: COLUMN profit_sharing_orders.distributable_amount; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.profit_sharing_orders.distributable_amount IS '可分账金额（分）= total_amount - delivery_fee';


--
-- Name: COLUMN profit_sharing_orders.platform_rate; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.profit_sharing_orders.platform_rate IS '平台分账比例（百分比），默认2%';


--
-- Name: COLUMN profit_sharing_orders.operator_rate; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.profit_sharing_orders.operator_rate IS '运营商分账比例（百分比），默认3%';


--
-- Name: profit_sharing_orders_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.profit_sharing_orders_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: recharge_rules; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.recharge_rules (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    merchant_id uuid NOT NULL,
    recharge_amount bigint NOT NULL,
    bonus_amount bigint NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    valid_from timestamp with time zone NOT NULL,
    valid_until timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone,
    CONSTRAINT check_valid_period CHECK ((valid_until > valid_from)),
    CONSTRAINT recharge_rules_bonus_amount_check CHECK ((bonus_amount >= 0)),
    CONSTRAINT recharge_rules_recharge_amount_check CHECK ((recharge_amount > 0))
);


--
-- Name: TABLE recharge_rules; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.recharge_rules IS 'M10: 充值规则表（充100送20等）';


--
-- Name: recharge_rules_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.recharge_rules_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: recommend_configs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.recommend_configs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(50) NOT NULL,
    distance_weight numeric(3,2) DEFAULT 0.40 NOT NULL,
    route_weight numeric(3,2) DEFAULT 0.30 NOT NULL,
    urgency_weight numeric(3,2) DEFAULT 0.20 NOT NULL,
    profit_weight numeric(3,2) DEFAULT 0.10 NOT NULL,
    max_distance integer DEFAULT 5000 NOT NULL,
    max_results integer DEFAULT 20 NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone
);


--
-- Name: TABLE recommend_configs; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.recommend_configs IS '推荐算法配置';


--
-- Name: COLUMN recommend_configs.max_distance; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.recommend_configs.max_distance IS '最大推荐距离（米）';


--
-- Name: recommend_configs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.recommend_configs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: recommendation_configs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.recommendation_configs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    region_id uuid NOT NULL,
    exploitation_ratio numeric(3,2) DEFAULT 0.60 NOT NULL,
    exploration_ratio numeric(3,2) DEFAULT 0.30 NOT NULL,
    random_ratio numeric(3,2) DEFAULT 0.10 NOT NULL,
    auto_adjust boolean DEFAULT false NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE recommendation_configs; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.recommendation_configs IS '推荐配置表：区域级别EE算法配置';


--
-- Name: COLUMN recommendation_configs.exploitation_ratio; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.recommendation_configs.exploitation_ratio IS '喜好推荐比例 0.00-1.00，默认60%';


--
-- Name: COLUMN recommendation_configs.exploration_ratio; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.recommendation_configs.exploration_ratio IS '探索推荐比例 0.00-1.00，默认30%';


--
-- Name: COLUMN recommendation_configs.random_ratio; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.recommendation_configs.random_ratio IS '随机推荐比例 0.00-1.00，默认10%';


--
-- Name: COLUMN recommendation_configs.auto_adjust; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.recommendation_configs.auto_adjust IS '是否启用自动调整比例（基于成交转化率，M12功能）';


--
-- Name: recommendation_configs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.recommendation_configs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: recommendations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.recommendations (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    dish_ids bigint[],
    combo_ids bigint[],
    merchant_ids bigint[],
    algorithm text NOT NULL,
    score numeric(5,4),
    generated_at timestamp with time zone DEFAULT now() NOT NULL,
    expired_at timestamp with time zone NOT NULL
);


--
-- Name: TABLE recommendations; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.recommendations IS '推荐结果表：缓存生成的推荐结果';


--
-- Name: COLUMN recommendations.algorithm; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.recommendations.algorithm IS '使用的算法：collaborative/content-based/hybrid/ee-algorithm';


--
-- Name: COLUMN recommendations.expired_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.recommendations.expired_at IS '推荐过期时间（通常5分钟后）';


--
-- Name: recommendations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.recommendations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: refund_orders; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.refund_orders (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    payment_order_id uuid NOT NULL,
    refund_type text NOT NULL,
    refund_amount bigint NOT NULL,
    refund_reason text,
    out_refund_no text NOT NULL,
    refund_id text,
    platform_refund bigint,
    operator_refund bigint,
    merchant_refund bigint,
    status text DEFAULT 'pending'::text NOT NULL,
    refunded_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT refund_orders_refund_amount_check CHECK ((refund_amount > 0)),
    CONSTRAINT refund_orders_refund_type_check CHECK ((refund_type = ANY (ARRAY['miniprogram'::text, 'profit_sharing'::text]))),
    CONSTRAINT refund_orders_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'processing'::text, 'success'::text, 'failed'::text, 'closed'::text])))
);


--
-- Name: refund_orders_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.refund_orders_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: regions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.regions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    level smallint NOT NULL,
    parent_id uuid,
    longitude numeric(10,7),
    latitude numeric(10,7),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    qweather_location_id text
);


--
-- Name: COLUMN regions.code; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.regions.code IS '行政区划代码';


--
-- Name: COLUMN regions.level; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.regions.level IS '1=省 2=市 3=区 4=县';


--
-- Name: COLUMN regions.qweather_location_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.regions.qweather_location_id IS '和风天气城市ID，首次查询后缓存';


--
-- Name: regions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.regions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: reservation_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.reservation_items (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    reservation_id uuid NOT NULL,
    dish_id uuid,
    combo_id uuid,
    quantity smallint NOT NULL,
    unit_price bigint NOT NULL,
    total_price bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT reservation_items_dish_or_combo_check CHECK ((((dish_id IS NOT NULL) AND (combo_id IS NULL)) OR ((dish_id IS NULL) AND (combo_id IS NOT NULL)))),
    CONSTRAINT reservation_items_price_check CHECK (((unit_price >= 0) AND (total_price >= 0))),
    CONSTRAINT reservation_items_quantity_check CHECK ((quantity > 0))
);


--
-- Name: reservation_items_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.reservation_items_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: reviews; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.reviews (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    order_id uuid NOT NULL,
    user_id uuid NOT NULL,
    merchant_id uuid NOT NULL,
    content text NOT NULL,
    images text[],
    is_visible boolean DEFAULT true NOT NULL,
    merchant_reply text,
    replied_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE reviews; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.reviews IS '订单评价表';


--
-- Name: COLUMN reviews.order_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.reviews.order_id IS '订单ID，每个订单只能评价一次';


--
-- Name: COLUMN reviews.user_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.reviews.user_id IS '评价用户ID';


--
-- Name: COLUMN reviews.merchant_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.reviews.merchant_id IS '商户ID';


--
-- Name: COLUMN reviews.content; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.reviews.content IS '评价内容';


--
-- Name: COLUMN reviews.images; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.reviews.images IS '评价图片URLs，PostgreSQL数组类型';


--
-- Name: COLUMN reviews.is_visible; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.reviews.is_visible IS '是否可见，低信用用户评价不展示';


--
-- Name: COLUMN reviews.merchant_reply; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.reviews.merchant_reply IS '商户回复内容';


--
-- Name: COLUMN reviews.replied_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.reviews.replied_at IS '回复时间';


--
-- Name: reviews_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.reviews_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: rider_applications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rider_applications (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    real_name text,
    phone text,
    id_card_front_url text,
    id_card_back_url text,
    id_card_ocr jsonb,
    health_cert_url text,
    health_cert_ocr jsonb,
    status text DEFAULT 'draft'::text NOT NULL,
    reject_reason text,
    reviewed_by uuid,
    reviewed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone,
    submitted_at timestamp with time zone,
    CONSTRAINT rider_applications_status_check CHECK ((status = ANY (ARRAY['draft'::text, 'submitted'::text, 'approved'::text, 'rejected'::text])))
);


--
-- Name: TABLE rider_applications; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.rider_applications IS '骑手入驻申请表，支持草稿保存和自动审核';


--
-- Name: COLUMN rider_applications.id_card_ocr; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.rider_applications.id_card_ocr IS '身份证OCR识别结果JSON';


--
-- Name: COLUMN rider_applications.health_cert_ocr; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.rider_applications.health_cert_ocr IS '健康证OCR识别结果JSON';


--
-- Name: COLUMN rider_applications.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.rider_applications.status IS 'draft=草稿, submitted=已提交待审核, approved=已通过, rejected=已拒绝';


--
-- Name: rider_applications_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.rider_applications_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: rider_deposits; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rider_deposits (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    rider_id uuid NOT NULL,
    amount bigint NOT NULL,
    type character varying(20) NOT NULL,
    related_order_id uuid,
    balance_after bigint NOT NULL,
    remark text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT rider_deposits_type_check CHECK (((type)::text = ANY ((ARRAY['deposit'::character varying, 'withdraw'::character varying, 'freeze'::character varying, 'unfreeze'::character varying, 'deduct'::character varying, 'withdraw_rollback'::character varying])::text[])))
);


--
-- Name: TABLE rider_deposits; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.rider_deposits IS '骑手押金流水';


--
-- Name: COLUMN rider_deposits.type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.rider_deposits.type IS '流水类型: deposit=充值, withdraw=提现, freeze=冻结, unfreeze=解冻, deduct=扣款, withdraw_rollback=提现回滚';


--
-- Name: rider_deposits_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.rider_deposits_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: rider_locations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rider_locations (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    rider_id uuid NOT NULL,
    delivery_id uuid,
    longitude numeric(10,7) NOT NULL,
    latitude numeric(10,7) NOT NULL,
    accuracy numeric(6,2),
    speed numeric(6,2),
    heading numeric(5,2),
    recorded_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE rider_locations; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.rider_locations IS '骑手位置记录';


--
-- Name: COLUMN rider_locations.accuracy; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.rider_locations.accuracy IS '定位精度（米）';


--
-- Name: COLUMN rider_locations.speed; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.rider_locations.speed IS '速度 m/s';


--
-- Name: COLUMN rider_locations.heading; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.rider_locations.heading IS '方向角度';


--
-- Name: rider_locations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.rider_locations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: rider_premium_score_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rider_premium_score_logs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    rider_id uuid NOT NULL,
    change_amount smallint NOT NULL,
    old_score smallint NOT NULL,
    new_score smallint NOT NULL,
    change_type character varying(32) NOT NULL,
    related_order_id uuid,
    related_delivery_id uuid,
    remark text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT rider_premium_score_logs_change_type_check CHECK (((change_type)::text = ANY ((ARRAY['normal_order'::character varying, 'premium_order'::character varying, 'timeout'::character varying, 'damage'::character varying, 'adjustment'::character varying])::text[])))
);


--
-- Name: TABLE rider_premium_score_logs; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.rider_premium_score_logs IS '高值单资格积分变更日志表';


--
-- Name: rider_premium_score_logs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.rider_premium_score_logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: rider_profiles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rider_profiles (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    rider_id uuid NOT NULL,
    trust_score smallint DEFAULT 850 NOT NULL,
    total_deliveries integer DEFAULT 0 NOT NULL,
    completed_deliveries integer DEFAULT 0 NOT NULL,
    on_time_deliveries integer DEFAULT 0 NOT NULL,
    delayed_deliveries integer DEFAULT 0 NOT NULL,
    cancelled_deliveries integer DEFAULT 0 NOT NULL,
    total_damage_incidents integer DEFAULT 0 NOT NULL,
    customer_complaints integer DEFAULT 0 NOT NULL,
    timeout_incidents integer DEFAULT 0 NOT NULL,
    recent_7d_damages integer DEFAULT 0 NOT NULL,
    recent_7d_delays integer DEFAULT 0 NOT NULL,
    recent_30d_damages integer DEFAULT 0 NOT NULL,
    recent_30d_delays integer DEFAULT 0 NOT NULL,
    recent_30d_complaints integer DEFAULT 0 NOT NULL,
    recent_90d_damages integer DEFAULT 0 NOT NULL,
    recent_90d_delays integer DEFAULT 0 NOT NULL,
    total_online_hours integer DEFAULT 0 NOT NULL,
    is_suspended boolean DEFAULT false NOT NULL,
    suspend_reason text,
    suspended_at timestamp with time zone,
    suspend_until timestamp with time zone,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    premium_score smallint DEFAULT 0 NOT NULL,
    CONSTRAINT rider_profiles_trust_score_check CHECK (((trust_score >= 300) AND (trust_score <= 850)))
);


--
-- Name: TABLE rider_profiles; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.rider_profiles IS '骑手信任画像表 - 餐损索赔由押金扣除';


--
-- Name: COLUMN rider_profiles.trust_score; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.rider_profiles.trust_score IS '骑手信任分，350以下暂停接单';


--
-- Name: COLUMN rider_profiles.total_damage_incidents; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.rider_profiles.total_damage_incidents IS '餐损事件：一周3次触发扣分';


--
-- Name: COLUMN rider_profiles.is_suspended; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.rider_profiles.is_suspended IS '是否暂停接单';


--
-- Name: COLUMN rider_profiles.premium_score; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.rider_profiles.premium_score IS '高值单资格积分：普通单+1，高值单-3，超时-5，餐损-10，≥0可接高值单';


--
-- Name: rider_profiles_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.rider_profiles_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: riders; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.riders (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    real_name character varying(50) NOT NULL,
    id_card_no character varying(18) NOT NULL,
    phone character varying(20) NOT NULL,
    deposit_amount bigint DEFAULT 0 NOT NULL,
    frozen_deposit bigint DEFAULT 0 NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    is_online boolean DEFAULT false NOT NULL,
    credit_score smallint DEFAULT 100 NOT NULL,
    current_longitude numeric(10,7),
    current_latitude numeric(10,7),
    location_updated_at timestamp with time zone,
    total_orders integer DEFAULT 0 NOT NULL,
    total_earnings bigint DEFAULT 0 NOT NULL,
    online_duration integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone,
    region_id uuid,
    application_id uuid,
    sub_mch_id text,
    CONSTRAINT riders_credit_score_check CHECK (((credit_score >= 0) AND (credit_score <= 100))),
    CONSTRAINT riders_deposit_check CHECK (((deposit_amount >= 0) AND (frozen_deposit >= 0))),
    CONSTRAINT riders_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'approved'::character varying, 'pending_bindbank'::character varying, 'bindbank_submitted'::character varying, 'active'::character varying, 'suspended'::character varying, 'rejected'::character varying])::text[])))
);


--
-- Name: TABLE riders; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.riders IS '骑手表';


--
-- Name: COLUMN riders.deposit_amount; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.riders.deposit_amount IS '可用押金余额（分）';


--
-- Name: COLUMN riders.frozen_deposit; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.riders.frozen_deposit IS '冻结押金（分）';


--
-- Name: COLUMN riders.online_duration; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.riders.online_duration IS '累计在线时长（秒）';


--
-- Name: COLUMN riders.region_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.riders.region_id IS '骑手所属区域ID，用于多租户隔离，骑手只能接该区域内的订单';


--
-- Name: COLUMN riders.application_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.riders.application_id IS '关联的入驻申请ID，审核通过后填充';


--
-- Name: COLUMN riders.sub_mch_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.riders.sub_mch_id IS '微信平台收付通二级商户号';


--
-- Name: riders_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.riders_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version bigint NOT NULL,
    dirty boolean NOT NULL
);


--
-- Name: sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sessions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    access_token text NOT NULL,
    refresh_token text NOT NULL,
    access_token_expires_at timestamp with time zone NOT NULL,
    refresh_token_expires_at timestamp with time zone NOT NULL,
    user_agent text NOT NULL,
    client_ip text NOT NULL,
    is_revoked boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: sessions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.sessions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: table_images; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.table_images (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    table_id uuid NOT NULL,
    image_url text NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    is_primary boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE table_images; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.table_images IS '桌台/包间图片表';


--
-- Name: COLUMN table_images.table_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.table_images.table_id IS '桌台ID';


--
-- Name: COLUMN table_images.image_url; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.table_images.image_url IS '图片URL';


--
-- Name: COLUMN table_images.sort_order; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.table_images.sort_order IS '排序顺序';


--
-- Name: COLUMN table_images.is_primary; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.table_images.is_primary IS '是否为主图';


--
-- Name: table_images_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.table_images_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: table_reservations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.table_reservations (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    table_id uuid NOT NULL,
    user_id uuid NOT NULL,
    merchant_id uuid NOT NULL,
    reservation_date date NOT NULL,
    reservation_time time without time zone NOT NULL,
    guest_count smallint NOT NULL,
    contact_name text NOT NULL,
    contact_phone text NOT NULL,
    payment_mode text DEFAULT 'deposit'::text NOT NULL,
    deposit_amount bigint DEFAULT 0 NOT NULL,
    prepaid_amount bigint DEFAULT 0 NOT NULL,
    refund_deadline timestamp with time zone NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    payment_deadline timestamp with time zone NOT NULL,
    notes text,
    paid_at timestamp with time zone,
    confirmed_at timestamp with time zone,
    completed_at timestamp with time zone,
    cancelled_at timestamp with time zone,
    cancel_reason text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone,
    checked_in_at timestamp with time zone,
    cooking_started_at timestamp with time zone,
    source character varying(20) DEFAULT 'online'::character varying,
    CONSTRAINT table_reservations_amounts_check CHECK (((deposit_amount >= 0) AND (prepaid_amount >= 0))),
    CONSTRAINT table_reservations_guest_count_check CHECK ((guest_count > 0)),
    CONSTRAINT table_reservations_payment_mode_check CHECK ((payment_mode = ANY (ARRAY['deposit'::text, 'full'::text]))),
    CONSTRAINT table_reservations_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'paid'::text, 'confirmed'::text, 'checked_in'::text, 'completed'::text, 'cancelled'::text, 'expired'::text, 'no_show'::text])))
);


--
-- Name: COLUMN table_reservations.checked_in_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.table_reservations.checked_in_at IS '顾客到店签到时间';


--
-- Name: COLUMN table_reservations.cooking_started_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.table_reservations.cooking_started_at IS '厨房开始制作时间';


--
-- Name: COLUMN table_reservations.source; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.table_reservations.source IS '预订来源：online(线上)、phone(电话)、walkin(现场)、merchant(商户代订)';


--
-- Name: table_reservations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.table_reservations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: table_tags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.table_tags (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    table_id uuid NOT NULL,
    tag_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: table_tags_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.table_tags_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: tables; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tables (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    merchant_id uuid NOT NULL,
    table_no text NOT NULL,
    table_type text DEFAULT 'table'::text NOT NULL,
    capacity smallint NOT NULL,
    description text,
    minimum_spend bigint,
    qr_code_url text,
    status text DEFAULT 'available'::text NOT NULL,
    current_reservation_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone,
    CONSTRAINT tables_capacity_check CHECK ((capacity > 0)),
    CONSTRAINT tables_minimum_spend_check CHECK (((minimum_spend IS NULL) OR (minimum_spend >= 0))),
    CONSTRAINT tables_status_check CHECK ((status = ANY (ARRAY['available'::text, 'occupied'::text, 'disabled'::text, 'reserved'::text]))),
    CONSTRAINT tables_table_type_check CHECK ((table_type = ANY (ARRAY['table'::text, 'room'::text])))
);


--
-- Name: tables_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.tables_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: tags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tags (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    type text NOT NULL,
    sort_order smallint DEFAULT 0 NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: COLUMN tags.type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.tags.type IS 'merchant, product, service, etc.';


--
-- Name: tags_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.tags_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: trust_score_changes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.trust_score_changes (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    entity_type text NOT NULL,
    entity_id uuid NOT NULL,
    old_score smallint NOT NULL,
    new_score smallint NOT NULL,
    score_change smallint NOT NULL,
    reason_type text NOT NULL,
    reason_description text NOT NULL,
    related_type text,
    related_id uuid,
    is_auto boolean DEFAULT true NOT NULL,
    operator_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT trust_score_changes_entity_type_check CHECK ((entity_type = ANY (ARRAY['customer'::text, 'merchant'::text, 'rider'::text])))
);


--
-- Name: TABLE trust_score_changes; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.trust_score_changes IS 'TrustScore变更记录表（审计日志）- 所有信用分变更可追溯';


--
-- Name: COLUMN trust_score_changes.score_change; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.trust_score_changes.score_change IS '变化值：负数=扣分，正数=加分（第一版不实现加分）';


--
-- Name: COLUMN trust_score_changes.reason_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.trust_score_changes.reason_type IS '变更原因类型：用于统计分析';


--
-- Name: COLUMN trust_score_changes.is_auto; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.trust_score_changes.is_auto IS '系统自动变更 vs 人工调整';


--
-- Name: trust_score_changes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.trust_score_changes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: user_addresses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_addresses (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    region_id uuid NOT NULL,
    detail_address text NOT NULL,
    contact_name text NOT NULL,
    contact_phone text NOT NULL,
    longitude numeric(10,7) NOT NULL,
    latitude numeric(10,7) NOT NULL,
    is_default boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: user_addresses_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_addresses_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: user_balance_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_balance_logs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    type character varying(32) NOT NULL,
    amount bigint NOT NULL,
    balance_before bigint NOT NULL,
    balance_after bigint NOT NULL,
    related_type character varying(32),
    related_id uuid,
    source_type character varying(32),
    source_id uuid,
    remark text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE user_balance_logs; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.user_balance_logs IS '用户余额变动日志';


--
-- Name: COLUMN user_balance_logs.type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_balance_logs.type IS '变动类型：claim_refund/order_pay/withdraw/recharge/adjustment';


--
-- Name: COLUMN user_balance_logs.source_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_balance_logs.source_type IS '资金来源：rider_deposit/merchant_refund/platform';


--
-- Name: user_balance_logs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_balance_logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: user_balances; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_balances (
    user_id uuid NOT NULL,
    balance bigint DEFAULT 0 NOT NULL,
    frozen_balance bigint DEFAULT 0 NOT NULL,
    total_income bigint DEFAULT 0 NOT NULL,
    total_expense bigint DEFAULT 0 NOT NULL,
    total_withdraw bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT user_balances_balance_check CHECK ((balance >= 0)),
    CONSTRAINT user_balances_frozen_balance_check CHECK ((frozen_balance >= 0))
);


--
-- Name: TABLE user_balances; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.user_balances IS '用户余额账户';


--
-- Name: COLUMN user_balances.balance; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_balances.balance IS '可用余额（分）';


--
-- Name: COLUMN user_balances.frozen_balance; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_balances.frozen_balance IS '冻结余额，提现处理中（分）';


--
-- Name: user_behaviors; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_behaviors (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    behavior_type text NOT NULL,
    dish_id uuid,
    combo_id uuid,
    merchant_id uuid,
    duration integer,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE user_behaviors; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.user_behaviors IS '用户行为埋点表：浏览、详情、加购、购买';


--
-- Name: COLUMN user_behaviors.behavior_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_behaviors.behavior_type IS 'view/detail/cart/purchase - 浏览列表/查看详情/加购/购买';


--
-- Name: COLUMN user_behaviors.duration; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_behaviors.duration IS '停留时长(秒)，仅view/detail行为有值';


--
-- Name: user_behaviors_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_behaviors_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: user_claim_warnings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_claim_warnings (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    warning_count integer DEFAULT 0 NOT NULL,
    last_warning_at timestamp with time zone,
    last_warning_reason text,
    requires_evidence boolean DEFAULT false NOT NULL,
    platform_pay_count integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE user_claim_warnings; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.user_claim_warnings IS '用户索赔警告状态表，记录用户的索赔行为模式';


--
-- Name: COLUMN user_claim_warnings.warning_count; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_claim_warnings.warning_count IS '被警告次数：5单3索赔首次警告+1';


--
-- Name: COLUMN user_claim_warnings.requires_evidence; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_claim_warnings.requires_evidence IS '是否需要提交证据：被警告后再次索赔时需要';


--
-- Name: COLUMN user_claim_warnings.platform_pay_count; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_claim_warnings.platform_pay_count IS '平台垫付次数：问题用户的索赔由平台承担';


--
-- Name: user_claim_warnings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_claim_warnings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: user_devices; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_devices (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    device_id text NOT NULL,
    device_type text NOT NULL,
    device_model text,
    os_version text,
    app_version text,
    user_agent text,
    ip_address text,
    last_login_at timestamp with time zone DEFAULT now() NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    first_seen timestamp with time zone DEFAULT now() NOT NULL,
    last_seen timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE user_devices; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.user_devices IS '用户设备指纹表，用于检测设备复用欺诈';


--
-- Name: COLUMN user_devices.device_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_devices.device_id IS '设备指纹，可以是设备IMEI、UUID或浏览器指纹（TEXT类型，遵循PostgreSQL最佳实践）';


--
-- Name: COLUMN user_devices.device_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_devices.device_type IS '设备类型：ios/android/web等';


--
-- Name: user_devices_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_devices_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: user_notification_preferences; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_notification_preferences (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    enable_order_notifications boolean DEFAULT true NOT NULL,
    enable_payment_notifications boolean DEFAULT true NOT NULL,
    enable_delivery_notifications boolean DEFAULT true NOT NULL,
    enable_system_notifications boolean DEFAULT true NOT NULL,
    enable_food_safety_notifications boolean DEFAULT true NOT NULL,
    do_not_disturb_start time without time zone,
    do_not_disturb_end time without time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone
);


--
-- Name: TABLE user_notification_preferences; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.user_notification_preferences IS '用户通知偏好设置';


--
-- Name: COLUMN user_notification_preferences.do_not_disturb_start; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_notification_preferences.do_not_disturb_start IS '免打扰开始时间';


--
-- Name: COLUMN user_notification_preferences.do_not_disturb_end; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_notification_preferences.do_not_disturb_end IS '免打扰结束时间';


--
-- Name: user_notification_preferences_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_notification_preferences_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: user_preferences; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_preferences (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    cuisine_preferences jsonb,
    price_range_min bigint,
    price_range_max bigint,
    avg_order_amount bigint,
    favorite_time_slots integer[],
    purchase_frequency smallint DEFAULT 0 NOT NULL,
    last_order_date date,
    top_cuisines jsonb,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE user_preferences; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.user_preferences IS '用户偏好表：基于行为和消费分析';


--
-- Name: COLUMN user_preferences.cuisine_preferences; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_preferences.cuisine_preferences IS 'JSON格式：{"川菜": 0.8, "粤菜": 0.6} - 菜系偏好得分';


--
-- Name: COLUMN user_preferences.favorite_time_slots; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_preferences.favorite_time_slots IS 'PostgreSQL数组：[11,12,18,19] - 常下单时段(小时)';


--
-- Name: COLUMN user_preferences.top_cuisines; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_preferences.top_cuisines IS 'JSON格式：{"川菜": 15, "粤菜": 8} - 购买次数统计';


--
-- Name: user_preferences_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_preferences_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: user_profiles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_profiles (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    role text NOT NULL,
    trust_score smallint DEFAULT 850 NOT NULL,
    total_orders integer DEFAULT 0 NOT NULL,
    completed_orders integer DEFAULT 0 NOT NULL,
    cancelled_orders integer DEFAULT 0 NOT NULL,
    total_claims integer DEFAULT 0 NOT NULL,
    malicious_claims integer DEFAULT 0 NOT NULL,
    food_safety_reports integer DEFAULT 0 NOT NULL,
    verified_violations integer DEFAULT 0 NOT NULL,
    recent_7d_claims integer DEFAULT 0 NOT NULL,
    recent_7d_orders integer DEFAULT 0 NOT NULL,
    recent_30d_claims integer DEFAULT 0 NOT NULL,
    recent_30d_orders integer DEFAULT 0 NOT NULL,
    recent_30d_cancels integer DEFAULT 0 NOT NULL,
    recent_90d_claims integer DEFAULT 0 NOT NULL,
    recent_90d_orders integer DEFAULT 0 NOT NULL,
    is_blacklisted boolean DEFAULT false NOT NULL,
    blacklist_reason text,
    blacklisted_at timestamp with time zone,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT user_profiles_role_check CHECK ((role = ANY (ARRAY['customer'::text, 'merchant'::text, 'rider'::text]))),
    CONSTRAINT user_profiles_trust_score_check CHECK (((trust_score >= 300) AND (trust_score <= 850)))
);


--
-- Name: TABLE user_profiles; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.user_profiles IS '用户信任画像表（顾客）- 信用驱动异常处理';


--
-- Name: COLUMN user_profiles.trust_score; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_profiles.trust_score IS '信任分，初始850（高信任），只有负面行为才扣分';


--
-- Name: COLUMN user_profiles.recent_7d_claims; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_profiles.recent_7d_claims IS '近7天索赔次数（快速识别异常）';


--
-- Name: COLUMN user_profiles.recent_30d_claims; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_profiles.recent_30d_claims IS '近30天索赔次数（回溯检查）';


--
-- Name: COLUMN user_profiles.recent_90d_claims; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_profiles.recent_90d_claims IS '近90天索赔次数（长期趋势）';


--
-- Name: user_profiles_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_profiles_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: user_roles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_roles (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    role text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    related_entity_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: COLUMN user_roles.role; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_roles.role IS 'customer/merchant/rider/operator/staff';


--
-- Name: COLUMN user_roles.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_roles.status IS 'active/suspended';


--
-- Name: COLUMN user_roles.related_entity_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_roles.related_entity_id IS '关联实体ID（如商户ID、骑手ID等）';


--
-- Name: user_roles_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_roles_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: user_vouchers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_vouchers (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    voucher_id uuid NOT NULL,
    user_id uuid NOT NULL,
    status text DEFAULT 'unused'::text NOT NULL,
    order_id uuid,
    used_at timestamp with time zone,
    obtained_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    CONSTRAINT user_vouchers_status_check CHECK ((status = ANY (ARRAY['unused'::text, 'used'::text, 'expired'::text])))
);


--
-- Name: TABLE user_vouchers; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.user_vouchers IS 'M10: 用户代金券表';


--
-- Name: user_vouchers_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_vouchers_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    wechat_openid text NOT NULL,
    wechat_unionid text,
    full_name text NOT NULL,
    phone text,
    avatar_url text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: COLUMN users.wechat_openid; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.wechat_openid IS '微信小程序唯一标识';


--
-- Name: COLUMN users.wechat_unionid; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.wechat_unionid IS '微信开放平台unionid';


--
-- Name: COLUMN users.phone; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.phone IS '手机号，可选绑定';


--
-- Name: COLUMN users.avatar_url; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.users.avatar_url IS '微信头像URL';


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: vouchers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.vouchers (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    merchant_id uuid NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    description text,
    amount bigint NOT NULL,
    min_order_amount bigint DEFAULT 0 NOT NULL,
    total_quantity integer NOT NULL,
    claimed_quantity integer DEFAULT 0 NOT NULL,
    used_quantity integer DEFAULT 0 NOT NULL,
    valid_from timestamp with time zone NOT NULL,
    valid_until timestamp with time zone NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone,
    allowed_order_types text[] DEFAULT ARRAY['takeout'::text, 'dine_in'::text, 'takeaway'::text, 'reservation'::text] NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT check_voucher_quantities CHECK (((claimed_quantity <= total_quantity) AND (used_quantity <= claimed_quantity))),
    CONSTRAINT check_voucher_valid_period CHECK ((valid_until > valid_from)),
    CONSTRAINT vouchers_amount_check CHECK ((amount > 0)),
    CONSTRAINT vouchers_claimed_quantity_check CHECK ((claimed_quantity >= 0)),
    CONSTRAINT vouchers_min_order_amount_check CHECK ((min_order_amount >= 0)),
    CONSTRAINT vouchers_total_quantity_check CHECK ((total_quantity > 0)),
    CONSTRAINT vouchers_used_quantity_check CHECK ((used_quantity >= 0))
);


--
-- Name: TABLE vouchers; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.vouchers IS 'M10: 代金券模板表';


--
-- Name: COLUMN vouchers.allowed_order_types; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.vouchers.allowed_order_types IS '允许使用的订单类型: takeout(外卖), dine_in(堂食), takeaway(外带), reservation(预定)';


--
-- Name: vouchers_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.vouchers_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: weather_coefficients; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.weather_coefficients (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    region_id uuid NOT NULL,
    recorded_at timestamp with time zone NOT NULL,
    weather_data jsonb,
    warning_data jsonb,
    weather_type text NOT NULL,
    weather_code text,
    temperature smallint,
    feels_like smallint,
    humidity smallint,
    wind_speed smallint,
    wind_scale text,
    precip numeric(5,2),
    visibility smallint,
    has_warning boolean DEFAULT false NOT NULL,
    warning_type text,
    warning_level text,
    warning_severity text,
    warning_text text,
    weather_coefficient numeric(3,2) DEFAULT 1.00 NOT NULL,
    warning_coefficient numeric(3,2) DEFAULT 1.00 NOT NULL,
    final_coefficient numeric(3,2) DEFAULT 1.00 NOT NULL,
    delivery_suspended boolean DEFAULT false NOT NULL,
    suspend_reason text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE weather_coefficients; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.weather_coefficients IS '天气系数记录表，定时抓取和风天气数据';


--
-- Name: COLUMN weather_coefficients.weather_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.weather_coefficients.weather_type IS 'sunny/cloudy/rainy/heavy_rain/snowy/extreme';


--
-- Name: COLUMN weather_coefficients.weather_code; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.weather_coefficients.weather_code IS '和风天气图标代码';


--
-- Name: COLUMN weather_coefficients.wind_speed; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.weather_coefficients.wind_speed IS '风速（km/h）';


--
-- Name: COLUMN weather_coefficients.precip; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.weather_coefficients.precip IS '降水量（mm）';


--
-- Name: COLUMN weather_coefficients.visibility; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.weather_coefficients.visibility IS '能见度（km）';


--
-- Name: COLUMN weather_coefficients.has_warning; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.weather_coefficients.has_warning IS '是否有预警';


--
-- Name: COLUMN weather_coefficients.warning_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.weather_coefficients.warning_type IS '预警类型代码，如1001=台风, 1003=暴雨';


--
-- Name: COLUMN weather_coefficients.warning_level; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.weather_coefficients.warning_level IS '预警等级: blue/yellow/orange/red';


--
-- Name: COLUMN weather_coefficients.warning_severity; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.weather_coefficients.warning_severity IS '严重程度: minor/moderate/severe/extreme';


--
-- Name: COLUMN weather_coefficients.weather_coefficient; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.weather_coefficients.weather_coefficient IS '天气系数';


--
-- Name: COLUMN weather_coefficients.warning_coefficient; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.weather_coefficients.warning_coefficient IS '预警系数';


--
-- Name: COLUMN weather_coefficients.final_coefficient; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.weather_coefficients.final_coefficient IS '最终系数 = max(天气系数, 预警系数)';


--
-- Name: COLUMN weather_coefficients.delivery_suspended; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.weather_coefficients.delivery_suspended IS '是否暂停配送（极端天气）';


--
-- Name: weather_coefficients_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.weather_coefficients_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: wechat_access_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.wechat_access_tokens (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    app_type text NOT NULL,
    access_token text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: COLUMN wechat_access_tokens.app_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.wechat_access_tokens.app_type IS 'miniprogram/official-account';


--
-- Name: wechat_access_tokens_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.wechat_access_tokens_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
--



--
-- Name: wechat_notifications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.wechat_notifications (
    id character varying(64) NOT NULL,
    event_type character varying(64) NOT NULL,
    resource_type character varying(64),
    summary text,
    out_trade_no character varying(64),
    transaction_id character varying(64),
    processed_at timestamp without time zone DEFAULT now() NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE wechat_notifications; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.wechat_notifications IS '微信支付回调通知记录表，用于防止重复处理（幂等性）';


--
-- Name: COLUMN wechat_notifications.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.wechat_notifications.id IS '微信通知ID，微信保证全局唯一';


--
-- Name: COLUMN wechat_notifications.event_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.wechat_notifications.event_type IS '通知事件类型';


--
-- Name: COLUMN wechat_notifications.out_trade_no; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.wechat_notifications.out_trade_no IS '商户订单号，用于关联业务订单';


--
-- Name: COLUMN wechat_notifications.processed_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.wechat_notifications.processed_at IS '通知处理完成时间';


--
-- Name: appeals id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: browse_history id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: cart_items id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: carts id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: claims id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: cloud_printers id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: combined_payment_orders id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: combined_payment_sub_orders id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: combo_dishes id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: combo_sets id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: combo_tags id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: daily_inventory id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: deliveries id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: delivery_fee_configs id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: delivery_pool id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: discount_rules id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: dish_categories id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: dish_customization_groups id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: dish_customization_options id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: dish_ingredients id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: dish_tags id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: dishes id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: ecommerce_applyments id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: favorites id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: food_safety_incidents id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: fraud_patterns id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: ingredients id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: membership_transactions id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: merchant_applications id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: merchant_bosses id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: merchant_business_hours id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: merchant_delivery_promotions id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: merchant_membership_settings id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: merchant_memberships id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: merchant_payment_configs id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: merchant_profiles id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: merchant_staff id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: merchants id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: notifications id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: operator_applications id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: operator_regions id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: operators id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: order_display_configs id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: order_items id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: order_status_logs id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: orders id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: payment_orders id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: peak_hour_configs id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: print_logs id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: profit_sharing_orders id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: recharge_rules id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: recommend_configs id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: recommendation_configs id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: recommendations id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: refund_orders id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: regions id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: reservation_items id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: reviews id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: rider_applications id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: rider_deposits id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: rider_locations id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: rider_premium_score_logs id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: rider_profiles id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: riders id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: sessions id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: table_images id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: table_reservations id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: table_tags id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: tables id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: tags id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: trust_score_changes id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: user_addresses id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: user_balance_logs id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: user_behaviors id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: user_claim_warnings id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: user_devices id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: user_notification_preferences id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: user_preferences id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: user_profiles id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: user_roles id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: user_vouchers id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: users id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: vouchers id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: weather_coefficients id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: wechat_access_tokens id; Type: DEFAULT; Schema: public; Owner: -
--



--
-- Name: appeals appeals_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.appeals
    ADD CONSTRAINT appeals_pkey PRIMARY KEY (id);


--
-- Name: browse_history browse_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.browse_history
    ADD CONSTRAINT browse_history_pkey PRIMARY KEY (id);


--
-- Name: cart_items cart_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cart_items
    ADD CONSTRAINT cart_items_pkey PRIMARY KEY (id);


--
-- Name: carts carts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.carts
    ADD CONSTRAINT carts_pkey PRIMARY KEY (id);


--
-- Name: claims claims_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.claims
    ADD CONSTRAINT claims_pkey PRIMARY KEY (id);


--
-- Name: cloud_printers cloud_printers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cloud_printers
    ADD CONSTRAINT cloud_printers_pkey PRIMARY KEY (id);


--
-- Name: cloud_printers cloud_printers_printer_sn_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cloud_printers
    ADD CONSTRAINT cloud_printers_printer_sn_key UNIQUE (printer_sn);


--
-- Name: combined_payment_orders combined_payment_orders_combine_out_trade_no_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.combined_payment_orders
    ADD CONSTRAINT combined_payment_orders_combine_out_trade_no_key UNIQUE (combine_out_trade_no);


--
-- Name: combined_payment_orders combined_payment_orders_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.combined_payment_orders
    ADD CONSTRAINT combined_payment_orders_pkey PRIMARY KEY (id);


--
-- Name: combined_payment_sub_orders combined_payment_sub_orders_out_trade_no_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.combined_payment_sub_orders
    ADD CONSTRAINT combined_payment_sub_orders_out_trade_no_key UNIQUE (out_trade_no);


--
-- Name: combined_payment_sub_orders combined_payment_sub_orders_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.combined_payment_sub_orders
    ADD CONSTRAINT combined_payment_sub_orders_pkey PRIMARY KEY (id);


--
-- Name: combo_dishes combo_dishes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.combo_dishes
    ADD CONSTRAINT combo_dishes_pkey PRIMARY KEY (id);


--
-- Name: combo_sets combo_sets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.combo_sets
    ADD CONSTRAINT combo_sets_pkey PRIMARY KEY (id);


--
-- Name: combo_tags combo_tags_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.combo_tags
    ADD CONSTRAINT combo_tags_pkey PRIMARY KEY (id);


--
-- Name: daily_inventory daily_inventory_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.daily_inventory
    ADD CONSTRAINT daily_inventory_pkey PRIMARY KEY (id);


--
-- Name: deliveries deliveries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.deliveries
    ADD CONSTRAINT deliveries_pkey PRIMARY KEY (id);


--
-- Name: delivery_fee_configs delivery_fee_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.delivery_fee_configs
    ADD CONSTRAINT delivery_fee_configs_pkey PRIMARY KEY (id);


--
-- Name: delivery_pool delivery_pool_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.delivery_pool
    ADD CONSTRAINT delivery_pool_pkey PRIMARY KEY (id);


--
-- Name: discount_rules discount_rules_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discount_rules
    ADD CONSTRAINT discount_rules_pkey PRIMARY KEY (id);


--
-- Name: dish_categories dish_categories_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dish_categories
    ADD CONSTRAINT dish_categories_name_key UNIQUE (name);


--
-- Name: dish_categories dish_categories_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dish_categories
    ADD CONSTRAINT dish_categories_pkey PRIMARY KEY (id);


--
-- Name: dish_customization_groups dish_customization_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dish_customization_groups
    ADD CONSTRAINT dish_customization_groups_pkey PRIMARY KEY (id);


--
-- Name: dish_customization_options dish_customization_options_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dish_customization_options
    ADD CONSTRAINT dish_customization_options_pkey PRIMARY KEY (id);


--
-- Name: dish_ingredients dish_ingredients_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dish_ingredients
    ADD CONSTRAINT dish_ingredients_pkey PRIMARY KEY (id);


--
-- Name: dish_tags dish_tags_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dish_tags
    ADD CONSTRAINT dish_tags_pkey PRIMARY KEY (id);


--
-- Name: dishes dishes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dishes
    ADD CONSTRAINT dishes_pkey PRIMARY KEY (id);


--
-- Name: ecommerce_applyments ecommerce_applyments_out_request_no_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ecommerce_applyments
    ADD CONSTRAINT ecommerce_applyments_out_request_no_key UNIQUE (out_request_no);


--
-- Name: ecommerce_applyments ecommerce_applyments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ecommerce_applyments
    ADD CONSTRAINT ecommerce_applyments_pkey PRIMARY KEY (id);


--
-- Name: favorites favorites_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.favorites
    ADD CONSTRAINT favorites_pkey PRIMARY KEY (id);


--
-- Name: food_safety_incidents food_safety_incidents_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.food_safety_incidents
    ADD CONSTRAINT food_safety_incidents_pkey PRIMARY KEY (id);


--
-- Name: fraud_patterns fraud_patterns_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fraud_patterns
    ADD CONSTRAINT fraud_patterns_pkey PRIMARY KEY (id);


--
-- Name: ingredients ingredients_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ingredients
    ADD CONSTRAINT ingredients_pkey PRIMARY KEY (id);


--
-- Name: membership_transactions membership_transactions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.membership_transactions
    ADD CONSTRAINT membership_transactions_pkey PRIMARY KEY (id);


--
-- Name: merchant_applications merchant_applications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_applications
    ADD CONSTRAINT merchant_applications_pkey PRIMARY KEY (id);


--
-- Name: merchant_bosses merchant_bosses_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_bosses
    ADD CONSTRAINT merchant_bosses_pkey PRIMARY KEY (id);


--
-- Name: merchant_bosses merchant_bosses_user_id_merchant_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_bosses
    ADD CONSTRAINT merchant_bosses_user_id_merchant_id_key UNIQUE (user_id, merchant_id);


--
-- Name: merchant_business_hours merchant_business_hours_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_business_hours
    ADD CONSTRAINT merchant_business_hours_pkey PRIMARY KEY (id);


--
-- Name: merchant_delivery_promotions merchant_delivery_promotions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_delivery_promotions
    ADD CONSTRAINT merchant_delivery_promotions_pkey PRIMARY KEY (id);


--
-- Name: merchant_dish_categories merchant_dish_categories_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_dish_categories
    ADD CONSTRAINT merchant_dish_categories_pkey PRIMARY KEY (merchant_id, category_id);


--
-- Name: merchant_membership_settings merchant_membership_settings_merchant_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_membership_settings
    ADD CONSTRAINT merchant_membership_settings_merchant_id_key UNIQUE (merchant_id);


--
-- Name: merchant_membership_settings merchant_membership_settings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_membership_settings
    ADD CONSTRAINT merchant_membership_settings_pkey PRIMARY KEY (id);


--
-- Name: merchant_memberships merchant_memberships_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_memberships
    ADD CONSTRAINT merchant_memberships_pkey PRIMARY KEY (id);


--
-- Name: merchant_payment_configs merchant_payment_configs_merchant_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_payment_configs
    ADD CONSTRAINT merchant_payment_configs_merchant_id_key UNIQUE (merchant_id);


--
-- Name: merchant_payment_configs merchant_payment_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_payment_configs
    ADD CONSTRAINT merchant_payment_configs_pkey PRIMARY KEY (id);


--
-- Name: merchant_profiles merchant_profiles_merchant_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_profiles
    ADD CONSTRAINT merchant_profiles_merchant_id_key UNIQUE (merchant_id);


--
-- Name: merchant_profiles merchant_profiles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_profiles
    ADD CONSTRAINT merchant_profiles_pkey PRIMARY KEY (id);


--
-- Name: merchant_staff merchant_staff_merchant_id_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_staff
    ADD CONSTRAINT merchant_staff_merchant_id_user_id_key UNIQUE (merchant_id, user_id);


--
-- Name: merchant_staff merchant_staff_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_staff
    ADD CONSTRAINT merchant_staff_pkey PRIMARY KEY (id);


--
-- Name: merchant_tags merchant_tags_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_tags
    ADD CONSTRAINT merchant_tags_pkey PRIMARY KEY (merchant_id, tag_id);


--
-- Name: merchants merchants_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchants
    ADD CONSTRAINT merchants_pkey PRIMARY KEY (id);


--
-- Name: notifications notifications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_pkey PRIMARY KEY (id);


--
-- Name: operator_applications operator_applications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.operator_applications
    ADD CONSTRAINT operator_applications_pkey PRIMARY KEY (id);


--
-- Name: operator_applications operator_applications_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.operator_applications
    ADD CONSTRAINT operator_applications_user_id_key UNIQUE (user_id);


--
-- Name: operator_regions operator_regions_operator_id_region_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.operator_regions
    ADD CONSTRAINT operator_regions_operator_id_region_id_key UNIQUE (operator_id, region_id);


--
-- Name: operator_regions operator_regions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.operator_regions
    ADD CONSTRAINT operator_regions_pkey PRIMARY KEY (id);


--
-- Name: operators operators_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.operators
    ADD CONSTRAINT operators_pkey PRIMARY KEY (id);


--
-- Name: operators operators_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.operators
    ADD CONSTRAINT operators_user_id_key UNIQUE (user_id);


--
-- Name: order_display_configs order_display_configs_merchant_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_display_configs
    ADD CONSTRAINT order_display_configs_merchant_id_key UNIQUE (merchant_id);


--
-- Name: order_display_configs order_display_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_display_configs
    ADD CONSTRAINT order_display_configs_pkey PRIMARY KEY (id);


--
-- Name: order_items order_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_items
    ADD CONSTRAINT order_items_pkey PRIMARY KEY (id);


--
-- Name: order_status_logs order_status_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_status_logs
    ADD CONSTRAINT order_status_logs_pkey PRIMARY KEY (id);


--
-- Name: orders orders_order_no_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_order_no_key UNIQUE (order_no);


--
-- Name: orders orders_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_pkey PRIMARY KEY (id);


--
-- Name: payment_orders payment_orders_out_trade_no_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_orders
    ADD CONSTRAINT payment_orders_out_trade_no_key UNIQUE (out_trade_no);


--
-- Name: payment_orders payment_orders_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_orders
    ADD CONSTRAINT payment_orders_pkey PRIMARY KEY (id);


--
-- Name: peak_hour_configs peak_hour_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.peak_hour_configs
    ADD CONSTRAINT peak_hour_configs_pkey PRIMARY KEY (id);


--
-- Name: print_logs print_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.print_logs
    ADD CONSTRAINT print_logs_pkey PRIMARY KEY (id);


--
-- Name: profit_sharing_orders profit_sharing_orders_out_order_no_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.profit_sharing_orders
    ADD CONSTRAINT profit_sharing_orders_out_order_no_key UNIQUE (out_order_no);


--
-- Name: profit_sharing_orders profit_sharing_orders_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.profit_sharing_orders
    ADD CONSTRAINT profit_sharing_orders_pkey PRIMARY KEY (id);


--
-- Name: recharge_rules recharge_rules_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.recharge_rules
    ADD CONSTRAINT recharge_rules_pkey PRIMARY KEY (id);


--
-- Name: recommend_configs recommend_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.recommend_configs
    ADD CONSTRAINT recommend_configs_pkey PRIMARY KEY (id);


--
-- Name: recommendation_configs recommendation_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.recommendation_configs
    ADD CONSTRAINT recommendation_configs_pkey PRIMARY KEY (id);


--
-- Name: recommendation_configs recommendation_configs_region_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.recommendation_configs
    ADD CONSTRAINT recommendation_configs_region_id_key UNIQUE (region_id);


--
-- Name: recommendations recommendations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.recommendations
    ADD CONSTRAINT recommendations_pkey PRIMARY KEY (id);


--
-- Name: refund_orders refund_orders_out_refund_no_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.refund_orders
    ADD CONSTRAINT refund_orders_out_refund_no_key UNIQUE (out_refund_no);


--
-- Name: refund_orders refund_orders_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.refund_orders
    ADD CONSTRAINT refund_orders_pkey PRIMARY KEY (id);


--
-- Name: regions regions_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.regions
    ADD CONSTRAINT regions_code_key UNIQUE (code);


--
-- Name: regions regions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.regions
    ADD CONSTRAINT regions_pkey PRIMARY KEY (id);


--
-- Name: reservation_items reservation_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reservation_items
    ADD CONSTRAINT reservation_items_pkey PRIMARY KEY (id);


--
-- Name: reviews reviews_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviews
    ADD CONSTRAINT reviews_pkey PRIMARY KEY (id);


--
-- Name: rider_applications rider_applications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rider_applications
    ADD CONSTRAINT rider_applications_pkey PRIMARY KEY (id);


--
-- Name: rider_applications rider_applications_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rider_applications
    ADD CONSTRAINT rider_applications_user_id_key UNIQUE (user_id);


--
-- Name: rider_deposits rider_deposits_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rider_deposits
    ADD CONSTRAINT rider_deposits_pkey PRIMARY KEY (id);


--
-- Name: rider_locations rider_locations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rider_locations
    ADD CONSTRAINT rider_locations_pkey PRIMARY KEY (id);


--
-- Name: rider_premium_score_logs rider_premium_score_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rider_premium_score_logs
    ADD CONSTRAINT rider_premium_score_logs_pkey PRIMARY KEY (id);


--
-- Name: rider_profiles rider_profiles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rider_profiles
    ADD CONSTRAINT rider_profiles_pkey PRIMARY KEY (id);


--
-- Name: rider_profiles rider_profiles_rider_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rider_profiles
    ADD CONSTRAINT rider_profiles_rider_id_key UNIQUE (rider_id);


--
-- Name: riders riders_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.riders
    ADD CONSTRAINT riders_pkey PRIMARY KEY (id);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: sessions sessions_access_token_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT sessions_access_token_key UNIQUE (access_token);


--
-- Name: sessions sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT sessions_pkey PRIMARY KEY (id);


--
-- Name: sessions sessions_refresh_token_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT sessions_refresh_token_key UNIQUE (refresh_token);


--
-- Name: table_images table_images_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.table_images
    ADD CONSTRAINT table_images_pkey PRIMARY KEY (id);


--
-- Name: table_reservations table_reservations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.table_reservations
    ADD CONSTRAINT table_reservations_pkey PRIMARY KEY (id);


--
-- Name: table_tags table_tags_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.table_tags
    ADD CONSTRAINT table_tags_pkey PRIMARY KEY (id);


--
-- Name: tables tables_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tables
    ADD CONSTRAINT tables_pkey PRIMARY KEY (id);


--
-- Name: tags tags_name_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tags
    ADD CONSTRAINT tags_name_unique UNIQUE (name);


--
-- Name: tags tags_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tags
    ADD CONSTRAINT tags_pkey PRIMARY KEY (id);


--
-- Name: trust_score_changes trust_score_changes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trust_score_changes
    ADD CONSTRAINT trust_score_changes_pkey PRIMARY KEY (id);


--
-- Name: merchant_memberships unique_merchant_user; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_memberships
    ADD CONSTRAINT unique_merchant_user UNIQUE (merchant_id, user_id);


--
-- Name: user_profiles unique_user_role; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_profiles
    ADD CONSTRAINT unique_user_role UNIQUE (user_id, role);


--
-- Name: browse_history unique_user_target; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.browse_history
    ADD CONSTRAINT unique_user_target UNIQUE (user_id, target_type, target_id);


--
-- Name: user_addresses user_addresses_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_addresses
    ADD CONSTRAINT user_addresses_pkey PRIMARY KEY (id);


--
-- Name: user_balance_logs user_balance_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_balance_logs
    ADD CONSTRAINT user_balance_logs_pkey PRIMARY KEY (id);


--
-- Name: user_balances user_balances_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_balances
    ADD CONSTRAINT user_balances_pkey PRIMARY KEY (user_id);


--
-- Name: user_behaviors user_behaviors_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_behaviors
    ADD CONSTRAINT user_behaviors_pkey PRIMARY KEY (id);


--
-- Name: user_claim_warnings user_claim_warnings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_claim_warnings
    ADD CONSTRAINT user_claim_warnings_pkey PRIMARY KEY (id);


--
-- Name: user_claim_warnings user_claim_warnings_user_id_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_claim_warnings
    ADD CONSTRAINT user_claim_warnings_user_id_unique UNIQUE (user_id);


--
-- Name: user_devices user_devices_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_devices
    ADD CONSTRAINT user_devices_pkey PRIMARY KEY (id);


--
-- Name: user_devices user_devices_user_id_device_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_devices
    ADD CONSTRAINT user_devices_user_id_device_id_key UNIQUE (user_id, device_id);


--
-- Name: user_notification_preferences user_notification_preferences_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_notification_preferences
    ADD CONSTRAINT user_notification_preferences_pkey PRIMARY KEY (id);


--
-- Name: user_notification_preferences user_notification_preferences_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_notification_preferences
    ADD CONSTRAINT user_notification_preferences_user_id_key UNIQUE (user_id);


--
-- Name: user_preferences user_preferences_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_preferences
    ADD CONSTRAINT user_preferences_pkey PRIMARY KEY (id);


--
-- Name: user_preferences user_preferences_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_preferences
    ADD CONSTRAINT user_preferences_user_id_key UNIQUE (user_id);


--
-- Name: user_profiles user_profiles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_profiles
    ADD CONSTRAINT user_profiles_pkey PRIMARY KEY (id);


--
-- Name: user_roles user_roles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_roles
    ADD CONSTRAINT user_roles_pkey PRIMARY KEY (id);


--
-- Name: user_vouchers user_vouchers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_vouchers
    ADD CONSTRAINT user_vouchers_pkey PRIMARY KEY (id);


--
-- Name: users users_phone_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_phone_key UNIQUE (phone);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: users users_wechat_openid_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_wechat_openid_key UNIQUE (wechat_openid);


--
-- Name: users users_wechat_unionid_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_wechat_unionid_key UNIQUE (wechat_unionid);


--
-- Name: vouchers vouchers_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vouchers
    ADD CONSTRAINT vouchers_code_key UNIQUE (code);


--
-- Name: vouchers vouchers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vouchers
    ADD CONSTRAINT vouchers_pkey PRIMARY KEY (id);


--
-- Name: weather_coefficients weather_coefficients_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.weather_coefficients
    ADD CONSTRAINT weather_coefficients_pkey PRIMARY KEY (id);


--
-- Name: wechat_access_tokens wechat_access_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wechat_access_tokens
    ADD CONSTRAINT wechat_access_tokens_pkey PRIMARY KEY (id);


--
-- Name: wechat_notifications wechat_notifications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wechat_notifications
    ADD CONSTRAINT wechat_notifications_pkey PRIMARY KEY (id);


--
-- Name: cart_items_cart_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX cart_items_cart_id_idx ON public.cart_items USING btree (cart_id);


--
-- Name: cart_items_combo_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX cart_items_combo_id_idx ON public.cart_items USING btree (combo_id);


--
-- Name: cart_items_dish_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX cart_items_dish_id_idx ON public.cart_items USING btree (dish_id);


--
-- Name: carts_merchant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX carts_merchant_id_idx ON public.carts USING btree (merchant_id);


--
-- Name: carts_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX carts_user_id_idx ON public.carts USING btree (user_id);


--
-- Name: carts_user_id_merchant_id_order_type_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX carts_user_id_merchant_id_order_type_idx ON public.carts USING btree (user_id, merchant_id, order_type, table_id, reservation_id);


--
-- Name: cloud_printers_merchant_active_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX cloud_printers_merchant_active_idx ON public.cloud_printers USING btree (merchant_id, is_active);


--
-- Name: cloud_printers_merchant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX cloud_printers_merchant_id_idx ON public.cloud_printers USING btree (merchant_id);


--
-- Name: cloud_printers_printer_sn_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX cloud_printers_printer_sn_idx ON public.cloud_printers USING btree (printer_sn);


--
-- Name: combined_payment_orders_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX combined_payment_orders_created_at_idx ON public.combined_payment_orders USING btree (created_at);


--
-- Name: combined_payment_orders_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX combined_payment_orders_status_idx ON public.combined_payment_orders USING btree (status);


--
-- Name: combined_payment_orders_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX combined_payment_orders_user_id_idx ON public.combined_payment_orders USING btree (user_id);


--
-- Name: combined_payment_sub_orders_combined_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX combined_payment_sub_orders_combined_id_idx ON public.combined_payment_sub_orders USING btree (combined_payment_id);


--
-- Name: combined_payment_sub_orders_merchant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX combined_payment_sub_orders_merchant_id_idx ON public.combined_payment_sub_orders USING btree (merchant_id);


--
-- Name: combined_payment_sub_orders_order_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX combined_payment_sub_orders_order_id_idx ON public.combined_payment_sub_orders USING btree (order_id);


--
-- Name: combo_dishes_combo_id_dish_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX combo_dishes_combo_id_dish_id_idx ON public.combo_dishes USING btree (combo_id, dish_id);


--
-- Name: combo_dishes_combo_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX combo_dishes_combo_id_idx ON public.combo_dishes USING btree (combo_id);


--
-- Name: combo_sets_merchant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX combo_sets_merchant_id_idx ON public.combo_sets USING btree (merchant_id);


--
-- Name: combo_sets_merchant_id_is_online_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX combo_sets_merchant_id_is_online_idx ON public.combo_sets USING btree (merchant_id, is_online);


--
-- Name: combo_tags_combo_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX combo_tags_combo_id_idx ON public.combo_tags USING btree (combo_id);


--
-- Name: combo_tags_combo_id_tag_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX combo_tags_combo_id_tag_id_idx ON public.combo_tags USING btree (combo_id, tag_id);


--
-- Name: combo_tags_tag_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX combo_tags_tag_id_idx ON public.combo_tags USING btree (tag_id);


--
-- Name: daily_inventory_date_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX daily_inventory_date_idx ON public.daily_inventory USING btree (date);


--
-- Name: daily_inventory_merchant_id_dish_id_date_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX daily_inventory_merchant_id_dish_id_date_idx ON public.daily_inventory USING btree (merchant_id, dish_id, date);


--
-- Name: daily_inventory_merchant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX daily_inventory_merchant_id_idx ON public.daily_inventory USING btree (merchant_id);


--
-- Name: deliveries_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX deliveries_created_at_idx ON public.deliveries USING btree (created_at);


--
-- Name: deliveries_order_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX deliveries_order_id_idx ON public.deliveries USING btree (order_id);


--
-- Name: deliveries_rider_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX deliveries_rider_id_idx ON public.deliveries USING btree (rider_id);


--
-- Name: deliveries_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX deliveries_status_idx ON public.deliveries USING btree (status);


--
-- Name: delivery_fee_configs_is_active_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX delivery_fee_configs_is_active_idx ON public.delivery_fee_configs USING btree (is_active);


--
-- Name: delivery_fee_configs_region_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX delivery_fee_configs_region_id_idx ON public.delivery_fee_configs USING btree (region_id);


--
-- Name: delivery_pool_expires_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX delivery_pool_expires_at_idx ON public.delivery_pool USING btree (expires_at);


--
-- Name: delivery_pool_order_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX delivery_pool_order_id_idx ON public.delivery_pool USING btree (order_id);


--
-- Name: delivery_pool_pickup_location_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX delivery_pool_pickup_location_idx ON public.delivery_pool USING btree (pickup_longitude, pickup_latitude);


--
-- Name: delivery_pool_priority_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX delivery_pool_priority_idx ON public.delivery_pool USING btree (priority);


--
-- Name: dish_customization_groups_dish_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dish_customization_groups_dish_id_idx ON public.dish_customization_groups USING btree (dish_id);


--
-- Name: dish_customization_groups_dish_id_sort_order_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dish_customization_groups_dish_id_sort_order_idx ON public.dish_customization_groups USING btree (dish_id, sort_order);


--
-- Name: dish_customization_options_group_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dish_customization_options_group_id_idx ON public.dish_customization_options USING btree (group_id);


--
-- Name: dish_customization_options_group_id_sort_order_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dish_customization_options_group_id_sort_order_idx ON public.dish_customization_options USING btree (group_id, sort_order);


--
-- Name: dish_customization_options_group_id_tag_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX dish_customization_options_group_id_tag_id_idx ON public.dish_customization_options USING btree (group_id, tag_id);


--
-- Name: dish_ingredients_dish_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dish_ingredients_dish_id_idx ON public.dish_ingredients USING btree (dish_id);


--
-- Name: dish_ingredients_dish_id_ingredient_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX dish_ingredients_dish_id_ingredient_id_idx ON public.dish_ingredients USING btree (dish_id, ingredient_id);


--
-- Name: dish_ingredients_ingredient_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dish_ingredients_ingredient_id_idx ON public.dish_ingredients USING btree (ingredient_id);


--
-- Name: dish_tags_dish_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dish_tags_dish_id_idx ON public.dish_tags USING btree (dish_id);


--
-- Name: dish_tags_dish_id_tag_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX dish_tags_dish_id_tag_id_idx ON public.dish_tags USING btree (dish_id, tag_id);


--
-- Name: dish_tags_tag_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dish_tags_tag_id_idx ON public.dish_tags USING btree (tag_id);


--
-- Name: dishes_category_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dishes_category_id_idx ON public.dishes USING btree (category_id);


--
-- Name: dishes_merchant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dishes_merchant_id_idx ON public.dishes USING btree (merchant_id);


--
-- Name: dishes_merchant_id_is_online_is_available_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dishes_merchant_id_is_online_is_available_idx ON public.dishes USING btree (merchant_id, is_online, is_available);


--
-- Name: dishes_name_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dishes_name_idx ON public.dishes USING btree (name);


--
-- Name: ecommerce_applyments_applyment_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ecommerce_applyments_applyment_id_idx ON public.ecommerce_applyments USING btree (applyment_id);


--
-- Name: ecommerce_applyments_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ecommerce_applyments_status_idx ON public.ecommerce_applyments USING btree (status);


--
-- Name: ecommerce_applyments_sub_mch_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ecommerce_applyments_sub_mch_id_idx ON public.ecommerce_applyments USING btree (sub_mch_id);


--
-- Name: ecommerce_applyments_subject_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ecommerce_applyments_subject_idx ON public.ecommerce_applyments USING btree (subject_type, subject_id);


--
-- Name: favorites_dish_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX favorites_dish_id_idx ON public.favorites USING btree (dish_id) WHERE (dish_id IS NOT NULL);


--
-- Name: favorites_merchant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX favorites_merchant_id_idx ON public.favorites USING btree (merchant_id) WHERE (merchant_id IS NOT NULL);


--
-- Name: favorites_user_id_favorite_type_dish_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX favorites_user_id_favorite_type_dish_id_idx ON public.favorites USING btree (user_id, favorite_type, dish_id) WHERE (dish_id IS NOT NULL);


--
-- Name: favorites_user_id_favorite_type_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX favorites_user_id_favorite_type_idx ON public.favorites USING btree (user_id, favorite_type);


--
-- Name: favorites_user_id_favorite_type_merchant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX favorites_user_id_favorite_type_merchant_id_idx ON public.favorites USING btree (user_id, favorite_type, merchant_id) WHERE (merchant_id IS NOT NULL);


--
-- Name: idx_appeals_appellant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_appeals_appellant ON public.appeals USING btree (appellant_type, appellant_id);


--
-- Name: idx_appeals_claim_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_appeals_claim_id ON public.appeals USING btree (claim_id);


--
-- Name: idx_appeals_claim_id_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_appeals_claim_id_unique ON public.appeals USING btree (claim_id);


--
-- Name: INDEX idx_appeals_claim_id_unique; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.idx_appeals_claim_id_unique IS '确保每个索赔只能有一个申诉';


--
-- Name: idx_appeals_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_appeals_created_at ON public.appeals USING btree (created_at);


--
-- Name: idx_appeals_pending_region; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_appeals_pending_region ON public.appeals USING btree (region_id, status) WHERE (status = 'pending'::text);


--
-- Name: idx_appeals_region_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_appeals_region_id ON public.appeals USING btree (region_id);


--
-- Name: idx_appeals_reviewer_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_appeals_reviewer_id ON public.appeals USING btree (reviewer_id) WHERE (reviewer_id IS NOT NULL);


--
-- Name: idx_appeals_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_appeals_status ON public.appeals USING btree (status);


--
-- Name: idx_appeals_unique_claim; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_appeals_unique_claim ON public.appeals USING btree (claim_id);


--
-- Name: idx_browse_history_target; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_browse_history_target ON public.browse_history USING btree (target_type, target_id);


--
-- Name: idx_browse_history_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_browse_history_user ON public.browse_history USING btree (user_id);


--
-- Name: idx_browse_history_user_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_browse_history_user_time ON public.browse_history USING btree (user_id, last_viewed_at DESC);


--
-- Name: idx_claims_approval_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_claims_approval_type ON public.claims USING btree (approval_type);


--
-- Name: idx_claims_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_claims_created_at ON public.claims USING btree (created_at);


--
-- Name: idx_claims_malicious; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_claims_malicious ON public.claims USING btree (is_malicious);


--
-- Name: idx_claims_order_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_claims_order_id ON public.claims USING btree (order_id);


--
-- Name: idx_claims_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_claims_status ON public.claims USING btree (status);


--
-- Name: idx_claims_trust_snapshot; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_claims_trust_snapshot ON public.claims USING btree (trust_score_snapshot);


--
-- Name: idx_claims_user_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_claims_user_created ON public.claims USING btree (user_id, created_at);


--
-- Name: idx_claims_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_claims_user_id ON public.claims USING btree (user_id);


--
-- Name: idx_claims_user_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_claims_user_status ON public.claims USING btree (user_id, status);


--
-- Name: idx_combo_sets_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_combo_sets_deleted_at ON public.combo_sets USING btree (deleted_at) WHERE (deleted_at IS NULL);


--
-- Name: idx_discount_rules_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_discount_rules_deleted_at ON public.discount_rules USING btree (deleted_at) WHERE (deleted_at IS NULL);


--
-- Name: idx_discount_rules_merchant_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_discount_rules_merchant_active ON public.discount_rules USING btree (merchant_id, is_active);


--
-- Name: idx_discount_rules_merchant_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_discount_rules_merchant_id ON public.discount_rules USING btree (merchant_id);


--
-- Name: idx_discount_rules_valid_period; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_discount_rules_valid_period ON public.discount_rules USING btree (valid_from, valid_until);


--
-- Name: idx_dish_categories_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dish_categories_deleted_at ON public.dish_categories USING btree (deleted_at) WHERE (deleted_at IS NULL);


--
-- Name: idx_dishes_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dishes_deleted_at ON public.dishes USING btree (deleted_at) WHERE (deleted_at IS NULL);


--
-- Name: idx_food_safety_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_food_safety_created_at ON public.food_safety_incidents USING btree (created_at);


--
-- Name: idx_food_safety_merchant_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_food_safety_merchant_id ON public.food_safety_incidents USING btree (merchant_id);


--
-- Name: idx_food_safety_merchant_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_food_safety_merchant_status ON public.food_safety_incidents USING btree (merchant_id, status);


--
-- Name: idx_food_safety_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_food_safety_status ON public.food_safety_incidents USING btree (status);


--
-- Name: idx_food_safety_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_food_safety_user_id ON public.food_safety_incidents USING btree (user_id);


--
-- Name: idx_fraud_patterns_confirmed; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_fraud_patterns_confirmed ON public.fraud_patterns USING btree (is_confirmed);


--
-- Name: idx_fraud_patterns_detected; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_fraud_patterns_detected ON public.fraud_patterns USING btree (detected_at);


--
-- Name: idx_fraud_patterns_match_count; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_fraud_patterns_match_count ON public.fraud_patterns USING btree (match_count);


--
-- Name: idx_fraud_patterns_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_fraud_patterns_type ON public.fraud_patterns USING btree (pattern_type);


--
-- Name: idx_membership_transactions_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_membership_transactions_created_at ON public.membership_transactions USING btree (created_at);


--
-- Name: idx_membership_transactions_membership_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_membership_transactions_membership_created ON public.membership_transactions USING btree (membership_id, created_at DESC);


--
-- Name: idx_membership_transactions_membership_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_membership_transactions_membership_id ON public.membership_transactions USING btree (membership_id);


--
-- Name: idx_membership_transactions_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_membership_transactions_type ON public.membership_transactions USING btree (type);


--
-- Name: idx_merchant_applications_region_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_merchant_applications_region_id ON public.merchant_applications USING btree (region_id);


--
-- Name: idx_merchant_bosses_merchant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_merchant_bosses_merchant ON public.merchant_bosses USING btree (merchant_id);


--
-- Name: idx_merchant_bosses_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_merchant_bosses_user ON public.merchant_bosses USING btree (user_id);


--
-- Name: idx_merchant_membership_settings_merchant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_merchant_membership_settings_merchant ON public.merchant_membership_settings USING btree (merchant_id);


--
-- Name: idx_merchant_memberships_balance; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_merchant_memberships_balance ON public.merchant_memberships USING btree (balance);


--
-- Name: idx_merchant_memberships_merchant_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_merchant_memberships_merchant_id ON public.merchant_memberships USING btree (merchant_id);


--
-- Name: idx_merchant_memberships_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_merchant_memberships_user_id ON public.merchant_memberships USING btree (user_id);


--
-- Name: idx_merchant_profiles_suspended; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_merchant_profiles_suspended ON public.merchant_profiles USING btree (is_suspended);


--
-- Name: idx_merchant_profiles_trust_score; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_merchant_profiles_trust_score ON public.merchant_profiles USING btree (trust_score);


--
-- Name: idx_merchant_staff_merchant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_merchant_staff_merchant ON public.merchant_staff USING btree (merchant_id);


--
-- Name: idx_merchant_staff_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_merchant_staff_user ON public.merchant_staff USING btree (user_id);


--
-- Name: idx_merchants_address_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_merchants_address_unique ON public.merchants USING btree (address) WHERE (deleted_at IS NULL);


--
-- Name: INDEX idx_merchants_address_unique; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.idx_merchants_address_unique IS '同一地址只能注册一家餐厅';


--
-- Name: idx_merchants_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_merchants_deleted_at ON public.merchants USING btree (deleted_at) WHERE (deleted_at IS NULL);


--
-- Name: idx_merchants_is_open; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_merchants_is_open ON public.merchants USING btree (is_open);


--
-- Name: idx_merchants_region_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_merchants_region_id ON public.merchants USING btree (region_id);


--
-- Name: idx_notification_prefs_user; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_notification_prefs_user ON public.user_notification_preferences USING btree (user_id);


--
-- Name: idx_notifications_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_created ON public.notifications USING btree (created_at DESC);


--
-- Name: idx_notifications_related; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_related ON public.notifications USING btree (related_type, related_id) WHERE (related_type IS NOT NULL);


--
-- Name: idx_notifications_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_type ON public.notifications USING btree (type);


--
-- Name: idx_notifications_user_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_user_created ON public.notifications USING btree (user_id, created_at DESC);


--
-- Name: idx_notifications_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_user_id ON public.notifications USING btree (user_id);


--
-- Name: idx_notifications_user_read; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_user_read ON public.notifications USING btree (user_id, is_read);


--
-- Name: idx_operator_regions_operator_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_operator_regions_operator_id ON public.operator_regions USING btree (operator_id);


--
-- Name: idx_operator_regions_region_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_operator_regions_region_id ON public.operator_regions USING btree (region_id);


--
-- Name: idx_operator_regions_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_operator_regions_status ON public.operator_regions USING btree (status);


--
-- Name: idx_order_items_dish_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_order_items_dish_created ON public.order_items USING btree (dish_id, created_at);


--
-- Name: idx_orders_final_amount; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_orders_final_amount ON public.orders USING btree (final_amount);


--
-- Name: idx_orders_merchant_created_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_orders_merchant_created_status ON public.orders USING btree (merchant_id, created_at, status);


--
-- Name: idx_orders_user_merchant_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_orders_user_merchant_created ON public.orders USING btree (user_id, merchant_id, created_at) WHERE (status = ANY (ARRAY['delivered'::text, 'completed'::text]));


--
-- Name: idx_profit_sharing_orders_merchant_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_profit_sharing_orders_merchant_created ON public.profit_sharing_orders USING btree (merchant_id, created_at);


--
-- Name: idx_profit_sharing_orders_merchant_status_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_profit_sharing_orders_merchant_status_created ON public.profit_sharing_orders USING btree (merchant_id, status, created_at);


--
-- Name: idx_recharge_rules_merchant_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_recharge_rules_merchant_active ON public.recharge_rules USING btree (merchant_id, is_active);


--
-- Name: idx_recharge_rules_merchant_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_recharge_rules_merchant_id ON public.recharge_rules USING btree (merchant_id);


--
-- Name: idx_recharge_rules_valid_period; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_recharge_rules_valid_period ON public.recharge_rules USING btree (valid_from, valid_until);


--
-- Name: idx_reviews_merchant_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_reviews_merchant_id ON public.reviews USING btree (merchant_id);


--
-- Name: idx_reviews_merchant_visible_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_reviews_merchant_visible_created ON public.reviews USING btree (merchant_id, is_visible, created_at);


--
-- Name: idx_reviews_order_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_reviews_order_id ON public.reviews USING btree (order_id);


--
-- Name: idx_reviews_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_reviews_user_id ON public.reviews USING btree (user_id);


--
-- Name: idx_rider_premium_score_logs_change_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_rider_premium_score_logs_change_type ON public.rider_premium_score_logs USING btree (change_type);


--
-- Name: idx_rider_premium_score_logs_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_rider_premium_score_logs_created_at ON public.rider_premium_score_logs USING btree (created_at);


--
-- Name: idx_rider_premium_score_logs_rider_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_rider_premium_score_logs_rider_id ON public.rider_premium_score_logs USING btree (rider_id);


--
-- Name: idx_rider_profiles_suspended; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_rider_profiles_suspended ON public.rider_profiles USING btree (is_suspended);


--
-- Name: idx_rider_profiles_trust_score; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_rider_profiles_trust_score ON public.rider_profiles USING btree (trust_score);


--
-- Name: idx_riders_region_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_riders_region_id ON public.riders USING btree (region_id);


--
-- Name: idx_table_images_primary; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_table_images_primary ON public.table_images USING btree (table_id) WHERE (is_primary = true);


--
-- Name: idx_table_images_table_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_table_images_table_id ON public.table_images USING btree (table_id);


--
-- Name: idx_trust_score_changes_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_trust_score_changes_created ON public.trust_score_changes USING btree (created_at);


--
-- Name: idx_trust_score_changes_entity; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_trust_score_changes_entity ON public.trust_score_changes USING btree (entity_type, entity_id);


--
-- Name: idx_trust_score_changes_entity_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_trust_score_changes_entity_created ON public.trust_score_changes USING btree (entity_type, entity_id, created_at);


--
-- Name: idx_trust_score_changes_reason; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_trust_score_changes_reason ON public.trust_score_changes USING btree (reason_type);


--
-- Name: idx_user_balance_logs_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_balance_logs_created_at ON public.user_balance_logs USING btree (created_at);


--
-- Name: idx_user_balance_logs_related; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_balance_logs_related ON public.user_balance_logs USING btree (related_type, related_id);


--
-- Name: idx_user_balance_logs_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_balance_logs_type ON public.user_balance_logs USING btree (type);


--
-- Name: idx_user_balance_logs_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_balance_logs_user_id ON public.user_balance_logs USING btree (user_id);


--
-- Name: idx_user_claim_warnings_requires_evidence; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_claim_warnings_requires_evidence ON public.user_claim_warnings USING btree (requires_evidence) WHERE (requires_evidence = true);


--
-- Name: idx_user_claim_warnings_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_claim_warnings_user_id ON public.user_claim_warnings USING btree (user_id);


--
-- Name: idx_user_devices_device_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_devices_device_id ON public.user_devices USING btree (device_id);


--
-- Name: idx_user_devices_user_device; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_user_devices_user_device ON public.user_devices USING btree (user_id, device_id);


--
-- Name: idx_user_devices_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_devices_user_id ON public.user_devices USING btree (user_id);


--
-- Name: idx_user_profiles_blacklisted; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_profiles_blacklisted ON public.user_profiles USING btree (is_blacklisted);


--
-- Name: idx_user_profiles_trust_score; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_profiles_trust_score ON public.user_profiles USING btree (trust_score);


--
-- Name: idx_user_profiles_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_profiles_user_id ON public.user_profiles USING btree (user_id);


--
-- Name: idx_user_vouchers_expires_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_vouchers_expires_at ON public.user_vouchers USING btree (expires_at);


--
-- Name: idx_user_vouchers_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_vouchers_status ON public.user_vouchers USING btree (status);


--
-- Name: idx_user_vouchers_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_vouchers_user_id ON public.user_vouchers USING btree (user_id);


--
-- Name: idx_user_vouchers_user_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_vouchers_user_status ON public.user_vouchers USING btree (user_id, status);


--
-- Name: idx_user_vouchers_voucher_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_vouchers_voucher_id ON public.user_vouchers USING btree (voucher_id);


--
-- Name: idx_user_vouchers_voucher_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_vouchers_voucher_status ON public.user_vouchers USING btree (voucher_id, status);


--
-- Name: idx_vouchers_code; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_vouchers_code ON public.vouchers USING btree (code);


--
-- Name: idx_vouchers_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_vouchers_deleted_at ON public.vouchers USING btree (deleted_at) WHERE (deleted_at IS NULL);


--
-- Name: idx_vouchers_merchant_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_vouchers_merchant_active ON public.vouchers USING btree (merchant_id, is_active);


--
-- Name: idx_vouchers_merchant_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_vouchers_merchant_id ON public.vouchers USING btree (merchant_id);


--
-- Name: idx_vouchers_valid_period; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_vouchers_valid_period ON public.vouchers USING btree (valid_from, valid_until);


--
-- Name: idx_wechat_notifications_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_wechat_notifications_created_at ON public.wechat_notifications USING btree (created_at);


--
-- Name: idx_wechat_notifications_event_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_wechat_notifications_event_type ON public.wechat_notifications USING btree (event_type);


--
-- Name: idx_wechat_notifications_out_trade_no; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_wechat_notifications_out_trade_no ON public.wechat_notifications USING btree (out_trade_no) WHERE (out_trade_no IS NOT NULL);


--
-- Name: ingredients_category_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ingredients_category_idx ON public.ingredients USING btree (category);


--
-- Name: ingredients_is_system_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ingredients_is_system_idx ON public.ingredients USING btree (is_system);


--
-- Name: ingredients_name_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ingredients_name_idx ON public.ingredients USING btree (name);


--
-- Name: merchant_applications_business_license_number_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX merchant_applications_business_license_number_idx ON public.merchant_applications USING btree (business_license_number);


--
-- Name: merchant_applications_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX merchant_applications_status_idx ON public.merchant_applications USING btree (status);


--
-- Name: merchant_applications_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX merchant_applications_user_id_idx ON public.merchant_applications USING btree (user_id);


--
-- Name: merchant_business_hours_merchant_id_day_of_week_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX merchant_business_hours_merchant_id_day_of_week_idx ON public.merchant_business_hours USING btree (merchant_id, day_of_week);


--
-- Name: merchant_business_hours_merchant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX merchant_business_hours_merchant_id_idx ON public.merchant_business_hours USING btree (merchant_id);


--
-- Name: merchant_business_hours_merchant_id_special_date_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX merchant_business_hours_merchant_id_special_date_idx ON public.merchant_business_hours USING btree (merchant_id, special_date);


--
-- Name: merchant_delivery_promotions_merchant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX merchant_delivery_promotions_merchant_id_idx ON public.merchant_delivery_promotions USING btree (merchant_id);


--
-- Name: merchant_delivery_promotions_merchant_id_is_active_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX merchant_delivery_promotions_merchant_id_is_active_idx ON public.merchant_delivery_promotions USING btree (merchant_id, is_active);


--
-- Name: merchant_delivery_promotions_valid_from_valid_until_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX merchant_delivery_promotions_valid_from_valid_until_idx ON public.merchant_delivery_promotions USING btree (valid_from, valid_until);


--
-- Name: merchant_dish_categories_merchant_id_sort_order_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX merchant_dish_categories_merchant_id_sort_order_idx ON public.merchant_dish_categories USING btree (merchant_id, sort_order);


--
-- Name: merchant_payment_configs_sub_mch_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX merchant_payment_configs_sub_mch_id_idx ON public.merchant_payment_configs USING btree (sub_mch_id);


--
-- Name: merchants_owner_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX merchants_owner_user_id_idx ON public.merchants USING btree (owner_user_id);


--
-- Name: merchants_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX merchants_status_idx ON public.merchants USING btree (status);


--
-- Name: operator_applications_region_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX operator_applications_region_id_idx ON public.operator_applications USING btree (region_id);


--
-- Name: operator_applications_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX operator_applications_status_idx ON public.operator_applications USING btree (status);


--
-- Name: operator_applications_submitted_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX operator_applications_submitted_at_idx ON public.operator_applications USING btree (submitted_at);


--
-- Name: operators_contract_end_date_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX operators_contract_end_date_idx ON public.operators USING btree (contract_end_date);


--
-- Name: operators_sub_mch_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX operators_sub_mch_id_idx ON public.operators USING btree (sub_mch_id);


--
-- Name: operators_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX operators_user_id_idx ON public.operators USING btree (user_id);


--
-- Name: operators_wechat_mch_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX operators_wechat_mch_id_idx ON public.operators USING btree (wechat_mch_id);


--
-- Name: order_display_configs_merchant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX order_display_configs_merchant_id_idx ON public.order_display_configs USING btree (merchant_id);


--
-- Name: order_items_combo_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX order_items_combo_id_idx ON public.order_items USING btree (combo_id);


--
-- Name: order_items_dish_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX order_items_dish_id_idx ON public.order_items USING btree (dish_id);


--
-- Name: order_items_order_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX order_items_order_id_idx ON public.order_items USING btree (order_id);


--
-- Name: order_status_logs_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX order_status_logs_created_at_idx ON public.order_status_logs USING btree (created_at);


--
-- Name: order_status_logs_order_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX order_status_logs_order_id_idx ON public.order_status_logs USING btree (order_id);


--
-- Name: orders_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX orders_created_at_idx ON public.orders USING btree (created_at);


--
-- Name: orders_membership_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX orders_membership_id_idx ON public.orders USING btree (membership_id) WHERE (membership_id IS NOT NULL);


--
-- Name: orders_merchant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX orders_merchant_id_idx ON public.orders USING btree (merchant_id);


--
-- Name: orders_merchant_status_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX orders_merchant_status_created_idx ON public.orders USING btree (merchant_id, status, created_at);


--
-- Name: orders_order_no_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX orders_order_no_idx ON public.orders USING btree (order_no);


--
-- Name: orders_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX orders_status_idx ON public.orders USING btree (status);


--
-- Name: orders_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX orders_user_id_idx ON public.orders USING btree (user_id);


--
-- Name: orders_user_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX orders_user_status_idx ON public.orders USING btree (user_id, status);


--
-- Name: orders_user_voucher_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX orders_user_voucher_id_idx ON public.orders USING btree (user_voucher_id) WHERE (user_voucher_id IS NOT NULL);


--
-- Name: payment_orders_attach_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX payment_orders_attach_idx ON public.payment_orders USING gin (to_tsvector('simple'::regconfig, attach));


--
-- Name: payment_orders_combined_payment_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX payment_orders_combined_payment_id_idx ON public.payment_orders USING btree (combined_payment_id);


--
-- Name: payment_orders_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX payment_orders_created_at_idx ON public.payment_orders USING btree (created_at);


--
-- Name: payment_orders_order_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX payment_orders_order_id_idx ON public.payment_orders USING btree (order_id);


--
-- Name: payment_orders_out_trade_no_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX payment_orders_out_trade_no_idx ON public.payment_orders USING btree (out_trade_no);


--
-- Name: payment_orders_reservation_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX payment_orders_reservation_id_idx ON public.payment_orders USING btree (reservation_id);


--
-- Name: payment_orders_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX payment_orders_status_idx ON public.payment_orders USING btree (status);


--
-- Name: payment_orders_transaction_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX payment_orders_transaction_id_idx ON public.payment_orders USING btree (transaction_id);


--
-- Name: payment_orders_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX payment_orders_user_id_idx ON public.payment_orders USING btree (user_id);


--
-- Name: peak_hour_configs_is_active_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX peak_hour_configs_is_active_idx ON public.peak_hour_configs USING btree (is_active);


--
-- Name: peak_hour_configs_region_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX peak_hour_configs_region_id_idx ON public.peak_hour_configs USING btree (region_id);


--
-- Name: peak_hour_configs_region_id_is_active_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX peak_hour_configs_region_id_is_active_idx ON public.peak_hour_configs USING btree (region_id, is_active);


--
-- Name: print_logs_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX print_logs_created_at_idx ON public.print_logs USING btree (created_at);


--
-- Name: print_logs_order_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX print_logs_order_id_idx ON public.print_logs USING btree (order_id);


--
-- Name: print_logs_printer_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX print_logs_printer_id_idx ON public.print_logs USING btree (printer_id);


--
-- Name: print_logs_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX print_logs_status_idx ON public.print_logs USING btree (status);


--
-- Name: profit_sharing_orders_merchant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX profit_sharing_orders_merchant_id_idx ON public.profit_sharing_orders USING btree (merchant_id);


--
-- Name: profit_sharing_orders_operator_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX profit_sharing_orders_operator_id_idx ON public.profit_sharing_orders USING btree (operator_id);


--
-- Name: profit_sharing_orders_out_order_no_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX profit_sharing_orders_out_order_no_idx ON public.profit_sharing_orders USING btree (out_order_no);


--
-- Name: profit_sharing_orders_payment_order_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX profit_sharing_orders_payment_order_id_idx ON public.profit_sharing_orders USING btree (payment_order_id);


--
-- Name: profit_sharing_orders_rider_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX profit_sharing_orders_rider_id_idx ON public.profit_sharing_orders USING btree (rider_id);


--
-- Name: profit_sharing_orders_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX profit_sharing_orders_status_idx ON public.profit_sharing_orders USING btree (status);


--
-- Name: recommend_configs_is_active_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX recommend_configs_is_active_idx ON public.recommend_configs USING btree (is_active);


--
-- Name: recommend_configs_name_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX recommend_configs_name_idx ON public.recommend_configs USING btree (name);


--
-- Name: recommendation_configs_region_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX recommendation_configs_region_id_idx ON public.recommendation_configs USING btree (region_id);


--
-- Name: recommendations_generated_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX recommendations_generated_at_idx ON public.recommendations USING btree (generated_at);


--
-- Name: recommendations_user_id_generated_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX recommendations_user_id_generated_at_idx ON public.recommendations USING btree (user_id, generated_at);


--
-- Name: recommendations_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX recommendations_user_id_idx ON public.recommendations USING btree (user_id);


--
-- Name: refund_orders_out_refund_no_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX refund_orders_out_refund_no_idx ON public.refund_orders USING btree (out_refund_no);


--
-- Name: refund_orders_payment_order_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX refund_orders_payment_order_id_idx ON public.refund_orders USING btree (payment_order_id);


--
-- Name: refund_orders_refund_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX refund_orders_refund_id_idx ON public.refund_orders USING btree (refund_id);


--
-- Name: refund_orders_refund_type_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX refund_orders_refund_type_idx ON public.refund_orders USING btree (refund_type);


--
-- Name: refund_orders_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX refund_orders_status_idx ON public.refund_orders USING btree (status);


--
-- Name: regions_code_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX regions_code_idx ON public.regions USING btree (code);


--
-- Name: regions_longitude_latitude_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX regions_longitude_latitude_idx ON public.regions USING btree (longitude, latitude);


--
-- Name: regions_parent_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX regions_parent_id_idx ON public.regions USING btree (parent_id);


--
-- Name: reservation_items_combo_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX reservation_items_combo_id_idx ON public.reservation_items USING btree (combo_id) WHERE (combo_id IS NOT NULL);


--
-- Name: reservation_items_dish_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX reservation_items_dish_id_idx ON public.reservation_items USING btree (dish_id) WHERE (dish_id IS NOT NULL);


--
-- Name: reservation_items_reservation_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX reservation_items_reservation_id_idx ON public.reservation_items USING btree (reservation_id);


--
-- Name: rider_applications_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX rider_applications_status_idx ON public.rider_applications USING btree (status);


--
-- Name: rider_applications_submitted_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX rider_applications_submitted_at_idx ON public.rider_applications USING btree (submitted_at);


--
-- Name: rider_deposits_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX rider_deposits_created_at_idx ON public.rider_deposits USING btree (created_at);


--
-- Name: rider_deposits_rider_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX rider_deposits_rider_id_idx ON public.rider_deposits USING btree (rider_id);


--
-- Name: rider_deposits_type_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX rider_deposits_type_idx ON public.rider_deposits USING btree (type);


--
-- Name: rider_locations_delivery_recorded_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX rider_locations_delivery_recorded_idx ON public.rider_locations USING btree (delivery_id, recorded_at);


--
-- Name: rider_locations_rider_recorded_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX rider_locations_rider_recorded_idx ON public.rider_locations USING btree (rider_id, recorded_at);


--
-- Name: riders_application_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX riders_application_id_idx ON public.riders USING btree (application_id) WHERE (application_id IS NOT NULL);


--
-- Name: riders_id_card_no_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX riders_id_card_no_idx ON public.riders USING btree (id_card_no);


--
-- Name: riders_is_online_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX riders_is_online_idx ON public.riders USING btree (is_online);


--
-- Name: riders_location_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX riders_location_idx ON public.riders USING btree (current_longitude, current_latitude);


--
-- Name: riders_phone_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX riders_phone_idx ON public.riders USING btree (phone);


--
-- Name: riders_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX riders_status_idx ON public.riders USING btree (status);


--
-- Name: riders_sub_mch_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX riders_sub_mch_id_idx ON public.riders USING btree (sub_mch_id);


--
-- Name: riders_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX riders_user_id_idx ON public.riders USING btree (user_id);


--
-- Name: sessions_access_token_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX sessions_access_token_idx ON public.sessions USING btree (access_token);


--
-- Name: sessions_refresh_token_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX sessions_refresh_token_idx ON public.sessions USING btree (refresh_token);


--
-- Name: sessions_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sessions_user_id_idx ON public.sessions USING btree (user_id);


--
-- Name: sessions_user_id_is_revoked_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sessions_user_id_is_revoked_idx ON public.sessions USING btree (user_id, is_revoked);


--
-- Name: table_reservations_merchant_date_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX table_reservations_merchant_date_idx ON public.table_reservations USING btree (merchant_id, reservation_date);


--
-- Name: table_reservations_merchant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX table_reservations_merchant_id_idx ON public.table_reservations USING btree (merchant_id);


--
-- Name: table_reservations_payment_deadline_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX table_reservations_payment_deadline_idx ON public.table_reservations USING btree (payment_deadline);


--
-- Name: table_reservations_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX table_reservations_status_idx ON public.table_reservations USING btree (status);


--
-- Name: table_reservations_table_date_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX table_reservations_table_date_status_idx ON public.table_reservations USING btree (table_id, reservation_date, status);


--
-- Name: table_reservations_table_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX table_reservations_table_id_idx ON public.table_reservations USING btree (table_id);


--
-- Name: table_reservations_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX table_reservations_user_id_idx ON public.table_reservations USING btree (user_id);


--
-- Name: table_tags_table_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX table_tags_table_id_idx ON public.table_tags USING btree (table_id);


--
-- Name: table_tags_table_tag_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX table_tags_table_tag_idx ON public.table_tags USING btree (table_id, tag_id);


--
-- Name: tables_merchant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tables_merchant_id_idx ON public.tables USING btree (merchant_id);


--
-- Name: tables_merchant_table_no_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX tables_merchant_table_no_idx ON public.tables USING btree (merchant_id, table_no);


--
-- Name: tables_merchant_type_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tables_merchant_type_status_idx ON public.tables USING btree (merchant_id, table_type, status);


--
-- Name: tags_name_type_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX tags_name_type_idx ON public.tags USING btree (name, type);


--
-- Name: tags_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tags_status_idx ON public.tags USING btree (status);


--
-- Name: tags_type_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tags_type_idx ON public.tags USING btree (type);


--
-- Name: tags_type_idx1; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tags_type_idx1 ON public.tags USING btree (type);


--
-- Name: tags_type_name_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tags_type_name_idx ON public.tags USING btree (type, name);


--
-- Name: user_addresses_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_addresses_user_id_idx ON public.user_addresses USING btree (user_id);


--
-- Name: user_addresses_user_id_is_default_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_addresses_user_id_is_default_idx ON public.user_addresses USING btree (user_id, is_default);


--
-- Name: user_behaviors_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_behaviors_created_at_idx ON public.user_behaviors USING btree (created_at);


--
-- Name: user_behaviors_user_id_behavior_type_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_behaviors_user_id_behavior_type_idx ON public.user_behaviors USING btree (user_id, behavior_type);


--
-- Name: user_behaviors_user_id_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_behaviors_user_id_created_at_idx ON public.user_behaviors USING btree (user_id, created_at);


--
-- Name: user_behaviors_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_behaviors_user_id_idx ON public.user_behaviors USING btree (user_id);


--
-- Name: user_devices_device_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_devices_device_id_idx ON public.user_devices USING btree (device_id);


--
-- Name: user_devices_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_devices_user_id_idx ON public.user_devices USING btree (user_id);


--
-- Name: user_preferences_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX user_preferences_user_id_idx ON public.user_preferences USING btree (user_id);


--
-- Name: user_roles_role_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_roles_role_idx ON public.user_roles USING btree (role);


--
-- Name: user_roles_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_roles_user_id_idx ON public.user_roles USING btree (user_id);


--
-- Name: user_roles_user_id_role_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX user_roles_user_id_role_idx ON public.user_roles USING btree (user_id, role);


--
-- Name: users_phone_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX users_phone_idx ON public.users USING btree (phone);


--
-- Name: users_wechat_openid_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX users_wechat_openid_idx ON public.users USING btree (wechat_openid);


--
-- Name: users_wechat_unionid_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX users_wechat_unionid_idx ON public.users USING btree (wechat_unionid) WHERE (wechat_unionid IS NOT NULL);


--
-- Name: weather_coefficients_has_warning_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX weather_coefficients_has_warning_idx ON public.weather_coefficients USING btree (has_warning);


--
-- Name: weather_coefficients_recorded_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX weather_coefficients_recorded_at_idx ON public.weather_coefficients USING btree (recorded_at);


--
-- Name: weather_coefficients_region_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX weather_coefficients_region_id_idx ON public.weather_coefficients USING btree (region_id);


--
-- Name: weather_coefficients_region_id_recorded_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX weather_coefficients_region_id_recorded_at_idx ON public.weather_coefficients USING btree (region_id, recorded_at);


--
-- Name: wechat_access_tokens_app_type_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX wechat_access_tokens_app_type_idx ON public.wechat_access_tokens USING btree (app_type);


--
-- Name: discount_rules update_discount_rules_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_discount_rules_updated_at BEFORE UPDATE ON public.discount_rules FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: merchant_membership_settings update_merchant_membership_settings_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_merchant_membership_settings_updated_at BEFORE UPDATE ON public.merchant_membership_settings FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: merchant_memberships update_merchant_memberships_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_merchant_memberships_updated_at BEFORE UPDATE ON public.merchant_memberships FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: recharge_rules update_recharge_rules_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_recharge_rules_updated_at BEFORE UPDATE ON public.recharge_rules FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: user_devices update_user_devices_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_user_devices_updated_at BEFORE UPDATE ON public.user_devices FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: vouchers update_vouchers_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_vouchers_updated_at BEFORE UPDATE ON public.vouchers FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: appeals appeals_claim_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.appeals
    ADD CONSTRAINT appeals_claim_id_fkey FOREIGN KEY (claim_id) REFERENCES public.claims(id) ON DELETE CASCADE;


--
-- Name: appeals appeals_region_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.appeals
    ADD CONSTRAINT appeals_region_id_fkey FOREIGN KEY (region_id) REFERENCES public.regions(id);


--
-- Name: browse_history browse_history_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.browse_history
    ADD CONSTRAINT browse_history_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: cart_items cart_items_cart_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cart_items
    ADD CONSTRAINT cart_items_cart_id_fkey FOREIGN KEY (cart_id) REFERENCES public.carts(id) ON DELETE CASCADE;


--
-- Name: cart_items cart_items_combo_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cart_items
    ADD CONSTRAINT cart_items_combo_id_fkey FOREIGN KEY (combo_id) REFERENCES public.combo_sets(id) ON DELETE CASCADE;


--
-- Name: cart_items cart_items_dish_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cart_items
    ADD CONSTRAINT cart_items_dish_id_fkey FOREIGN KEY (dish_id) REFERENCES public.dishes(id) ON DELETE CASCADE;


--
-- Name: carts carts_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.carts
    ADD CONSTRAINT carts_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id);


--
-- Name: carts carts_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.carts
    ADD CONSTRAINT carts_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: claims claims_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.claims
    ADD CONSTRAINT claims_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.orders(id) ON DELETE CASCADE;


--
-- Name: claims claims_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.claims
    ADD CONSTRAINT claims_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: cloud_printers cloud_printers_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cloud_printers
    ADD CONSTRAINT cloud_printers_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id);


--
-- Name: combined_payment_orders combined_payment_orders_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.combined_payment_orders
    ADD CONSTRAINT combined_payment_orders_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: combined_payment_sub_orders combined_payment_sub_orders_combined_payment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.combined_payment_sub_orders
    ADD CONSTRAINT combined_payment_sub_orders_combined_payment_id_fkey FOREIGN KEY (combined_payment_id) REFERENCES public.combined_payment_orders(id) ON DELETE CASCADE;


--
-- Name: combined_payment_sub_orders combined_payment_sub_orders_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.combined_payment_sub_orders
    ADD CONSTRAINT combined_payment_sub_orders_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id);


--
-- Name: combined_payment_sub_orders combined_payment_sub_orders_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.combined_payment_sub_orders
    ADD CONSTRAINT combined_payment_sub_orders_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.orders(id);


--
-- Name: combo_dishes combo_dishes_combo_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.combo_dishes
    ADD CONSTRAINT combo_dishes_combo_id_fkey FOREIGN KEY (combo_id) REFERENCES public.combo_sets(id) ON DELETE CASCADE;


--
-- Name: combo_dishes combo_dishes_dish_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.combo_dishes
    ADD CONSTRAINT combo_dishes_dish_id_fkey FOREIGN KEY (dish_id) REFERENCES public.dishes(id) ON DELETE CASCADE;


--
-- Name: combo_sets combo_sets_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.combo_sets
    ADD CONSTRAINT combo_sets_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id) ON DELETE CASCADE;


--
-- Name: combo_tags combo_tags_combo_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.combo_tags
    ADD CONSTRAINT combo_tags_combo_id_fkey FOREIGN KEY (combo_id) REFERENCES public.combo_sets(id) ON DELETE CASCADE;


--
-- Name: combo_tags combo_tags_tag_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.combo_tags
    ADD CONSTRAINT combo_tags_tag_id_fkey FOREIGN KEY (tag_id) REFERENCES public.tags(id) ON DELETE CASCADE;


--
-- Name: daily_inventory daily_inventory_dish_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.daily_inventory
    ADD CONSTRAINT daily_inventory_dish_id_fkey FOREIGN KEY (dish_id) REFERENCES public.dishes(id) ON DELETE CASCADE;


--
-- Name: daily_inventory daily_inventory_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.daily_inventory
    ADD CONSTRAINT daily_inventory_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id) ON DELETE CASCADE;


--
-- Name: deliveries deliveries_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.deliveries
    ADD CONSTRAINT deliveries_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.orders(id);


--
-- Name: deliveries deliveries_rider_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.deliveries
    ADD CONSTRAINT deliveries_rider_id_fkey FOREIGN KEY (rider_id) REFERENCES public.riders(id);


--
-- Name: delivery_fee_configs delivery_fee_configs_region_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.delivery_fee_configs
    ADD CONSTRAINT delivery_fee_configs_region_id_fkey FOREIGN KEY (region_id) REFERENCES public.regions(id) ON DELETE CASCADE;


--
-- Name: delivery_pool delivery_pool_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.delivery_pool
    ADD CONSTRAINT delivery_pool_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id);


--
-- Name: delivery_pool delivery_pool_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.delivery_pool
    ADD CONSTRAINT delivery_pool_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.orders(id);


--
-- Name: discount_rules discount_rules_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discount_rules
    ADD CONSTRAINT discount_rules_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id) ON DELETE CASCADE;


--
-- Name: dish_customization_groups dish_customization_groups_dish_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dish_customization_groups
    ADD CONSTRAINT dish_customization_groups_dish_id_fkey FOREIGN KEY (dish_id) REFERENCES public.dishes(id) ON DELETE CASCADE;


--
-- Name: dish_customization_options dish_customization_options_group_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dish_customization_options
    ADD CONSTRAINT dish_customization_options_group_id_fkey FOREIGN KEY (group_id) REFERENCES public.dish_customization_groups(id) ON DELETE CASCADE;


--
-- Name: dish_customization_options dish_customization_options_tag_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dish_customization_options
    ADD CONSTRAINT dish_customization_options_tag_id_fkey FOREIGN KEY (tag_id) REFERENCES public.tags(id) ON DELETE CASCADE;


--
-- Name: dish_ingredients dish_ingredients_dish_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dish_ingredients
    ADD CONSTRAINT dish_ingredients_dish_id_fkey FOREIGN KEY (dish_id) REFERENCES public.dishes(id) ON DELETE CASCADE;


--
-- Name: dish_ingredients dish_ingredients_ingredient_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dish_ingredients
    ADD CONSTRAINT dish_ingredients_ingredient_id_fkey FOREIGN KEY (ingredient_id) REFERENCES public.ingredients(id) ON DELETE CASCADE;


--
-- Name: dish_tags dish_tags_dish_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dish_tags
    ADD CONSTRAINT dish_tags_dish_id_fkey FOREIGN KEY (dish_id) REFERENCES public.dishes(id) ON DELETE CASCADE;


--
-- Name: dish_tags dish_tags_tag_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dish_tags
    ADD CONSTRAINT dish_tags_tag_id_fkey FOREIGN KEY (tag_id) REFERENCES public.tags(id) ON DELETE CASCADE;


--
-- Name: dishes dishes_category_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dishes
    ADD CONSTRAINT dishes_category_id_fkey FOREIGN KEY (category_id) REFERENCES public.dish_categories(id) ON DELETE SET NULL;


--
-- Name: dishes dishes_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dishes
    ADD CONSTRAINT dishes_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id) ON DELETE CASCADE;


--
-- Name: favorites favorites_dish_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.favorites
    ADD CONSTRAINT favorites_dish_id_fkey FOREIGN KEY (dish_id) REFERENCES public.dishes(id) ON DELETE CASCADE;


--
-- Name: favorites favorites_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.favorites
    ADD CONSTRAINT favorites_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id) ON DELETE CASCADE;


--
-- Name: favorites favorites_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.favorites
    ADD CONSTRAINT favorites_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: merchant_applications fk_merchant_applications_region; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_applications
    ADD CONSTRAINT fk_merchant_applications_region FOREIGN KEY (region_id) REFERENCES public.regions(id);


--
-- Name: food_safety_incidents food_safety_incidents_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.food_safety_incidents
    ADD CONSTRAINT food_safety_incidents_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id) ON DELETE CASCADE;


--
-- Name: food_safety_incidents food_safety_incidents_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.food_safety_incidents
    ADD CONSTRAINT food_safety_incidents_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.orders(id) ON DELETE CASCADE;


--
-- Name: food_safety_incidents food_safety_incidents_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.food_safety_incidents
    ADD CONSTRAINT food_safety_incidents_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: ingredients ingredients_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ingredients
    ADD CONSTRAINT ingredients_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id);


--
-- Name: membership_transactions membership_transactions_membership_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.membership_transactions
    ADD CONSTRAINT membership_transactions_membership_id_fkey FOREIGN KEY (membership_id) REFERENCES public.merchant_memberships(id) ON DELETE CASCADE;


--
-- Name: membership_transactions membership_transactions_recharge_rule_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.membership_transactions
    ADD CONSTRAINT membership_transactions_recharge_rule_id_fkey FOREIGN KEY (recharge_rule_id) REFERENCES public.recharge_rules(id) ON DELETE SET NULL;


--
-- Name: merchant_applications merchant_applications_reviewed_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_applications
    ADD CONSTRAINT merchant_applications_reviewed_by_fkey FOREIGN KEY (reviewed_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: merchant_applications merchant_applications_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_applications
    ADD CONSTRAINT merchant_applications_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: merchant_bosses merchant_bosses_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_bosses
    ADD CONSTRAINT merchant_bosses_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id) ON DELETE CASCADE;


--
-- Name: merchant_bosses merchant_bosses_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_bosses
    ADD CONSTRAINT merchant_bosses_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: merchant_business_hours merchant_business_hours_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_business_hours
    ADD CONSTRAINT merchant_business_hours_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id) ON DELETE CASCADE;


--
-- Name: merchant_delivery_promotions merchant_delivery_promotions_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_delivery_promotions
    ADD CONSTRAINT merchant_delivery_promotions_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id) ON DELETE CASCADE;


--
-- Name: merchant_dish_categories merchant_dish_categories_category_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_dish_categories
    ADD CONSTRAINT merchant_dish_categories_category_id_fkey FOREIGN KEY (category_id) REFERENCES public.dish_categories(id) ON DELETE CASCADE;


--
-- Name: merchant_dish_categories merchant_dish_categories_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_dish_categories
    ADD CONSTRAINT merchant_dish_categories_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id) ON DELETE CASCADE;


--
-- Name: merchant_membership_settings merchant_membership_settings_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_membership_settings
    ADD CONSTRAINT merchant_membership_settings_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id) ON DELETE CASCADE;


--
-- Name: merchant_memberships merchant_memberships_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_memberships
    ADD CONSTRAINT merchant_memberships_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id) ON DELETE CASCADE;


--
-- Name: merchant_memberships merchant_memberships_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_memberships
    ADD CONSTRAINT merchant_memberships_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: merchant_payment_configs merchant_payment_configs_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_payment_configs
    ADD CONSTRAINT merchant_payment_configs_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id);


--
-- Name: merchant_profiles merchant_profiles_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_profiles
    ADD CONSTRAINT merchant_profiles_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id) ON DELETE CASCADE;


--
-- Name: merchant_staff merchant_staff_invited_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_staff
    ADD CONSTRAINT merchant_staff_invited_by_fkey FOREIGN KEY (invited_by) REFERENCES public.users(id);


--
-- Name: merchant_staff merchant_staff_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_staff
    ADD CONSTRAINT merchant_staff_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id) ON DELETE CASCADE;


--
-- Name: merchant_staff merchant_staff_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_staff
    ADD CONSTRAINT merchant_staff_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: merchant_tags merchant_tags_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_tags
    ADD CONSTRAINT merchant_tags_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id) ON DELETE CASCADE;


--
-- Name: merchant_tags merchant_tags_tag_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchant_tags
    ADD CONSTRAINT merchant_tags_tag_id_fkey FOREIGN KEY (tag_id) REFERENCES public.tags(id) ON DELETE CASCADE;


--
-- Name: merchants merchants_owner_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchants
    ADD CONSTRAINT merchants_owner_user_id_fkey FOREIGN KEY (owner_user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: merchants merchants_region_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merchants
    ADD CONSTRAINT merchants_region_id_fkey FOREIGN KEY (region_id) REFERENCES public.regions(id);


--
-- Name: notifications notifications_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: operator_applications operator_applications_region_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.operator_applications
    ADD CONSTRAINT operator_applications_region_id_fkey FOREIGN KEY (region_id) REFERENCES public.regions(id);


--
-- Name: operator_applications operator_applications_reviewed_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.operator_applications
    ADD CONSTRAINT operator_applications_reviewed_by_fkey FOREIGN KEY (reviewed_by) REFERENCES public.users(id);


--
-- Name: operator_applications operator_applications_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.operator_applications
    ADD CONSTRAINT operator_applications_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: operator_regions operator_regions_operator_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.operator_regions
    ADD CONSTRAINT operator_regions_operator_id_fkey FOREIGN KEY (operator_id) REFERENCES public.operators(id) ON DELETE CASCADE;


--
-- Name: operator_regions operator_regions_region_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.operator_regions
    ADD CONSTRAINT operator_regions_region_id_fkey FOREIGN KEY (region_id) REFERENCES public.regions(id);


--
-- Name: operators operators_region_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.operators
    ADD CONSTRAINT operators_region_id_fkey FOREIGN KEY (region_id) REFERENCES public.regions(id);


--
-- Name: operators operators_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.operators
    ADD CONSTRAINT operators_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: order_display_configs order_display_configs_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_display_configs
    ADD CONSTRAINT order_display_configs_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id);


--
-- Name: order_items order_items_combo_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_items
    ADD CONSTRAINT order_items_combo_id_fkey FOREIGN KEY (combo_id) REFERENCES public.combo_sets(id);


--
-- Name: order_items order_items_dish_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_items
    ADD CONSTRAINT order_items_dish_id_fkey FOREIGN KEY (dish_id) REFERENCES public.dishes(id);


--
-- Name: order_items order_items_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_items
    ADD CONSTRAINT order_items_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.orders(id) ON DELETE CASCADE;


--
-- Name: order_status_logs order_status_logs_operator_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_status_logs
    ADD CONSTRAINT order_status_logs_operator_id_fkey FOREIGN KEY (operator_id) REFERENCES public.users(id);


--
-- Name: order_status_logs order_status_logs_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_status_logs
    ADD CONSTRAINT order_status_logs_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.orders(id) ON DELETE CASCADE;


--
-- Name: orders orders_address_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_address_id_fkey FOREIGN KEY (address_id) REFERENCES public.user_addresses(id);


--
-- Name: orders orders_membership_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_membership_id_fkey FOREIGN KEY (membership_id) REFERENCES public.merchant_memberships(id);


--
-- Name: orders orders_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id);


--
-- Name: orders orders_reservation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_reservation_id_fkey FOREIGN KEY (reservation_id) REFERENCES public.table_reservations(id);


--
-- Name: orders orders_table_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_table_id_fkey FOREIGN KEY (table_id) REFERENCES public.tables(id);


--
-- Name: orders orders_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: orders orders_user_voucher_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_user_voucher_id_fkey FOREIGN KEY (user_voucher_id) REFERENCES public.user_vouchers(id);


--
-- Name: payment_orders payment_orders_combined_payment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_orders
    ADD CONSTRAINT payment_orders_combined_payment_id_fkey FOREIGN KEY (combined_payment_id) REFERENCES public.combined_payment_orders(id);


--
-- Name: payment_orders payment_orders_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_orders
    ADD CONSTRAINT payment_orders_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.orders(id);


--
-- Name: payment_orders payment_orders_reservation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_orders
    ADD CONSTRAINT payment_orders_reservation_id_fkey FOREIGN KEY (reservation_id) REFERENCES public.table_reservations(id);


--
-- Name: payment_orders payment_orders_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_orders
    ADD CONSTRAINT payment_orders_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: peak_hour_configs peak_hour_configs_region_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.peak_hour_configs
    ADD CONSTRAINT peak_hour_configs_region_id_fkey FOREIGN KEY (region_id) REFERENCES public.regions(id) ON DELETE CASCADE;


--
-- Name: print_logs print_logs_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.print_logs
    ADD CONSTRAINT print_logs_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.orders(id) ON DELETE CASCADE;


--
-- Name: print_logs print_logs_printer_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.print_logs
    ADD CONSTRAINT print_logs_printer_id_fkey FOREIGN KEY (printer_id) REFERENCES public.cloud_printers(id);


--
-- Name: profit_sharing_orders profit_sharing_orders_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.profit_sharing_orders
    ADD CONSTRAINT profit_sharing_orders_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id);


--
-- Name: profit_sharing_orders profit_sharing_orders_operator_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.profit_sharing_orders
    ADD CONSTRAINT profit_sharing_orders_operator_id_fkey FOREIGN KEY (operator_id) REFERENCES public.operators(id);


--
-- Name: profit_sharing_orders profit_sharing_orders_payment_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.profit_sharing_orders
    ADD CONSTRAINT profit_sharing_orders_payment_order_id_fkey FOREIGN KEY (payment_order_id) REFERENCES public.payment_orders(id);


--
-- Name: profit_sharing_orders profit_sharing_orders_rider_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.profit_sharing_orders
    ADD CONSTRAINT profit_sharing_orders_rider_id_fkey FOREIGN KEY (rider_id) REFERENCES public.riders(id);


--
-- Name: recharge_rules recharge_rules_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.recharge_rules
    ADD CONSTRAINT recharge_rules_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id) ON DELETE CASCADE;


--
-- Name: recommendation_configs recommendation_configs_region_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.recommendation_configs
    ADD CONSTRAINT recommendation_configs_region_id_fkey FOREIGN KEY (region_id) REFERENCES public.regions(id);


--
-- Name: recommendations recommendations_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.recommendations
    ADD CONSTRAINT recommendations_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: refund_orders refund_orders_payment_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.refund_orders
    ADD CONSTRAINT refund_orders_payment_order_id_fkey FOREIGN KEY (payment_order_id) REFERENCES public.payment_orders(id);


--
-- Name: regions regions_parent_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.regions
    ADD CONSTRAINT regions_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES public.regions(id);


--
-- Name: reservation_items reservation_items_combo_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reservation_items
    ADD CONSTRAINT reservation_items_combo_id_fkey FOREIGN KEY (combo_id) REFERENCES public.combo_sets(id) ON DELETE SET NULL;


--
-- Name: reservation_items reservation_items_dish_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reservation_items
    ADD CONSTRAINT reservation_items_dish_id_fkey FOREIGN KEY (dish_id) REFERENCES public.dishes(id) ON DELETE SET NULL;


--
-- Name: reservation_items reservation_items_reservation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reservation_items
    ADD CONSTRAINT reservation_items_reservation_id_fkey FOREIGN KEY (reservation_id) REFERENCES public.table_reservations(id) ON DELETE CASCADE;


--
-- Name: reviews reviews_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviews
    ADD CONSTRAINT reviews_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id) ON DELETE CASCADE;


--
-- Name: reviews reviews_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviews
    ADD CONSTRAINT reviews_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.orders(id) ON DELETE CASCADE;


--
-- Name: reviews reviews_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviews
    ADD CONSTRAINT reviews_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: rider_applications rider_applications_reviewed_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rider_applications
    ADD CONSTRAINT rider_applications_reviewed_by_fkey FOREIGN KEY (reviewed_by) REFERENCES public.users(id);


--
-- Name: rider_applications rider_applications_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rider_applications
    ADD CONSTRAINT rider_applications_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: rider_deposits rider_deposits_related_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rider_deposits
    ADD CONSTRAINT rider_deposits_related_order_id_fkey FOREIGN KEY (related_order_id) REFERENCES public.orders(id);


--
-- Name: rider_deposits rider_deposits_rider_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rider_deposits
    ADD CONSTRAINT rider_deposits_rider_id_fkey FOREIGN KEY (rider_id) REFERENCES public.riders(id);


--
-- Name: rider_locations rider_locations_delivery_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rider_locations
    ADD CONSTRAINT rider_locations_delivery_id_fkey FOREIGN KEY (delivery_id) REFERENCES public.deliveries(id);


--
-- Name: rider_locations rider_locations_rider_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rider_locations
    ADD CONSTRAINT rider_locations_rider_id_fkey FOREIGN KEY (rider_id) REFERENCES public.riders(id);


--
-- Name: rider_premium_score_logs rider_premium_score_logs_related_delivery_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rider_premium_score_logs
    ADD CONSTRAINT rider_premium_score_logs_related_delivery_id_fkey FOREIGN KEY (related_delivery_id) REFERENCES public.deliveries(id);


--
-- Name: rider_premium_score_logs rider_premium_score_logs_related_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rider_premium_score_logs
    ADD CONSTRAINT rider_premium_score_logs_related_order_id_fkey FOREIGN KEY (related_order_id) REFERENCES public.orders(id);


--
-- Name: rider_premium_score_logs rider_premium_score_logs_rider_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rider_premium_score_logs
    ADD CONSTRAINT rider_premium_score_logs_rider_id_fkey FOREIGN KEY (rider_id) REFERENCES public.riders(id) ON DELETE CASCADE;


--
-- Name: rider_profiles rider_profiles_rider_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rider_profiles
    ADD CONSTRAINT rider_profiles_rider_id_fkey FOREIGN KEY (rider_id) REFERENCES public.riders(id) ON DELETE CASCADE;


--
-- Name: riders riders_application_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.riders
    ADD CONSTRAINT riders_application_id_fkey FOREIGN KEY (application_id) REFERENCES public.rider_applications(id);


--
-- Name: riders riders_region_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.riders
    ADD CONSTRAINT riders_region_id_fkey FOREIGN KEY (region_id) REFERENCES public.regions(id);


--
-- Name: riders riders_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.riders
    ADD CONSTRAINT riders_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: sessions sessions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: table_images table_images_table_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.table_images
    ADD CONSTRAINT table_images_table_id_fkey FOREIGN KEY (table_id) REFERENCES public.tables(id) ON DELETE CASCADE;


--
-- Name: table_reservations table_reservations_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.table_reservations
    ADD CONSTRAINT table_reservations_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id);


--
-- Name: table_reservations table_reservations_table_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.table_reservations
    ADD CONSTRAINT table_reservations_table_id_fkey FOREIGN KEY (table_id) REFERENCES public.tables(id);


--
-- Name: table_reservations table_reservations_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.table_reservations
    ADD CONSTRAINT table_reservations_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: table_tags table_tags_table_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.table_tags
    ADD CONSTRAINT table_tags_table_id_fkey FOREIGN KEY (table_id) REFERENCES public.tables(id) ON DELETE CASCADE;


--
-- Name: table_tags table_tags_tag_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.table_tags
    ADD CONSTRAINT table_tags_tag_id_fkey FOREIGN KEY (tag_id) REFERENCES public.tags(id) ON DELETE CASCADE;


--
-- Name: tables tables_current_reservation_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tables
    ADD CONSTRAINT tables_current_reservation_fk FOREIGN KEY (current_reservation_id) REFERENCES public.table_reservations(id) ON DELETE SET NULL;


--
-- Name: tables tables_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tables
    ADD CONSTRAINT tables_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id);


--
-- Name: user_addresses user_addresses_region_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_addresses
    ADD CONSTRAINT user_addresses_region_id_fkey FOREIGN KEY (region_id) REFERENCES public.regions(id);


--
-- Name: user_addresses user_addresses_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_addresses
    ADD CONSTRAINT user_addresses_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_balance_logs user_balance_logs_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_balance_logs
    ADD CONSTRAINT user_balance_logs_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: user_balances user_balances_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_balances
    ADD CONSTRAINT user_balances_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: user_behaviors user_behaviors_combo_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_behaviors
    ADD CONSTRAINT user_behaviors_combo_id_fkey FOREIGN KEY (combo_id) REFERENCES public.combo_sets(id);


--
-- Name: user_behaviors user_behaviors_dish_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_behaviors
    ADD CONSTRAINT user_behaviors_dish_id_fkey FOREIGN KEY (dish_id) REFERENCES public.dishes(id);


--
-- Name: user_behaviors user_behaviors_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_behaviors
    ADD CONSTRAINT user_behaviors_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id);


--
-- Name: user_behaviors user_behaviors_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_behaviors
    ADD CONSTRAINT user_behaviors_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: user_claim_warnings user_claim_warnings_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_claim_warnings
    ADD CONSTRAINT user_claim_warnings_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: user_devices user_devices_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_devices
    ADD CONSTRAINT user_devices_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_notification_preferences user_notification_preferences_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_notification_preferences
    ADD CONSTRAINT user_notification_preferences_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_preferences user_preferences_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_preferences
    ADD CONSTRAINT user_preferences_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: user_profiles user_profiles_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_profiles
    ADD CONSTRAINT user_profiles_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_roles user_roles_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_roles
    ADD CONSTRAINT user_roles_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_vouchers user_vouchers_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_vouchers
    ADD CONSTRAINT user_vouchers_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_vouchers user_vouchers_voucher_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_vouchers
    ADD CONSTRAINT user_vouchers_voucher_id_fkey FOREIGN KEY (voucher_id) REFERENCES public.vouchers(id) ON DELETE CASCADE;


--
-- Name: vouchers vouchers_merchant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vouchers
    ADD CONSTRAINT vouchers_merchant_id_fkey FOREIGN KEY (merchant_id) REFERENCES public.merchants(id) ON DELETE CASCADE;


--
-- Name: weather_coefficients weather_coefficients_region_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.weather_coefficients
    ADD CONSTRAINT weather_coefficients_region_id_fkey FOREIGN KEY (region_id) REFERENCES public.regions(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

\unrestrict 6nWZASry3GkjxglF8eh70qww1Fj4X0V0Od1kp2Ngj5fEFyO6iSB6EnDvWdlDlfN

