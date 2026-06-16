DB_URL ?= $(if $(strip $(DB_SOURCE)),$(strip $(DB_SOURCE)),postgresql:///locallife_test?host=/var/run/postgresql&sslmode=disable)

migrateup:
	$(HOME)/go/bin/migrate -path db/migration -database "$(DB_URL)" -verbose up

migrateup1:
	$(HOME)/go/bin/migrate -path db/migration -database "$(DB_URL)" -verbose up 1

migratedown:
	$(HOME)/go/bin/migrate -path db/migration -database "$(DB_URL)" -verbose down

migratedown1:
	$(HOME)/go/bin/migrate -path db/migration -database "$(DB_URL)" -verbose down 1

migratestatus:
	psql "$(DB_URL)" -c "SELECT version, dirty FROM schema_migrations;"

migrateforce:
	@test -n "$(version)" || (echo "usage: make migrateforce version=<migration_version>" && exit 1)
	$(HOME)/go/bin/migrate -path db/migration -database "$(DB_URL)" force $(version)

new_migration:
	$(HOME)/go/bin/migrate create -ext sql -dir db/migration -seq $(name)

#db_docs:
#	dbdocs build doc/db.dbml

#db_schema:
#	dbml2sql --postgres -o doc/schema.sql doc/db.dbml

sqlc:
	$(HOME)/go/bin/sqlc generate
	$(HOME)/go/bin/mockgen -package mockdb -destination db/mock/store.go github.com/merrydance/locallife/db/sqlc Store
	$(HOME)/go/bin/mockgen -package mockwk -destination worker/mock/distributor.go github.com/merrydance/locallife/worker TaskDistributor
	$(HOME)/go/bin/mockgen -package mockwechat -destination wechat/mock/wechat_client.go github.com/merrydance/locallife/wechat WechatClient
	$(HOME)/go/bin/mockgen -package mockwechat -destination wechat/mock/payment_client.go github.com/merrydance/locallife/wechat DirectPaymentClientInterface,TransferClientInterface

test:
	go test -v -cover -short ./...

test-unit:
	go test -v -cover -short $$(go list ./... | grep -v '/integration$$')

test-integration:
	go test -v -cover -count=1 -p 1 ./integration

test-safety:
	./scripts/test_safety.sh

check-generated:
	./scripts/check_generated.sh

check-baofu-contract:
	./scripts/check_baofu_contract_drift.sh

check-baofu-provider-evidence-gate:
	PATH="/usr/local/go/bin:$(PATH)" bash ./scripts/test_baofu_provider_evidence_gate.sh

check-release-readiness-target-evidence:
	@test -n "$(evidence)" || (echo "usage: make check-release-readiness-target-evidence evidence=<evidence.md>" && exit 1)
	bash ./scripts/check_release_readiness_target_evidence.sh "$(evidence)"

doc-audit:
	go run ./cmd/doc_audit -report docs/phase0/user_journey_coverage_report.md

# Sync regions from Tencent LBS (Hebei + Guangzhou, up to level 3)
sync_regions_test:
	go run ./cmd/sync_regions_tencent -config . -db "$(DB_URL)"

swagger:
	$(HOME)/go/bin/swag init -g main.go -o docs --parseDependency --parseInternal

ocr-baseline:
	@test -n "$(manifest)" || (echo "usage: make ocr-baseline manifest=<manifest.json> report=<run.json> [out=<summary.json>]" && exit 1)
	@test -n "$(report)" || (echo "usage: make ocr-baseline manifest=<manifest.json> report=<run.json> [out=<summary.json>]" && exit 1)
	go run ./cmd/ocr_baseline_eval -manifest "$(manifest)" -run "$(report)" $(if $(out),-out "$(out)",)

server:
	go run main.go

mock:
	$(HOME)/go/bin/mockgen -package mockdb -destination db/mock/store.go github.com/merrydance/locallife/db/sqlc Store
	$(HOME)/go/bin/mockgen -package mockwk -destination worker/mock/distributor.go github.com/merrydance/locallife/worker TaskDistributor
	$(HOME)/go/bin/mockgen -package mockwechat -destination wechat/mock/wechat_client.go github.com/merrydance/locallife/wechat WechatClient
	$(HOME)/go/bin/mockgen -package mockwechat -destination wechat/mock/payment_client.go github.com/merrydance/locallife/wechat DirectPaymentClientInterface,TransferClientInterface

proto:
	rm -f pb/*.go
	rm -f doc/swagger/*.swagger.json
	protoc --proto_path=proto --go_out=pb --go_opt=paths=source_relative \
	--go-grpc_out=pb --go-grpc_opt=paths=source_relative \
	--grpc-gateway_out=pb --grpc-gateway_opt=paths=source_relative \
	--openapiv2_out=doc/swagger --openapiv2_opt=allow_merge=true,merge_file_name=locallife \
	proto/*.proto
	statik -src=./doc/swagger -dest=./doc

evans:
	evans --host localhost --port 9090 -r repl

# Handler/Logic 文件行数检查（上限 500 行，排除自动生成和测试文件）
MAX_FILE_LINES ?= 500
lint-filesize:
	@echo "Checking Go files exceeding $(MAX_FILE_LINES) lines..."
	@violations=0; \
	for f in $$(find api logic worker -name '*.go' \
	  ! -name '*_test.go' ! -name 'mock_*.go' ! -name '*.sql.go' \
	  ! -path '*/mock/*' ! -path '*/sqlc/*.sql.go' 2>/dev/null); do \
	  lines=$$(wc -l < "$$f"); \
	  if [ "$$lines" -gt $(MAX_FILE_LINES) ]; then \
	    echo "  ⚠️  $$f: $$lines lines (limit $(MAX_FILE_LINES))"; \
	    violations=$$((violations + 1)); \
	  fi; \
	done; \
	if [ "$$violations" -gt 0 ]; then \
	  echo "\n❌ $$violations file(s) exceed $(MAX_FILE_LINES)-line limit"; \
	  exit 1; \
	else \
	  echo "✅ All files within $(MAX_FILE_LINES)-line limit"; \
	fi

.PHONY: migrateup migratedown migrateup1 migratedown1 migrateforce migratestatus new_migration sqlc swagger ocr-baseline test test-unit test-integration test-safety check-generated check-baofu-contract check-baofu-provider-evidence-gate check-release-readiness-target-evidence server mock proto evans sync_regions_test doc-audit lint-filesize
