#!/usr/bin/env bash

set -euo pipefail

base_ref="${1:-}"
head_ref="${2:-HEAD}"
repo_root="$(git rev-parse --show-toplevel)"

if [[ -z "$base_ref" || "$base_ref" == "0000000000000000000000000000000000000000" ]]; then
  echo "No usable base ref provided; skipping SQL guardrail."
  exit 0
fi

mapfile -t changed_files < <(
  git -C "$repo_root" diff --name-only --diff-filter=ACMR "$base_ref" "$head_ref" | \
    grep -E '^locallife/db/query/.*\.sql$' || true
)

if [[ ${#changed_files[@]} -eq 0 ]]; then
  echo "No changed SQL query files matched the SQL guardrail."
  exit 0
fi

collect_ranges() {
  local mode="$1"
  local file="$2"
  local line start count end

  while IFS= read -r line; do
    [[ "$line" == @@* ]] || continue

    if [[ "$mode" == "old" && "$line" =~ ^@@[[:space:]]-([0-9]+)(,([0-9]+))?[[:space:]]\+ ]]; then
      start="${BASH_REMATCH[1]}"
      count="${BASH_REMATCH[3]:-1}"
    elif [[ "$mode" == "new" && "$line" =~ ^@@[[:space:]]-[0-9]+(,[0-9]+)?[[:space:]]\+([0-9]+)(,([0-9]+))?[[:space:]]@@ ]]; then
      start="${BASH_REMATCH[2]}"
      count="${BASH_REMATCH[4]:-1}"
    else
      continue
    fi

    if (( count > 0 )); then
      end=$((start + count - 1))
      printf '%s:%s\n' "$start" "$end"
    fi
  done < <(git -C "$repo_root" diff --unified=0 "$base_ref" "$head_ref" -- "$file")
}

query_names_for_ranges() {
  local ref_kind="$1"
  local file="$2"
  local ranges="$3"

  if [[ -z "$ranges" ]]; then
    return 0
  fi

  local content
  if [[ "$ref_kind" == "head" ]]; then
    [[ -f "$repo_root/$file" ]] || return 0
    content="$(<"$repo_root/$file")"
  else
    if ! git -C "$repo_root" cat-file -e "$ref_kind:$file" 2>/dev/null; then
      return 0
    fi
    content="$(git -C "$repo_root" show "$ref_kind:$file")"
  fi

  printf '%s\n' "$content" | awk -v ranges="$ranges" '
    BEGIN {
      range_count = split(ranges, raw_ranges, /[[:space:]]+/)
      for (i = 1; i <= range_count; i++) {
        if (raw_ranges[i] == "") {
          continue
        }
        split(raw_ranges[i], bounds, ":")
        starts[i] = bounds[1] + 0
        ends[i] = bounds[2] + 0
      }
    }

    function current_query_name(line, cleaned, parts) {
      cleaned = line
      sub(/^--[[:space:]]*name:[[:space:]]*/, "", cleaned)
      split(cleaned, parts, /[[:space:]]+/)
      return parts[1]
    }

    function is_substantive_line(line, trimmed) {
      trimmed = line
      sub(/^[[:space:]]+/, "", trimmed)
      sub(/[[:space:]]+$/, "", trimmed)

      if (trimmed == "") {
        return 0
      }

      if (trimmed ~ /^--[[:space:]]*name:[[:space:]]*/) {
        return 1
      }

      if (trimmed ~ /^--/) {
        return 0
      }

      return 1
    }

    /^--[[:space:]]*name:[[:space:]]*/ {
      current = current_query_name($0)
    }

    {
      if (current == "") {
        next
      }

      for (i = 1; i <= range_count; i++) {
        if (NR >= starts[i] && NR <= ends[i] && is_substantive_line($0)) {
          seen[current] = 1
        }
      }
    }

    END {
      for (name in seen) {
        print name
      }
    }
  '
}

query_block_for_name() {
  local file="$1"
  local query_name="$2"

  awk -v query_name="$query_name" '
    function current_query_name(line, cleaned, parts) {
      cleaned = line
      sub(/^--[[:space:]]*name:[[:space:]]*/, "", cleaned)
      split(cleaned, parts, /[[:space:]]+/)
      return parts[1]
    }

    /^--[[:space:]]*name:[[:space:]]*/ {
      next_name = current_query_name($0)
      if (capturing && next_name != query_name) {
        exit
      }
      capturing = (next_name == query_name)
    }

    capturing {
      print
    }
  ' "$repo_root/$file"
}

has_sqlguard_justification() {
  local block="$1"
  local token="$2"
  local escaped_token

  escaped_token="$(printf '%s' "$token" | sed 's/[][(){}.^$*+?|\\/]/\\&/g')"

  printf '%s\n' "$block" | grep -Eq -- "^[[:space:]]*--.*${escaped_token}([[:space:]]+[^[:space:]].*)$"
}

  find_unscoped_write_statements() {
    local block="$1"
    local normalized statement

    normalized="$(printf '%s\n' "$block" | sed '/^[[:space:]]*--/d' | tr '[:upper:]' '[:lower:]' | tr '\n\t' '  ' | tr -s ' ')"

    while IFS=';' read -r statement; do
      statement="$(printf ' %s ' "$statement" | tr -s ' ')"
      [[ "$statement" =~ [^[:space:]] ]] || continue

      if [[ "$statement" =~ [[:space:]]update[[:space:]] ]] && [[ ! "$statement" =~ [[:space:]]where[[:space:]] ]]; then
        printf 'update\n'
      fi

      if [[ "$statement" =~ [[:space:]]delete[[:space:]]+from[[:space:]] ]] && [[ ! "$statement" =~ [[:space:]]where[[:space:]] ]]; then
        printf 'delete\n'
      fi
    done <<< "$normalized"
  }

    find_implicit_insert_column_omissions() {
      local block="$1"
      local normalized statement

      normalized="$(printf '%s\n' "$block" | sed '/^[[:space:]]*--/d' | tr '[:upper:]' '[:lower:]' | tr '\n\t' '  ' | tr -s ' ')"

      while IFS=';' read -r statement; do
        statement="$(printf ' %s ' "$statement" | tr -s ' ')"
        [[ "$statement" =~ [^[:space:]] ]] || continue

        if printf '%s\n' "$statement" | grep -Eq '[[:space:]]insert[[:space:]]+into[[:space:]]+[^[:space:](;]+[[:space:]]+values[[:space:]]*\('; then
          printf 'implicit-insert-columns\n'
        fi
      done <<< "$normalized"
    }

violations=0

echo "Checking changed SQL query blocks against lightweight guardrails..."

for file in "${changed_files[@]}"; do
  [[ -f "$repo_root/$file" ]] || continue

  old_ranges="$(collect_ranges old "$file" | tr '\n' ' ')"
  new_ranges="$(collect_ranges new "$file" | tr '\n' ' ')"

  declare -A touched_queries=()

  while IFS= read -r query_name; do
    [[ -n "$query_name" ]] && touched_queries["$query_name"]=1
  done < <(query_names_for_ranges "$base_ref" "$file" "$old_ranges")

  while IFS= read -r query_name; do
    [[ -n "$query_name" ]] && touched_queries["$query_name"]=1
  done < <(query_names_for_ranges head "$file" "$new_ranges")

  if [[ ${#touched_queries[@]} -eq 0 ]]; then
    echo "  ℹ️ no named query block detected for changed lines: $file"
    unset touched_queries
    continue
  fi

  for query_name in "${!touched_queries[@]}"; do
    block="$(query_block_for_name "$file" "$query_name")"
    if [[ -z "$block" ]]; then
      continue
    fi

    header="$(printf '%s\n' "$block" | head -n 1)"
    normalized="$(printf '%s' "$block" | tr '[:upper:]' '[:lower:]' | tr '\n\t' '  ' | tr -s ' ')"

    allow_select_star=0
    if [[ "$normalized" == *"sqlguard: allow-select-star"* ]]; then
      if has_sqlguard_justification "$block" "sqlguard: allow-select-star"; then
        allow_select_star=1
      else
        echo "  ❌ $file :: $query_name uses bare 'sqlguard: allow-select-star' without a concrete justification"
        echo "     keep the allow comment on one line and explain why the default rule does not apply and why the exception is safe here"
        violations=$((violations + 1))
      fi
    fi

    if (( allow_select_star == 0 )) && [[ "$normalized" =~ select[[:space:]]+\* ]]; then
      echo "  ❌ $file :: $query_name uses SELECT * in a touched query block"
      echo "     add explicit columns or annotate with 'sqlguard: allow-select-star' when justified"
      violations=$((violations + 1))
    fi

    allow_unordered_limit=0
    if [[ "$normalized" == *"sqlguard: allow-unordered-limit"* ]]; then
      if has_sqlguard_justification "$block" "sqlguard: allow-unordered-limit"; then
        allow_unordered_limit=1
      else
        echo "  ❌ $file :: $query_name uses bare 'sqlguard: allow-unordered-limit' without a concrete justification"
        echo "     keep the allow comment on one line and explain why the default rule does not apply and why the exception is safe here"
        violations=$((violations + 1))
      fi
    fi

    if [[ "$header" == *":many"* ]] && [[ "$normalized" =~ (limit|offset) ]] && [[ ! "$normalized" =~ order[[:space:]]+by ]] && (( allow_unordered_limit == 0 )); then
      echo "  ❌ $file :: $query_name is a :many query with LIMIT/OFFSET but no ORDER BY"
      echo "     add ORDER BY or annotate with 'sqlguard: allow-unordered-limit' when the result is provably stable"
      violations=$((violations + 1))
    fi

    allow_unscoped_write=0
    if [[ "$normalized" == *"sqlguard: allow-unscoped-write"* ]]; then
      if has_sqlguard_justification "$block" "sqlguard: allow-unscoped-write"; then
        allow_unscoped_write=1
      else
        echo "  ❌ $file :: $query_name uses bare 'sqlguard: allow-unscoped-write' without a concrete justification"
        echo "     keep the allow comment on one line and explain why the default rule does not apply and why the exception is safe here"
        violations=$((violations + 1))
      fi
    fi

    if (( allow_unscoped_write == 0 )); then
      while IFS= read -r write_kind; do
        [[ -n "$write_kind" ]] || continue
        echo "  ❌ $file :: $query_name has a $write_kind statement without WHERE in a touched query block"
        echo "     add an explicit WHERE scope or annotate with 'sqlguard: allow-unscoped-write' when the full-table write is intentional"
        violations=$((violations + 1))
      done < <(find_unscoped_write_statements "$block")
    fi

    allow_implicit_insert=0
    if [[ "$normalized" == *"sqlguard: allow-implicit-insert-columns"* ]]; then
      if has_sqlguard_justification "$block" "sqlguard: allow-implicit-insert-columns"; then
        allow_implicit_insert=1
      else
        echo "  ❌ $file :: $query_name uses bare 'sqlguard: allow-implicit-insert-columns' without a concrete justification"
        echo "     keep the allow comment on one line and explain why the default rule does not apply and why the exception is safe here"
        violations=$((violations + 1))
      fi
    fi

    if (( allow_implicit_insert == 0 )); then
      while IFS= read -r insert_kind; do
        [[ -n "$insert_kind" ]] || continue
        echo "  ❌ $file :: $query_name uses INSERT without an explicit column list in a touched query block"
        echo "     add explicit column names or annotate with 'sqlguard: allow-implicit-insert-columns' when the omission is truly intentional"
        violations=$((violations + 1))
      done < <(find_implicit_insert_column_omissions "$block")
    fi
  done

  unset touched_queries
done

if (( violations > 0 )); then
  echo
  echo "SQL guardrail failed with $violations violation(s)."
  exit 1
fi

echo
echo "SQL guardrail passed. Generated sqlc file validation remains enforced by the generated-artifacts workflow regeneration diff check."