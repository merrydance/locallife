#!/usr/bin/env bash

set -euo pipefail

base_ref="${1:-}"
head_ref="${2:-HEAD}"
repo_root="$(git rev-parse --show-toplevel)"

if [[ -z "$base_ref" || "$base_ref" == "0000000000000000000000000000000000000000" ]]; then
  echo "No usable base ref provided; skipping Flutter changed-file guardrail."
  exit 0
fi

mapfile -t changed_files < <(
  git -C "$repo_root" diff --name-only --diff-filter=ACMR "$base_ref" "$head_ref" | \
    grep -E '^merchant_app/lib/.*\.dart$' || true
)

if [[ ${#changed_files[@]} -eq 0 ]]; then
  echo "No changed Flutter Dart files matched the architecture guardrail."
  exit 0
fi

violations=0
widget_like_pattern='^merchant_app/lib/widgets/.*\.dart$|^merchant_app/lib/features/.*/.*(_page|_dialog)\.dart$'
forbidden_import_pattern="package:dio/dio.dart|package:flutter_secure_storage/flutter_secure_storage.dart|package:sqflite/sqflite.dart|package:merchant_app/core/network/api_client.dart|package:merchant_app/core/network/api_provider.dart|package:merchant_app/core/network/ws_client.dart"
forbidden_direct_use_pattern='(^|[^A-Za-z0-9_])(Dio|FlutterSecureStorage|openDatabase)\('
hardcoded_endpoint_pattern="['\"](https?|wss?)://"

echo "Checking changed Flutter files for architecture and config guardrails..."

for file in "${changed_files[@]}"; do
  abs_path="$repo_root/$file"
  if [[ ! -f "$abs_path" ]]; then
    continue
  fi

  if [[ "$file" =~ $widget_like_pattern ]]; then
    if grep -nE "$forbidden_import_pattern" "$abs_path" >/tmp/flutter_guard_match.txt; then
      echo "  ❌ widget/page layer imported infrastructure directly: $file"
      sed 's/^/     /' /tmp/flutter_guard_match.txt
      violations=$((violations + 1))
    fi

    if grep -nE "$forbidden_direct_use_pattern" "$abs_path" >/tmp/flutter_guard_match.txt; then
      echo "  ❌ widget/page layer instantiated network or storage infrastructure directly: $file"
      sed 's/^/     /' /tmp/flutter_guard_match.txt
      violations=$((violations + 1))
    fi
  fi

  if [[ "$file" != "merchant_app/lib/config/env.dart" ]]; then
    if grep -nE "$hardcoded_endpoint_pattern" "$abs_path" >/tmp/flutter_guard_match.txt; then
      echo "  ❌ hardcoded endpoint found outside env config: $file"
      sed 's/^/     /' /tmp/flutter_guard_match.txt
      violations=$((violations + 1))
    fi
  fi
done

rm -f /tmp/flutter_guard_match.txt

if (( violations > 0 )); then
  echo
  echo "Flutter changed-file guardrail failed with $violations violation(s)."
  exit 1
fi

echo
echo "Flutter changed-file guardrail passed."