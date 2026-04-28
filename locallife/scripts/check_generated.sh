#!/usr/bin/env bash
set -euo pipefail

export PATH="$HOME/go/bin:$PATH"

required_tools=(
  "$HOME/go/bin/sqlc"
  "$HOME/go/bin/mockgen"
  "$HOME/go/bin/swag"
)

for tool in "${required_tools[@]}"; do
  if [ ! -x "$tool" ]; then
    echo "missing required tool: $tool" >&2
    exit 1
  fi
done

if ! command -v go >/dev/null 2>&1; then
	echo "missing required tool: go" >&2
	exit 1
fi

generated_paths=(
  db/sqlc
  db/mock
  worker/mock
  wechat/mock
  docs/docs.go
  docs/swagger.json
  docs/swagger.yaml
)

before_status="$(git status --porcelain -- "${generated_paths[@]}" || true)"

make sqlc
make swagger

after_status="$(git status --porcelain -- "${generated_paths[@]}" || true)"

if [ "$before_status" != "$after_status" ]; then
  echo "generated artifacts drifted after regeneration" >&2
  git diff -- "${generated_paths[@]}" >&2 || true
  exit 1
fi

echo "generated artifacts are in sync"
