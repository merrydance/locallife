#!/usr/bin/env bash

set -euo pipefail

base_ref="${1:-}"
head_ref="${2:-HEAD}"
max_lines="${MAX_FILE_LINES:-500}"
repo_root="$(git rev-parse --show-toplevel)"

if [[ -z "$base_ref" || "$base_ref" == "0000000000000000000000000000000000000000" ]]; then
  echo "No usable base ref provided; skipping changed-file size guardrail."
  exit 0
fi

mapfile -t changed_files < <(
  git -C "$repo_root" diff --name-only --diff-filter=ACMR "$base_ref" "$head_ref" | \
    grep -E '^locallife/(api|logic|worker)/.*\.go$' | \
    grep -Ev '(_test\.go$|mock_.*\.go$|\.sql\.go$|/mock/|/sqlc/)' || true
)

if [[ ${#changed_files[@]} -eq 0 ]]; then
  echo "No changed backend handler/logic/worker files matched the size guardrail."
  exit 0
fi

violations=0

echo "Checking changed backend files against ${max_lines}-line non-regression guardrail..."

for file in "${changed_files[@]}"; do
  abs_path="$repo_root/$file"
  if [[ ! -f "$abs_path" ]]; then
    continue
  fi

  current_lines="$(wc -l < "$abs_path")"

  if git -C "$repo_root" cat-file -e "$base_ref:$file" 2>/dev/null; then
    base_lines="$(git -C "$repo_root" show "$base_ref:$file" | wc -l)"
  else
    base_lines=0
  fi

  if (( base_lines == 0 )); then
    if (( current_lines > max_lines )); then
      echo "  ❌ new oversized file: $file ($current_lines lines, limit $max_lines)"
      violations=$((violations + 1))
    else
      echo "  ✅ new file within limit: $file ($current_lines lines)"
    fi
    continue
  fi

  if (( base_lines > max_lines )); then
    if (( current_lines > base_lines )); then
      echo "  ❌ legacy oversized file grew: $file ($base_lines -> $current_lines lines)"
      violations=$((violations + 1))
    elif (( current_lines < base_lines )); then
      echo "  ✅ legacy oversized file shrank: $file ($base_lines -> $current_lines lines)"
    else
      echo "  ⚠️ legacy oversized file unchanged: $file ($current_lines lines)"
    fi
    continue
  fi

  if (( current_lines > max_lines )); then
    echo "  ❌ file crossed size limit: $file ($base_lines -> $current_lines lines, limit $max_lines)"
    violations=$((violations + 1))
  else
    echo "  ✅ file within limit: $file ($base_lines -> $current_lines lines)"
  fi
done

if (( violations > 0 )); then
  echo
  echo "Changed-file size guardrail failed with $violations violation(s)."
  exit 1
fi

echo
echo "Changed-file size guardrail passed. Legacy oversized files may still exist; use 'make lint-filesize' for full-audit debt visibility."