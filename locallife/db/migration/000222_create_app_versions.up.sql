CREATE TABLE IF NOT EXISTS app_versions (
    id BIGSERIAL PRIMARY KEY,
    platform VARCHAR(32) NOT NULL,
    channel VARCHAR(64) NOT NULL,
    package_name VARCHAR(120) NOT NULL,
    version_code INTEGER NOT NULL,
    version_name VARCHAR(50) NOT NULL,
    download_url TEXT NOT NULL,
    changelog TEXT NOT NULL DEFAULT '',
    is_force BOOLEAN NOT NULL DEFAULT FALSE,
    file_size_bytes BIGINT,
    checksum_sha256 VARCHAR(64),
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    published_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT app_versions_platform_check CHECK (platform IN ('android')),
    CONSTRAINT app_versions_channel_check CHECK (channel IN ('merchant_app')),
    CONSTRAINT app_versions_version_code_check CHECK (version_code > 0),
    CONSTRAINT app_versions_status_check CHECK (status IN ('draft', 'active', 'disabled')),
    CONSTRAINT app_versions_package_name_not_blank CHECK (btrim(package_name) <> ''),
    CONSTRAINT app_versions_version_name_not_blank CHECK (btrim(version_name) <> ''),
    CONSTRAINT app_versions_download_url_not_blank CHECK (btrim(download_url) <> ''),
    CONSTRAINT app_versions_checksum_sha256_check CHECK (checksum_sha256 IS NULL OR checksum_sha256 ~ '^[a-fA-F0-9]{64}$'),
    CONSTRAINT app_versions_file_size_bytes_check CHECK (file_size_bytes IS NULL OR file_size_bytes > 0)
);

CREATE UNIQUE INDEX idx_app_versions_platform_channel_package_version
    ON app_versions(platform, channel, package_name, version_code);

CREATE INDEX idx_app_versions_active_latest
    ON app_versions(platform, channel, package_name, version_code DESC, published_at DESC)
    WHERE status = 'active';

COMMENT ON TABLE app_versions IS 'Published mobile app versions for update checks';
COMMENT ON COLUMN app_versions.channel IS 'Client channel, currently merchant_app';
COMMENT ON COLUMN app_versions.download_url IS 'Public APK download URL for active releases';