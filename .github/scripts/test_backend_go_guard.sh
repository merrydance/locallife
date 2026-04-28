#!/usr/bin/env bash

set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
guard_script="$repo_root/.github/scripts/backend_go_guard.sh"
tmp_root="$(mktemp -d)"

cleanup() {
  rm -rf "$tmp_root"
}

trap cleanup EXIT

run_case() {
  local case_name="$1"
  local expected_exit="$2"
  local expected_text="$3"
  local base_go="$4"
  local head_go="$5"
  local rel_path="${6:-locallife/logic/sample.go}"

  local case_dir="$tmp_root/$case_name"
  mkdir -p "$case_dir/$(dirname "$rel_path")"

  pushd "$case_dir" >/dev/null
  git init -q

  printf '%s
' "$base_go" > "$rel_path"
  git add "$rel_path"
  git -c user.name='Go Guard Test' -c user.email='go-guard-test@example.com' commit -q -m 'base'

  printf '%s
' "$head_go" > "$rel_path"
  git add "$rel_path"
  git -c user.name='Go Guard Test' -c user.email='go-guard-test@example.com' commit -q -m 'head'

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
  unstructured_log_violation \
  1 \
  "unstructured logging" \
  "package logic

func run() {}" \
  "package logic

import \"fmt\"

func run() {
    fmt.Println(\"oops\")
}"

run_case \
  background_context_violation \
  1 \
  "adds context.Background()" \
  "package logic

func run(ctx context.Context) {}" \
  "package logic

import \"context\"

func run(ctx context.Context) {
    _ = context.Background()
}"

run_case \
  panic_violation \
  1 \
  "adds panic(...)" \
  "package logic

func run() error { return nil }" \
  "package logic

func run() error {
    panic(\"boom\")
}"

run_case \
  ignored_parse_json_violation \
  1 \
  "ignores JSON decode errors" \
  "package api

func run() {}" \
  "package api

func run() {
    _ = parseJSON(payload, &resp)
}" \
  "locallife/api/sample.go"

run_case \
  ignored_json_unmarshal_violation \
  1 \
  "ignores JSON decode errors" \
  "package logic

func run() {}" \
  "package logic

import \"encoding/json\"

func run() {
    _ = json.Unmarshal(data, &resp)
}"

run_case \
  logic_request_error_5xx_violation \
  1 \
  "NewRequestError(...) with a 5xx status" \
  "package logic

func run() error { return nil }" \
  "package logic

func run() error {
    return NewRequestError(http.StatusInternalServerError, errors.New(\"boom\"))
}"

run_case \
  api_error_response_5xx_violation \
  1 \
  "ctx.JSON(..., errorResponse(...)) for a 5xx status" \
  "package api

func run() {}" \
  "package api

func run(ctx *gin.Context) {
    ctx.JSON(http.StatusBadGateway, errorResponse(err))
}" \
  "locallife/api/sample.go"

run_case \
  api_internal_error_passes \
  0 \
  "Go guardrail passed" \
  "package api

func run() {}" \
  "package api

func run(ctx *gin.Context, err error) {
    ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
}" \
  "locallife/api/sample.go"

run_case \
  handled_parse_json_passes \
  0 \
  "Go guardrail passed" \
  "package api

func run() {}" \
  "package api

func run() error {
    if err := parseJSON(payload, &resp); err != nil {
        return err
    }
    return nil
}" \
  "locallife/api/sample.go"

run_case \
  panic_allow_passes \
  0 \
  "Go guardrail passed" \
  "package util

func mustInit() {}" \
  "package util

func mustInit() {
    panic(\"boom\") // goguard: allow-panic process bootstrap invariant
}" \
  "locallife/util/sample.go"

run_case \
  bare_allow_panic_rejected \
  1 \
  "bare 'goguard: allow-panic'" \
  "package util

func mustInit() {}" \
  "package util

func mustInit() {
    panic(\"boom\") // goguard: allow-panic
}" \
  "locallife/util/sample.go"

run_case \
  bare_allow_background_context_rejected \
  1 \
  "bare 'goguard: allow-background-context'" \
  "package logic

import \"context\"

func run(ctx context.Context) {}" \
  "package logic

import \"context\"

func run(ctx context.Context) {
    _ = context.Background() // goguard: allow-background-context
}"

run_case \
  test_file_ignored \
  0 \
  "No changed non-test Go files matched" \
  "package logic

func TestRun() {}" \
  "package logic

func TestRun() {
    panic(\"test panic\")
}" \
  "locallife/logic/sample_test.go"

echo "backend_go_guard self-test passed."

