#!/usr/bin/env bash

set -euo pipefail

base_ref="${1:-}"
head_ref="${2:-HEAD}"
repo_root="$(git rev-parse --show-toplevel)"

if [[ -z "$base_ref" || "$base_ref" == "0000000000000000000000000000000000000000" ]]; then
  echo "No usable base ref provided; skipping Go guardrail."
  exit 0
fi

mapfile -t changed_files < <(
  git -C "$repo_root" diff --name-only --diff-filter=ACMR "$base_ref" "$head_ref" | \
    grep -E '^locallife/.*\.go$' | \
    grep -Ev '(_test\.go$|\.sql\.go$|/mock/)' || true
)

if [[ ${#changed_files[@]} -eq 0 ]]; then
  echo "No changed non-test Go files matched the Go guardrail."
  exit 0
fi

violations=0

echo "Checking changed Go lines against lightweight guardrails..."

for file in "${changed_files[@]}"; do
  while IFS= read -r diff_line; do
    [[ "$diff_line" =~ ^\+[^+] ]] || continue

    line="${diff_line:1}"
    trimmed="$(printf '%s' "$line" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')"

    [[ -n "$trimmed" ]] || continue

    if [[ "$trimmed" =~ ^// ]]; then
      continue
    fi

    if [[ "$trimmed" =~ fmt\.(Print|Printf|Println)\( ]] || [[ "$trimmed" =~ (^|[^[:alnum:]_])log\.(Print|Printf|Println)\( ]]; then
      if [[ "$trimmed" == *"goguard: allow-unstructured-log"* ]]; then
        if [[ ! "$trimmed" =~ goguard:\ allow-unstructured-log[[:space:]]+[^[:space:]] ]]; then
          echo "  ❌ $file uses bare 'goguard: allow-unstructured-log' without a concrete same-line justification"
          echo "     explain why the default rule does not apply and why the exception is safe on this exact line"
          violations=$((violations + 1))
        fi
      else
        echo "  ❌ $file adds unstructured logging: $trimmed"
        echo "     use structured logging or annotate inline with 'goguard: allow-unstructured-log' when truly justified"
        violations=$((violations + 1))
      fi
    fi

    if [[ "$trimmed" =~ context\.Background\(\) ]]; then
      if [[ "$trimmed" == *"goguard: allow-background-context"* ]]; then
        if [[ ! "$trimmed" =~ goguard:\ allow-background-context[[:space:]]+[^[:space:]] ]]; then
          echo "  ❌ $file uses bare 'goguard: allow-background-context' without a concrete same-line justification"
          echo "     explain why the default rule does not apply and why the exception is safe on this exact line"
          violations=$((violations + 1))
        fi
      else
        echo "  ❌ $file adds context.Background(): $trimmed"
        echo "     thread the upstream ctx through the call chain or annotate inline with 'goguard: allow-background-context' for true process-entry or detached work"
        violations=$((violations + 1))
      fi
    fi

    if [[ "$trimmed" =~ (^|[^[:alnum:]_])panic\( ]]; then
      if [[ "$trimmed" == *"goguard: allow-panic"* ]]; then
        if [[ ! "$trimmed" =~ goguard:\ allow-panic[[:space:]]+[^[:space:]] ]]; then
          echo "  ❌ $file uses bare 'goguard: allow-panic' without a concrete same-line justification"
          echo "     explain why the default rule does not apply and why the exception is safe on this exact line"
          violations=$((violations + 1))
        fi
      else
        echo "  ❌ $file adds panic(...): $trimmed"
        echo "     return an explicit error or annotate inline with 'goguard: allow-panic' when the panic is truly process-entry or invariant-failure only"
        violations=$((violations + 1))
      fi
    fi

    if [[ "$trimmed" =~ ^_[[:space:]]*=[[:space:]]*(parseJSON|json\.Unmarshal)\( ]]; then
      echo "  ❌ $file ignores JSON decode errors: $trimmed"
      echo "     handle the decode error explicitly or document a narrow, contract-backed downgrade instead of discarding it"
      violations=$((violations + 1))
    fi

    if [[ "$file" == locallife/logic/* ]] && [[ "$trimmed" =~ NewRequestError\( ]] && [[ "$trimmed" =~ (http\.StatusInternalServerError|http\.StatusBadGateway|http\.StatusServiceUnavailable|http\.StatusGatewayTimeout|500|502|503|504) ]]; then
      echo "  ❌ $file adds NewRequestError(...) with a 5xx status: $trimmed"
      echo "     return a wrapped plain error so the handler can log via internalError(...) or the repo's logged server-error helper"
      violations=$((violations + 1))
    fi

    if [[ "$file" == locallife/api/* ]] && [[ "$trimmed" =~ ctx\.JSON\( ]] && [[ "$trimmed" =~ errorResponse\( ]] && [[ "$trimmed" =~ (http\.StatusInternalServerError|http\.StatusBadGateway|http\.StatusServiceUnavailable|500|502|503) ]]; then
      echo "  ❌ $file adds ctx.JSON(..., errorResponse(...)) for a 5xx status: $trimmed"
      echo "     use internalError(ctx, err) for 500, or loggedServerError(...) for 502/503 with a stable public message"
      violations=$((violations + 1))
    fi
  done < <(git -C "$repo_root" diff --unified=0 "$base_ref" "$head_ref" -- "$file")
done

if (( violations > 0 )); then
  echo
  echo "Go guardrail failed with $violations violation(s)."
  exit 1
fi

echo

echo "Go guardrail passed."

