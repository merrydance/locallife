CREATE TABLE wanted_merchants (
  id bigserial PRIMARY KEY,
  region_id bigint NOT NULL REFERENCES regions(id),
  normalized_name text NOT NULL,
  display_name text NOT NULL,
  address text,
  latitude decimal(10,7),
  longitude decimal(10,7),
  source text NOT NULL DEFAULT 'manual',
  status text NOT NULL DEFAULT 'active',
  want_count integer NOT NULL DEFAULT 0,
  created_by_user_id bigint NOT NULL REFERENCES users(id),
  matched_merchant_id bigint REFERENCES merchants(id),
  last_voted_at timestamptz,
  matched_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT wanted_merchants_source_check CHECK (source IN ('manual', 'map')),
  CONSTRAINT wanted_merchants_status_check CHECK (status IN ('active', 'matched', 'removed')),
  CONSTRAINT wanted_merchants_want_count_check CHECK (want_count >= 0)
);

CREATE UNIQUE INDEX wanted_merchants_active_region_name_uidx
  ON wanted_merchants(region_id, normalized_name)
  WHERE status = 'active';

CREATE INDEX wanted_merchants_region_rank_idx
  ON wanted_merchants(region_id, status, want_count DESC, last_voted_at DESC, id ASC);

CREATE INDEX wanted_merchants_matched_merchant_idx
  ON wanted_merchants(matched_merchant_id)
  WHERE matched_merchant_id IS NOT NULL;

CREATE TABLE wanted_merchant_votes (
  id bigserial PRIMARY KEY,
  wanted_merchant_id bigint NOT NULL REFERENCES wanted_merchants(id) ON DELETE CASCADE,
  region_id bigint NOT NULL REFERENCES regions(id),
  user_id bigint NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT wanted_merchant_votes_unique_user_candidate UNIQUE(region_id, user_id, wanted_merchant_id)
);

CREATE INDEX wanted_merchant_votes_candidate_idx
  ON wanted_merchant_votes(wanted_merchant_id);

CREATE INDEX wanted_merchant_votes_region_user_idx
  ON wanted_merchant_votes(region_id, user_id);
