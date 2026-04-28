-- name: GetLatestActiveAppVersion :one
SELECT id, platform, channel, package_name, version_code, version_name, download_url, changelog, is_force, file_size_bytes, checksum_sha256, status, published_at, created_at, updated_at
FROM app_versions
WHERE platform = sqlc.arg('platform')
  AND channel = sqlc.arg('channel')
  AND package_name = sqlc.arg('package_name')
  AND version_code > sqlc.arg('current_version_code')
  AND status = 'active'
  AND published_at <= now()
ORDER BY version_code DESC, published_at DESC, id DESC
LIMIT 1;