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
		// 显式走 unix socket，避免某些驱动（例如 pq）将空 host 解析为 TCP/localhost 导致密码认证失败。
		// 使用 /var/run/postgresql 是大多数 Linux 发行版的默认 socket 路径。
		testDBSource = "postgresql:///locallife_test?sslmode=disable&host=/var/run/postgresql"
	}

	// 运行 migrations，避免“数据库未初始化但测试仍然侥幸通过/失败不稳定”。
	// 允许通过 TEST_MIGRATION_URL 自定义，默认指向 db/migration。
	migrationURL := os.Getenv("TEST_MIGRATION_URL")
	if migrationURL == "" {
		// 当前文件位于 db/sqlc 下，migration 目录在 db/migration
		migrationURL = "file://../migration"
	}

	mig, err := migrate.New(migrationURL, testDBSource)
	if err != nil {
		log.Fatal("cannot create migrate instance:", err)
	}
	// 如果测试库处于 dirty 状态（通常是上次手动/CI 中断导致），强制清理 dirty 标记，
	// 让后续 Up 能够继续并使测试可重复。
	if v, dirty, err := mig.Version(); err == nil {
		if dirty {
			log.Printf("test db is dirty at version %d; forcing version and retrying migrations", v)
			if err := mig.Force(int(v)); err != nil {
				log.Fatal("cannot force migration version:", err)
			}
		}
	}
	if err := mig.Up(); err != nil && err != migrate.ErrNoChange {
		// 测试库经常被反复使用，可能出现 schema 已存在但 schema_migrations 版本不一致的情况。
		// 为保证测试可重复，遇到 migration 失败时，尝试 Drop 后再 Up 一次。
		log.Printf("migrations failed (%v); attempting drop and re-run", err)
		if dropErr := mig.Drop(); dropErr != nil {
			log.Fatal("cannot drop test db schema for migrations:", dropErr)
		}
		// Drop() 可能会删除 schema_migrations 表；复用旧实例有时会触发 TRUNCATE 不存在表。
		// 重新创建 migrate 实例，再执行 Up。
		if _, closeErr := mig.Close(); closeErr != nil {
			log.Printf("warning: cannot close migrate instance after drop: %v", closeErr)
		}
		mig2, newErr := migrate.New(migrationURL, testDBSource)
		if newErr != nil {
			log.Fatal("cannot recreate migrate instance after drop:", newErr)
		}
		if upErr := mig2.Up(); upErr != nil && upErr != migrate.ErrNoChange {
			log.Fatal("cannot run migrations after drop:", upErr)
		}
		if _, closeErr := mig2.Close(); closeErr != nil {
			log.Printf("warning: cannot close migrate instance: %v", closeErr)
		}
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
