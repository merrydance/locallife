package db

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

var testStore Store

func TestMain(m *testing.M) {
	// 使用测试数据库连接
	testDBSource := os.Getenv("TEST_DB_SOURCE")
	if testDBSource == "" {
		// 显式走 unix socket，避免某些驱动（例如 pq）将空 host 解析为 TCP/localhost 导致密码认证失败。
		// 使用 /var/run/postgresql 是大多数 Linux 发行版的默认 socket 路径。
		// 显式走 unix socket，避免某些驱动（例如 pq）将空 host 解析为 TCP/localhost 导致密码认证失败。
		// 使用 /var/run/postgresql 是大多数 Linux 发行版的默认 socket 路径。
		testDBSource = "postgresql:///locallife_dev?sslmode=disable&host=/var/run/postgresql"
	}

	// [SAFEGUARD] 既然使用 locallife_dev，禁用自动 migration 和 Drop 操作，防止误删数据。
	// 我们假设 dev 数据库结构已经是新的。
	/*
		migrationURL := os.Getenv("TEST_MIGRATION_URL")
		if migrationURL == "" {
			// 当前文件位于 db/sqlc 下，migration 目录在 db/migration
			migrationURL = "file://../migration"
		}

		mig, err := migrate.New(migrationURL, testDBSource)
		if err != nil {
			log.Fatal("cannot create migrate instance:", err)
		}
		// ... (省略原有危险的 Drop/Up 逻辑) ...
		if _, err := mig.Close(); err != nil {
			log.Printf("warning: cannot close migrate instance: %v", err)
		}
	*/

	connPool, err := pgxpool.New(context.Background(), testDBSource)
	if err != nil {
		log.Fatal("cannot connect to test db:", err)
	}

	testStore = NewStore(connPool)
	os.Exit(m.Run())
}
