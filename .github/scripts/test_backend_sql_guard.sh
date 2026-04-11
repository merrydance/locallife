#!/usr/bin/env bash

set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
guard_script="$repo_root/.github/scripts/backend_sql_guard.sh"
tmp_root="$(mktemp -d)"

cleanup() {
  rm -rf "$tmp_root"
}

trap cleanup EXIT

run_case() {
  local case_name="$1"
  local expected_exit="$2"
  local expected_text="$3"
  local base_sql="$4"
  local head_sql="$5"

  local case_dir="$tmp_root/$case_name"
  mkdir -p "$case_dir/locallife/db/query"

  pushd "$case_dir" >/dev/null
  git init -q

  printf '%s\n' "$base_sql" > locallife/db/query/sample.sql
  git add locallife/db/query/sample.sql
  git -c user.name='SQL Guard Test' -c user.email='sql-guard-test@example.com' commit -q -m 'base'

  printf '%s\n' "$head_sql" > locallife/db/query/sample.sql
  git add locallife/db/query/sample.sql
  git -c user.name='SQL Guard Test' -c user.email='sql-guard-test@example.com' commit -q -m 'head'

  local output
  set +e
  output="$(bash "$guard_script" HEAD~1 HEAD 2>&1)"
  local status=$?
  set -e

  if [[ "$status" -ne "$expected_exit" ]]; then
    echo "Case '$case_name' returned exit code $status, expected $expected_exit"
    echo "$output"
    exit 1
  fi

  if [[ -n "$expected_text" && "$output" != *"$expected_text"* ]]; then
    echo "Case '$case_name' did not include expected output: $expected_text"
    echo "$output"
    exit 1
  fi

  popd >/dev/null
}

run_case \
  select_star_violation \
  1 \
  "uses SELECT *" \
  "-- name: ListSample :many
SELECT id, created_at FROM sample_items
ORDER BY created_at DESC
LIMIT \$1;" \
  "-- name: ListSample :many
SELECT * FROM sample_items
ORDER BY created_at DESC
LIMIT \$1;"

run_case \
  unordered_limit_violation \
  1 \
  "LIMIT/OFFSET but no ORDER BY" \
  "-- name: ListSample :many
SELECT id, created_at FROM sample_items
ORDER BY created_at DESC
LIMIT \$1;" \
  "-- name: ListSample :many
SELECT id, created_at FROM sample_items
LIMIT \$1;"

run_case \
  unscoped_write_violation \
  1 \
  "without WHERE" \
  "-- name: ArchiveSample :exec
UPDATE sample_items
SET archived_at = NOW()
WHERE id = \$1;" \
  "-- name: ArchiveSample :exec
UPDATE sample_items
SET archived_at = NOW();"

run_case \
  justified_exception_passes \
  0 \
  "SQL guardrail passed" \
  "-- name: ArchiveSample :exec
UPDATE sample_items
SET archived_at = NOW()
WHERE id = \$1;" \
  "-- name: ArchiveSample :exec
-- sqlguard: allow-unscoped-write maintenance backfill window
UPDATE sample_items
SET archived_at = NOW();"

run_case \
  allow_select_star_passes \
  0 \
  "SQL guardrail passed" \
  "-- name: ListSample :many
SELECT id, created_at FROM sample_items
ORDER BY created_at DESC
LIMIT \$1;" \
  "-- name: ListSample :many
-- sqlguard: allow-select-star query must stay column-complete for sqlc row mapping parity
SELECT * FROM sample_items
ORDER BY created_at DESC
LIMIT \$1;"

run_case \
  allow_unordered_limit_passes \
  0 \
  "SQL guardrail passed" \
  "-- name: ListSample :many
SELECT id, created_at FROM sample_items
ORDER BY created_at DESC
LIMIT \$1;" \
  "-- name: ListSample :many
-- sqlguard: allow-unordered-limit result is constrained by unique filter upstream
SELECT id, created_at FROM sample_items
LIMIT \$1;"

run_case \
  untouched_violation_in_same_file_does_not_fail \
  0 \
  "SQL guardrail passed" \
  "-- name: LegacyBadQuery :many
SELECT * FROM legacy_items
LIMIT \$1;

-- name: SafeQuery :many
SELECT id, created_at FROM safe_items
ORDER BY created_at DESC
LIMIT \$1;" \
  "-- name: LegacyBadQuery :many
SELECT * FROM legacy_items
LIMIT \$1;

-- name: SafeQuery :many
SELECT id, created_at FROM safe_items
WHERE merchant_id = \$2
ORDER BY created_at DESC
LIMIT \$1;"

run_case \
  comment_only_change_in_legacy_bad_query_does_not_fail \
  0 \
  "SQL guardrail passed" \
  "-- name: LegacyBadQuery :many
SELECT * FROM legacy_items
LIMIT \$1;" \
  "-- name: LegacyBadQuery :many
-- refreshed comment only; SQL should not be re-linted on this change alone
SELECT * FROM legacy_items
LIMIT \$1;"

run_case \
  implicit_insert_columns_violation \
  1 \
  "explicit column list" \
  "-- name: CreateSample :one
INSERT INTO sample_items (id, status)
VALUES (\$1, \$2)
RETURNING *;" \
  "-- name: CreateSample :one
INSERT INTO sample_items
VALUES (\$1, \$2)
RETURNING *;"

run_case \
  allow_implicit_insert_columns_passes \
  0 \
  "SQL guardrail passed" \
  "-- name: CreateSample :one
INSERT INTO sample_items (id, status)
VALUES (\$1, \$2)
RETURNING *;" \
  "-- name: CreateSample :one
-- sqlguard: allow-implicit-insert-columns legacy fixture shape is fixed by vendor contract
INSERT INTO sample_items
VALUES (\$1, \$2)
RETURNING *;"

run_case \
  bare_allow_select_star_rejected \
  1 \
  "bare 'sqlguard: allow-select-star'" \
  "-- name: ListSample :many
SELECT id, created_at FROM sample_items
ORDER BY created_at DESC
LIMIT \$1;" \
  "-- name: ListSample :many
-- sqlguard: allow-select-star
SELECT * FROM sample_items
ORDER BY created_at DESC
LIMIT \$1;"

run_case \
  bare_allow_implicit_insert_rejected \
  1 \
  "bare 'sqlguard: allow-implicit-insert-columns'" \
  "-- name: CreateSample :one
INSERT INTO sample_items (id, status)
VALUES (\$1, \$2)
RETURNING *;" \
  "-- name: CreateSample :one
-- sqlguard: allow-implicit-insert-columns
INSERT INTO sample_items
VALUES (\$1, \$2)
RETURNING *;"

run_case \
  query_name_change_still_checks_bad_sql \
  1 \
  "uses SELECT *" \
  "-- name: LegacyBadQuery :many
SELECT * FROM legacy_items
LIMIT \$1;" \
  "-- name: RenamedLegacyBadQuery :many
SELECT * FROM legacy_items
LIMIT \$1;"

echo "backend_sql_guard self-test passed."