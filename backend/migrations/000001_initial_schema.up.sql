-- Squashed initial schema for the IMS backend.
--
-- Idempotent: every CREATE uses IF NOT EXISTS; every ALTER TABLE ADD
-- CONSTRAINT is wrapped in a DO block that swallows duplicate_object
-- and duplicate_table errors. Triggers + functions use OR REPLACE /
-- DROP IF EXISTS. The migration is safe to run repeatedly and against
-- a database that already has the schema.
--
-- Regenerating after a schema change: dump via pg_dump --schema-only and
-- post-process so every CREATE has IF NOT EXISTS, every ADD CONSTRAINT is
-- wrapped in a DO block that swallows duplicate_object, triggers use
-- DROP IF EXISTS + CREATE, and the schema_migrations table is stripped
-- (golang-migrate owns it).

--
-- PostgreSQL database dump
--


-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
-- pg_dump's `SELECT pg_catalog.set_config('search_path','',false);` was
-- stripped here because golang-migrate runs migrations on a shared
-- session: the setting leaks into 000002+ and breaks unqualified table
-- references. Tables in this dump are already fully qualified with public.
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: pg_trgm; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;


--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: update_product_search_vector(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE OR REPLACE FUNCTION public.update_product_search_vector() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.search_vector := to_tsvector('english',
        coalesce(NEW.name, '') || ' ' ||
        coalesce(NEW.description, '') || ' ' ||
        coalesce(NEW.sku, '')
    );
    RETURN NEW;
END;
$$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: audit_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.audit_logs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid,
    action character varying(50) NOT NULL,
    entity character varying(50) NOT NULL,
    entity_id uuid NOT NULL,
    ip_address character varying(45) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    org_id uuid NOT NULL
);


--
-- Name: cart_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.cart_items (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    cart_id uuid NOT NULL,
    listing_id uuid NOT NULL,
    quantity integer NOT NULL,
    added_at timestamp with time zone DEFAULT now(),
    CONSTRAINT cart_items_quantity_check CHECK ((quantity > 0))
);


--
-- Name: carts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.carts (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    buyer_org_id uuid,
    customer_id uuid,
    expires_at timestamp with time zone DEFAULT (now() + '24:00:00'::interval),
    created_at timestamp with time zone DEFAULT now(),
    CONSTRAINT cart_owner_check CHECK ((((buyer_org_id IS NOT NULL) AND (customer_id IS NULL)) OR ((buyer_org_id IS NULL) AND (customer_id IS NOT NULL))))
);


--
-- Name: categories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.categories (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(100) NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    org_id uuid NOT NULL
);


--
-- Name: customer_addresses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.customer_addresses (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    customer_id uuid NOT NULL,
    label character varying(50) DEFAULT 'Home'::character varying,
    line1 text NOT NULL,
    line2 text,
    city character varying(100),
    state character varying(100),
    country character varying(100),
    postal_code character varying(20),
    lat numeric(10,8),
    lng numeric(11,8),
    is_default boolean DEFAULT false,
    created_at timestamp with time zone DEFAULT now()
);


--
-- Name: customers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.customers (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(255) NOT NULL,
    email character varying(255),
    phone character varying(20),
    password_hash text,
    is_verified boolean DEFAULT false,
    is_guest boolean DEFAULT false,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);


--
-- Name: email_verifications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.email_verifications (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    email character varying(255) NOT NULL,
    otp_hash text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    consumed_at timestamp with time zone,
    attempts integer DEFAULT 0,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: inventory; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.inventory (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    product_id uuid NOT NULL,
    quantity bigint NOT NULL,
    low_stock_threshold bigint NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    org_id uuid NOT NULL
);


--
-- Name: inventory_reservations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.inventory_reservations (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    inventory_id uuid NOT NULL,
    order_id uuid,
    org_id uuid NOT NULL,
    quantity integer NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    reserved_at timestamp with time zone DEFAULT now(),
    expires_at timestamp with time zone DEFAULT (now() + '00:30:00'::interval),
    released_at timestamp with time zone,
    CONSTRAINT inventory_reservations_quantity_check CHECK ((quantity > 0)),
    CONSTRAINT inventory_reservations_status_check CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'committed'::character varying, 'released'::character varying, 'expired'::character varying])::text[])))
);


--
-- Name: login_attempts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.login_attempts (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    email character varying(255) NOT NULL,
    ip_address character varying(45),
    user_agent text,
    success boolean NOT NULL,
    failure_reason character varying(100),
    attempted_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: marketplace_listings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.marketplace_listings (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    org_id uuid NOT NULL,
    product_id uuid NOT NULL,
    location_id uuid,
    listing_price bigint NOT NULL,
    min_order_qty integer DEFAULT 1 NOT NULL,
    max_order_qty integer,
    is_active boolean DEFAULT true,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    CONSTRAINT marketplace_listings_check CHECK (((max_order_qty IS NULL) OR (max_order_qty >= min_order_qty))),
    CONSTRAINT marketplace_listings_listing_price_check CHECK ((listing_price >= 0)),
    CONSTRAINT marketplace_listings_min_order_qty_check CHECK ((min_order_qty >= 1))
);


--
-- Name: notifications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.notifications (
    id uuid NOT NULL,
    channel text DEFAULT 'email'::text NOT NULL,
    recipient text NOT NULL,
    subject text NOT NULL,
    body_text text NOT NULL,
    body_html text,
    status text DEFAULT 'pending'::text NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    last_error text,
    next_attempt timestamp with time zone DEFAULT now() NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    sent_at timestamp with time zone,
    CONSTRAINT notifications_channel_check CHECK ((channel = ANY (ARRAY['email'::text, 'sms'::text, 'push'::text]))),
    CONSTRAINT notifications_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'sent'::text, 'failed'::text, 'dlq'::text])))
);


--
-- Name: order_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.order_items (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    order_id uuid NOT NULL,
    product_id uuid NOT NULL,
    quantity bigint NOT NULL,
    unit_price bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    org_id uuid NOT NULL
);


--
-- Name: orders; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.orders (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    status character varying(50) NOT NULL,
    total_amount bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    org_id uuid NOT NULL,
    order_type character varying(20) DEFAULT 'internal'::character varying NOT NULL,
    buyer_org_id uuid,
    supplier_org_id uuid,
    supplier_location_id uuid,
    customer_id uuid,
    delivery_address_id uuid,
    delivery_address_snapshot jsonb,
    subtotal bigint DEFAULT 0,
    delivery_fee bigint DEFAULT 0,
    discount bigint DEFAULT 0,
    payment_status character varying(20) DEFAULT 'unpaid'::character varying,
    payment_id character varying(255),
    accepted_at timestamp with time zone,
    shipped_at timestamp with time zone,
    delivered_at timestamp with time zone,
    completed_at timestamp with time zone,
    cancelled_at timestamp with time zone,
    CONSTRAINT orders_order_type_check CHECK (((order_type)::text = ANY ((ARRAY['internal'::character varying, 'b2b'::character varying, 'b2c'::character varying])::text[]))),
    CONSTRAINT orders_payment_status_check CHECK (((payment_status)::text = ANY ((ARRAY['unpaid'::character varying, 'paid'::character varying, 'refunded'::character varying, 'partial'::character varying])::text[]))),
    CONSTRAINT orders_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'confirmed'::character varying, 'accepted'::character varying, 'rejected'::character varying, 'processing'::character varying, 'ready'::character varying, 'shipped'::character varying, 'delivered'::character varying, 'completed'::character varying, 'cancelled'::character varying, 'refunded'::character varying])::text[])))
);


--
-- Name: org_locations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.org_locations (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    org_id uuid NOT NULL,
    name character varying(255) NOT NULL,
    address text,
    city character varying(100),
    state character varying(100),
    country character varying(100),
    postal_code character varying(20),
    lat numeric(10,8),
    lng numeric(11,8),
    is_primary boolean DEFAULT false,
    is_active boolean DEFAULT true,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);


--
-- Name: organizations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.organizations (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(255) NOT NULL,
    slug character varying(255) NOT NULL,
    plan_type character varying(50) DEFAULT 'free'::character varying,
    is_active boolean DEFAULT true,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: payments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.payments (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    org_id uuid NOT NULL,
    order_id uuid,
    razorpay_order_id character varying(255) NOT NULL,
    razorpay_payment_id character varying(255),
    amount bigint NOT NULL,
    currency character varying(10) DEFAULT 'INR'::character varying NOT NULL,
    status character varying(20) DEFAULT 'created'::character varying NOT NULL,
    method character varying(50),
    failure_reason text,
    raw_payload jsonb,
    is_mock boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    raw_payload_enc bytea,
    CONSTRAINT payments_amount_check CHECK ((amount > 0)),
    CONSTRAINT payments_status_check CHECK (((status)::text = ANY ((ARRAY['created'::character varying, 'authorized'::character varying, 'captured'::character varying, 'failed'::character varying, 'refunded'::character varying])::text[])))
);


--
-- Name: permissions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.permissions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    resource character varying(50) NOT NULL,
    action character varying(50) NOT NULL,
    description text
);


--
-- Name: products; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.products (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    category_id uuid NOT NULL,
    name character varying(255) NOT NULL,
    description text,
    sku character varying(100) NOT NULL,
    price bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    org_id uuid NOT NULL,
    search_vector tsvector
);


--
-- Name: refresh_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.refresh_tokens (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    family_id uuid NOT NULL,
    token_hash text NOT NULL,
    issued_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    revoked_at timestamp with time zone,
    revoked_reason character varying(50),
    user_agent text,
    ip_address character varying(45)
);


--
-- Name: return_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.return_items (
    id uuid NOT NULL,
    return_id uuid NOT NULL,
    order_item_id uuid NOT NULL,
    product_id uuid NOT NULL,
    quantity integer NOT NULL,
    unit_price bigint NOT NULL,
    CONSTRAINT return_items_quantity_check CHECK ((quantity > 0))
);


--
-- Name: return_requests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.return_requests (
    id uuid NOT NULL,
    org_id uuid NOT NULL,
    order_id uuid NOT NULL,
    requested_by uuid NOT NULL,
    status text DEFAULT 'requested'::text NOT NULL,
    reason text NOT NULL,
    refund_amount bigint DEFAULT 0 NOT NULL,
    rejection_note text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    approved_at timestamp with time zone,
    received_at timestamp with time zone,
    refunded_at timestamp with time zone,
    CONSTRAINT return_requests_status_check CHECK ((status = ANY (ARRAY['requested'::text, 'approved'::text, 'rejected'::text, 'in_transit'::text, 'received'::text, 'refunded'::text])))
);


--
-- Name: role_permissions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.role_permissions (
    role_id uuid NOT NULL,
    permission_id uuid NOT NULL
);


--
-- Name: roles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.roles (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(50) NOT NULL,
    description text,
    is_system boolean DEFAULT false,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);


--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--




--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.users (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(255) NOT NULL,
    email character varying(255) NOT NULL,
    password_hash text NOT NULL,
    role character varying(20) DEFAULT 'staff'::character varying NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    org_id uuid NOT NULL,
    email_verified boolean DEFAULT false NOT NULL,
    failed_login_count integer DEFAULT 0 NOT NULL,
    locked_until timestamp with time zone,
    last_login_at timestamp with time zone,
    password_changed_at timestamp with time zone DEFAULT now() NOT NULL,
    totp_secret text,
    totp_enabled boolean DEFAULT false NOT NULL,
    totp_verified_at timestamp with time zone,
    totp_backup_codes text,
    email_2fa_enabled boolean DEFAULT false NOT NULL
);


--
-- Name: webhook_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE IF NOT EXISTS public.webhook_events (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    provider character varying(50) NOT NULL,
    event_id character varying(255) NOT NULL,
    event_type character varying(100) NOT NULL,
    signature text,
    payload jsonb,
    status character varying(20) DEFAULT 'received'::character varying NOT NULL,
    error text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    processed_at timestamp with time zone,
    payload_enc bytea,
    attempts integer DEFAULT 0 NOT NULL,
    next_retry_at timestamp with time zone,
    dlq boolean DEFAULT false NOT NULL,
    CONSTRAINT webhook_events_status_check CHECK (((status)::text = ANY ((ARRAY['received'::character varying, 'processed'::character varying, 'failed'::character varying, 'duplicate'::character varying])::text[])))
);


--
-- Name: audit_logs audit_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.audit_logs
    ADD CONSTRAINT audit_logs_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: cart_items cart_items_cart_id_listing_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.cart_items
    ADD CONSTRAINT cart_items_cart_id_listing_id_key UNIQUE (cart_id, listing_id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: cart_items cart_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.cart_items
    ADD CONSTRAINT cart_items_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: carts carts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.carts
    ADD CONSTRAINT carts_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: categories categories_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.categories
    ADD CONSTRAINT categories_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: customer_addresses customer_addresses_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.customer_addresses
    ADD CONSTRAINT customer_addresses_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: customers customers_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.customers
    ADD CONSTRAINT customers_email_key UNIQUE (email);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: customers customers_phone_key; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.customers
    ADD CONSTRAINT customers_phone_key UNIQUE (phone);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: customers customers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.customers
    ADD CONSTRAINT customers_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: email_verifications email_verifications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.email_verifications
    ADD CONSTRAINT email_verifications_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: inventory inventory_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.inventory
    ADD CONSTRAINT inventory_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: inventory inventory_product_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.inventory
    ADD CONSTRAINT inventory_product_id_key UNIQUE (product_id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: inventory_reservations inventory_reservations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.inventory_reservations
    ADD CONSTRAINT inventory_reservations_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: login_attempts login_attempts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.login_attempts
    ADD CONSTRAINT login_attempts_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: marketplace_listings marketplace_listings_org_id_product_id_location_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.marketplace_listings
    ADD CONSTRAINT marketplace_listings_org_id_product_id_location_id_key UNIQUE (org_id, product_id, location_id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: marketplace_listings marketplace_listings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.marketplace_listings
    ADD CONSTRAINT marketplace_listings_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: notifications notifications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: order_items order_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.order_items
    ADD CONSTRAINT order_items_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: orders orders_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: org_locations org_locations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.org_locations
    ADD CONSTRAINT org_locations_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: organizations organizations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.organizations
    ADD CONSTRAINT organizations_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: organizations organizations_slug_key; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.organizations
    ADD CONSTRAINT organizations_slug_key UNIQUE (slug);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: payments payments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.payments
    ADD CONSTRAINT payments_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: payments payments_razorpay_order_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.payments
    ADD CONSTRAINT payments_razorpay_order_id_key UNIQUE (razorpay_order_id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: permissions permissions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.permissions
    ADD CONSTRAINT permissions_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: permissions permissions_resource_action_key; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.permissions
    ADD CONSTRAINT permissions_resource_action_key UNIQUE (resource, action);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: products products_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: products products_sku_key; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_sku_key UNIQUE (sku);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: refresh_tokens refresh_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.refresh_tokens
    ADD CONSTRAINT refresh_tokens_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: refresh_tokens refresh_tokens_token_hash_key; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.refresh_tokens
    ADD CONSTRAINT refresh_tokens_token_hash_key UNIQUE (token_hash);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: return_items return_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.return_items
    ADD CONSTRAINT return_items_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: return_items return_items_return_id_order_item_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.return_items
    ADD CONSTRAINT return_items_return_id_order_item_id_key UNIQUE (return_id, order_item_id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: return_requests return_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.return_requests
    ADD CONSTRAINT return_requests_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: role_permissions role_permissions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.role_permissions
    ADD CONSTRAINT role_permissions_pkey PRIMARY KEY (role_id, permission_id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: roles roles_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.roles
    ADD CONSTRAINT roles_name_key UNIQUE (name);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: roles roles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.roles
    ADD CONSTRAINT roles_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--




--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: webhook_events webhook_events_event_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.webhook_events
    ADD CONSTRAINT webhook_events_event_id_key UNIQUE (event_id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: webhook_events webhook_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.webhook_events
    ADD CONSTRAINT webhook_events_pkey PRIMARY KEY (id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: idx_audit_logs_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON public.audit_logs USING btree (created_at);


--
-- Name: idx_audit_logs_entity; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_audit_logs_entity ON public.audit_logs USING btree (entity);


--
-- Name: idx_audit_logs_org_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_audit_logs_org_id ON public.audit_logs USING btree (org_id);


--
-- Name: idx_audit_logs_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON public.audit_logs USING btree (user_id);


--
-- Name: idx_cart_items_cart_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_cart_items_cart_id ON public.cart_items USING btree (cart_id);


--
-- Name: idx_carts_buyer_org_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_carts_buyer_org_id ON public.carts USING btree (buyer_org_id);


--
-- Name: idx_carts_customer_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_carts_customer_id ON public.carts USING btree (customer_id);


--
-- Name: idx_categories_org_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_categories_org_id ON public.categories USING btree (org_id);


--
-- Name: idx_customer_addresses_customer_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_customer_addresses_customer_id ON public.customer_addresses USING btree (customer_id);


--
-- Name: idx_customers_email; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_customers_email ON public.customers USING btree (email);


--
-- Name: idx_customers_phone; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_customers_phone ON public.customers USING btree (phone);


--
-- Name: idx_email_verifications_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_email_verifications_active ON public.email_verifications USING btree (expires_at) WHERE (consumed_at IS NULL);


--
-- Name: idx_email_verifications_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_email_verifications_user_id ON public.email_verifications USING btree (user_id);


--
-- Name: idx_inventory_org_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_inventory_org_id ON public.inventory USING btree (org_id);


--
-- Name: idx_inventory_reservations_expires; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_inventory_reservations_expires ON public.inventory_reservations USING btree (expires_at) WHERE ((status)::text = 'active'::text);


--
-- Name: idx_inventory_reservations_inventory_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_inventory_reservations_inventory_id ON public.inventory_reservations USING btree (inventory_id);


--
-- Name: idx_inventory_reservations_order_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_inventory_reservations_order_id ON public.inventory_reservations USING btree (order_id);


--
-- Name: idx_inventory_reservations_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_inventory_reservations_status ON public.inventory_reservations USING btree (status) WHERE ((status)::text = 'active'::text);


--
-- Name: idx_login_attempts_email_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_login_attempts_email_time ON public.login_attempts USING btree (email, attempted_at);


--
-- Name: idx_login_attempts_ip_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_login_attempts_ip_time ON public.login_attempts USING btree (ip_address, attempted_at);


--
-- Name: idx_marketplace_listings_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_marketplace_listings_active ON public.marketplace_listings USING btree (is_active) WHERE (is_active = true);


--
-- Name: idx_marketplace_listings_org_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_marketplace_listings_org_id ON public.marketplace_listings USING btree (org_id);


--
-- Name: idx_marketplace_listings_product_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_marketplace_listings_product_id ON public.marketplace_listings USING btree (product_id);


--
-- Name: idx_notifications_due; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_notifications_due ON public.notifications USING btree (next_attempt) WHERE (status = 'pending'::text);


--
-- Name: idx_notifications_status_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_notifications_status_created ON public.notifications USING btree (status, created_at DESC);


--
-- Name: idx_order_items_order_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON public.order_items USING btree (order_id);


--
-- Name: idx_order_items_org_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_order_items_org_id ON public.order_items USING btree (org_id);


--
-- Name: idx_orders_buyer_org_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_orders_buyer_org_id ON public.orders USING btree (buyer_org_id);


--
-- Name: idx_orders_customer_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_orders_customer_id ON public.orders USING btree (customer_id);


--
-- Name: idx_orders_order_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_orders_order_type ON public.orders USING btree (order_type);


--
-- Name: idx_orders_org_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_orders_org_id ON public.orders USING btree (org_id);


--
-- Name: idx_orders_payment_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_orders_payment_status ON public.orders USING btree (payment_status);


--
-- Name: idx_orders_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_orders_status ON public.orders USING btree (status);


--
-- Name: idx_orders_supplier_org_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_orders_supplier_org_id ON public.orders USING btree (supplier_org_id);


--
-- Name: idx_orders_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_orders_user_id ON public.orders USING btree (user_id);


--
-- Name: idx_org_locations_lat_lng; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_org_locations_lat_lng ON public.org_locations USING btree (lat, lng);


--
-- Name: idx_org_locations_org_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_org_locations_org_id ON public.org_locations USING btree (org_id);


--
-- Name: idx_payments_order_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_payments_order_id ON public.payments USING btree (order_id);


--
-- Name: idx_payments_org_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_payments_org_id ON public.payments USING btree (org_id);


--
-- Name: idx_payments_rzp_order; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_payments_rzp_order ON public.payments USING btree (razorpay_order_id);


--
-- Name: idx_payments_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_payments_status ON public.payments USING btree (status);


--
-- Name: idx_products_category_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_products_category_id ON public.products USING btree (category_id);


--
-- Name: idx_products_name_trgm; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_products_name_trgm ON public.products USING gin (name public.gin_trgm_ops);


--
-- Name: idx_products_org_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_products_org_id ON public.products USING btree (org_id);


--
-- Name: idx_products_search_vector; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_products_search_vector ON public.products USING gin (search_vector);


--
-- Name: idx_products_sku_trgm; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_products_sku_trgm ON public.products USING gin (sku public.gin_trgm_ops);


--
-- Name: idx_refresh_tokens_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_active ON public.refresh_tokens USING btree (expires_at) WHERE (revoked_at IS NULL);


--
-- Name: idx_refresh_tokens_family_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family_id ON public.refresh_tokens USING btree (family_id);


--
-- Name: idx_refresh_tokens_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_hash ON public.refresh_tokens USING btree (token_hash);


--
-- Name: idx_refresh_tokens_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON public.refresh_tokens USING btree (user_id);


--
-- Name: idx_return_items_return; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_return_items_return ON public.return_items USING btree (return_id);


--
-- Name: idx_return_requests_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_return_requests_created ON public.return_requests USING btree (created_at DESC);


--
-- Name: idx_return_requests_order; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_return_requests_order ON public.return_requests USING btree (order_id);


--
-- Name: idx_return_requests_org_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_return_requests_org_status ON public.return_requests USING btree (org_id, status);


--
-- Name: idx_users_email; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_users_email ON public.users USING btree (email);


--
-- Name: idx_users_org_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_users_org_id ON public.users USING btree (org_id);


--
-- Name: idx_webhook_events_dlq; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_webhook_events_dlq ON public.webhook_events USING btree (created_at DESC) WHERE (dlq = true);


--
-- Name: idx_webhook_events_event_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_webhook_events_event_id ON public.webhook_events USING btree (event_id);


--
-- Name: idx_webhook_events_provider_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_webhook_events_provider_type ON public.webhook_events USING btree (provider, event_type);


--
-- Name: idx_webhook_events_retry; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_webhook_events_retry ON public.webhook_events USING btree (next_retry_at) WHERE (((status)::text = 'failed'::text) AND (dlq = false));


--
-- Name: idx_webhook_events_unprocessed; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX IF NOT EXISTS idx_webhook_events_unprocessed ON public.webhook_events USING btree (created_at) WHERE ((status)::text = 'received'::text);


--
-- Name: products products_search_vector_update; Type: TRIGGER; Schema: public; Owner: -
--

DROP TRIGGER IF EXISTS products_search_vector_update ON public.products;
CREATE TRIGGER products_search_vector_update BEFORE INSERT OR UPDATE ON public.products FOR EACH ROW EXECUTE FUNCTION public.update_product_search_vector();


--
-- Name: audit_logs audit_logs_org_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.audit_logs
    ADD CONSTRAINT audit_logs_org_id_fkey FOREIGN KEY (org_id) REFERENCES public.organizations(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: audit_logs audit_logs_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.audit_logs
    ADD CONSTRAINT audit_logs_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE SET NULL;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: cart_items cart_items_cart_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.cart_items
    ADD CONSTRAINT cart_items_cart_id_fkey FOREIGN KEY (cart_id) REFERENCES public.carts(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: cart_items cart_items_listing_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.cart_items
    ADD CONSTRAINT cart_items_listing_id_fkey FOREIGN KEY (listing_id) REFERENCES public.marketplace_listings(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: carts carts_buyer_org_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.carts
    ADD CONSTRAINT carts_buyer_org_id_fkey FOREIGN KEY (buyer_org_id) REFERENCES public.organizations(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: carts carts_customer_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.carts
    ADD CONSTRAINT carts_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES public.customers(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: categories categories_org_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.categories
    ADD CONSTRAINT categories_org_id_fkey FOREIGN KEY (org_id) REFERENCES public.organizations(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: customer_addresses customer_addresses_customer_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.customer_addresses
    ADD CONSTRAINT customer_addresses_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES public.customers(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: email_verifications email_verifications_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.email_verifications
    ADD CONSTRAINT email_verifications_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: inventory inventory_org_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.inventory
    ADD CONSTRAINT inventory_org_id_fkey FOREIGN KEY (org_id) REFERENCES public.organizations(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: inventory inventory_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.inventory
    ADD CONSTRAINT inventory_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: inventory_reservations inventory_reservations_inventory_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.inventory_reservations
    ADD CONSTRAINT inventory_reservations_inventory_id_fkey FOREIGN KEY (inventory_id) REFERENCES public.inventory(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: inventory_reservations inventory_reservations_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.inventory_reservations
    ADD CONSTRAINT inventory_reservations_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.orders(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: inventory_reservations inventory_reservations_org_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.inventory_reservations
    ADD CONSTRAINT inventory_reservations_org_id_fkey FOREIGN KEY (org_id) REFERENCES public.organizations(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: marketplace_listings marketplace_listings_location_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.marketplace_listings
    ADD CONSTRAINT marketplace_listings_location_id_fkey FOREIGN KEY (location_id) REFERENCES public.org_locations(id) ON DELETE SET NULL;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: marketplace_listings marketplace_listings_org_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.marketplace_listings
    ADD CONSTRAINT marketplace_listings_org_id_fkey FOREIGN KEY (org_id) REFERENCES public.organizations(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: marketplace_listings marketplace_listings_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.marketplace_listings
    ADD CONSTRAINT marketplace_listings_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: order_items order_items_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.order_items
    ADD CONSTRAINT order_items_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.orders(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: order_items order_items_org_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.order_items
    ADD CONSTRAINT order_items_org_id_fkey FOREIGN KEY (org_id) REFERENCES public.organizations(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: order_items order_items_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.order_items
    ADD CONSTRAINT order_items_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE RESTRICT;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: orders orders_buyer_org_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_buyer_org_id_fkey FOREIGN KEY (buyer_org_id) REFERENCES public.organizations(id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: orders orders_customer_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES public.customers(id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: orders orders_delivery_address_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_delivery_address_id_fkey FOREIGN KEY (delivery_address_id) REFERENCES public.customer_addresses(id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: orders orders_org_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_org_id_fkey FOREIGN KEY (org_id) REFERENCES public.organizations(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: orders orders_supplier_location_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_supplier_location_id_fkey FOREIGN KEY (supplier_location_id) REFERENCES public.org_locations(id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: orders orders_supplier_org_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_supplier_org_id_fkey FOREIGN KEY (supplier_org_id) REFERENCES public.organizations(id);
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: orders orders_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE RESTRICT;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: org_locations org_locations_org_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.org_locations
    ADD CONSTRAINT org_locations_org_id_fkey FOREIGN KEY (org_id) REFERENCES public.organizations(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: payments payments_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.payments
    ADD CONSTRAINT payments_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.orders(id) ON DELETE SET NULL;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: payments payments_org_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.payments
    ADD CONSTRAINT payments_org_id_fkey FOREIGN KEY (org_id) REFERENCES public.organizations(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: products products_category_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_category_id_fkey FOREIGN KEY (category_id) REFERENCES public.categories(id) ON DELETE RESTRICT;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: products products_org_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_org_id_fkey FOREIGN KEY (org_id) REFERENCES public.organizations(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: refresh_tokens refresh_tokens_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.refresh_tokens
    ADD CONSTRAINT refresh_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: return_items return_items_order_item_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.return_items
    ADD CONSTRAINT return_items_order_item_id_fkey FOREIGN KEY (order_item_id) REFERENCES public.order_items(id) ON DELETE RESTRICT;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: return_items return_items_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.return_items
    ADD CONSTRAINT return_items_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE RESTRICT;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: return_items return_items_return_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.return_items
    ADD CONSTRAINT return_items_return_id_fkey FOREIGN KEY (return_id) REFERENCES public.return_requests(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: return_requests return_requests_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.return_requests
    ADD CONSTRAINT return_requests_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.orders(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: return_requests return_requests_org_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.return_requests
    ADD CONSTRAINT return_requests_org_id_fkey FOREIGN KEY (org_id) REFERENCES public.organizations(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: return_requests return_requests_requested_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.return_requests
    ADD CONSTRAINT return_requests_requested_by_fkey FOREIGN KEY (requested_by) REFERENCES public.users(id) ON DELETE RESTRICT;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: role_permissions role_permissions_permission_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.role_permissions
    ADD CONSTRAINT role_permissions_permission_id_fkey FOREIGN KEY (permission_id) REFERENCES public.permissions(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: role_permissions role_permissions_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.role_permissions
    ADD CONSTRAINT role_permissions_role_id_fkey FOREIGN KEY (role_id) REFERENCES public.roles(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: users users_org_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_org_id_fkey FOREIGN KEY (org_id) REFERENCES public.organizations(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- Name: users users_role_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

DO $do$ BEGIN
  ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_role_fkey FOREIGN KEY (role) REFERENCES public.roles(name) ON UPDATE CASCADE ON DELETE RESTRICT;
EXCEPTION WHEN duplicate_object THEN NULL;
  WHEN duplicate_table THEN NULL;
END $do$;


--
-- PostgreSQL database dump complete
--


