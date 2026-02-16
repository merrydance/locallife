package db

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

var testStore Store

func TestMain(m *testing.M) {
	// 使用测试数据库连接
	testDBSource := os.Getenv("TEST_DB_SOURCE")
	if testDBSource == "" {
		testDBSource = os.Getenv("DB_SOURCE")
	}
	if testDBSource == "" {
		// 显式走 unix socket，避免某些驱动（例如 pq）将空 host 解析为 TCP/localhost 导致密码认证失败。
		// 使用 /var/run/postgresql 是大多数 Linux 发行版的默认 socket 路径。
		testDBSource = "postgresql:///locallife_test?sslmode=disable&host=/var/run/postgresql"
	}

	// [SAFEGUARD] 只执行 Up（不做 Drop/Down），避免破坏性操作。
	migrationURL := os.Getenv("TEST_MIGRATION_URL")
	if migrationURL == "" {
		// 当前文件位于 db/sqlc 下，migration 目录在 db/migration
		migrationURL = "file://../migration"
	}

	mig, err := migrate.New(migrationURL, testDBSource)
	if err != nil {
		log.Fatal("cannot create migrate instance:", err)
	}
	if err := mig.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatal("migration failed:", err)
	}
	if _, err := mig.Close(); err != nil {
		log.Printf("warning: cannot close migrate instance: %v", err)
	}

	connPool, err := pgxpool.New(context.Background(), testDBSource)
	if err != nil {
		log.Fatal("cannot connect to test db:", err)
	}

	testStore = NewStore(connPool)
	os.Exit(m.Run())
}
