CREATE TABLE IF NOT EXISTS region_external_mappings (
  id BIGSERIAL PRIMARY KEY,
  region_id BIGINT NOT NULL REFERENCES regions(id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  external_code TEXT NOT NULL,
  external_name TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS region_external_mappings_provider_code_uq
  ON region_external_mappings (provider, external_code);

CREATE INDEX IF NOT EXISTS region_external_mappings_region_id_idx
  ON region_external_mappings (region_id);
