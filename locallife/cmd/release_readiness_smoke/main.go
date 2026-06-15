package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/internal/releasereadiness"
	"github.com/merrydance/locallife/util"
)

func main() {
	root := flag.String("root", ".", "backend repository root to scan")
	format := flag.String("format", "text", "output format: text or json")
	includeConfig := flag.Bool("include-config", false, "also load config from root and check production fail-fast readiness")
	includeRedis := flag.Bool("include-redis", false, "also ping Redis and read Asynq queue stats using loaded config")
	includeProviderClients := flag.Bool("include-provider-clients", false, "also construct provider clients using loaded config without making provider requests")
	includeFixtureClaimability := flag.Bool("include-fixture-claimability", false, "also claim explicit disposable DB fixture rows inside a rollback-only transaction")
	paymentFactApplicationFixtureID := flag.Int64("payment-fact-application-fixture-id", 0, "external_payment_fact_applications id to claim inside rollback-only fixture mode")
	paymentDomainOutboxFixtureID := flag.Int64("payment-domain-outbox-fixture-id", 0, "payment_domain_outbox id to claim inside rollback-only fixture mode")
	flag.Parse()

	var config util.Config
	report, err := releasereadiness.Check(releasereadiness.Options{
		Root:         *root,
		Expectations: releasereadiness.DefaultExpectations(),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "release readiness smoke failed:", err)
		os.Exit(2)
	}
	if err := validateFixtureClaimabilityFlagIDs(*includeFixtureClaimability, *paymentFactApplicationFixtureID, *paymentDomainOutboxFixtureID); err != nil {
		fmt.Fprintln(os.Stderr, "release readiness fixture claimability failed:", err)
		os.Exit(2)
	}
	if *includeConfig || *includeRedis || *includeProviderClients || *includeFixtureClaimability {
		config, err = util.LoadConfig(*root)
		if err != nil {
			fmt.Fprintln(os.Stderr, "release readiness config load failed:", err)
			os.Exit(2)
		}
	}
	if *includeConfig {
		report = releasereadiness.MergeReports(report, releasereadiness.CheckConfig(config))
	}
	if *includeRedis {
		report = releasereadiness.MergeReports(report, releasereadiness.CheckRedisAsynq(releasereadiness.RedisAsynqOptions{
			Address:        config.RedisAddress,
			Password:       config.RedisPassword,
			RequiredQueues: []string{"critical", "default"},
		}))
	}
	if *includeProviderClients {
		report = releasereadiness.MergeReports(report, releasereadiness.CheckBaofuProviderClients(config))
	}
	if *includeFixtureClaimability {
		fixtureReport, err := checkFixtureClaimabilityInRollbackTx(config, releasereadiness.FixtureClaimabilityOptions{
			PaymentFactApplicationID: *paymentFactApplicationFixtureID,
			PaymentDomainOutboxID:    *paymentDomainOutboxFixtureID,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "release readiness fixture claimability failed:", err)
			os.Exit(2)
		}
		report = releasereadiness.MergeReports(report, fixtureReport)
	}

	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			fmt.Fprintln(os.Stderr, "encode release readiness report:", err)
			os.Exit(2)
		}
	case "text", "":
		var sb strings.Builder
		releasereadiness.WriteText(report, &sb)
		fmt.Print(sb.String())
	default:
		fmt.Fprintln(os.Stderr, "unsupported format:", *format)
		os.Exit(2)
	}

	if report.Status != releasereadiness.StatusPass {
		os.Exit(1)
	}
}

func validateFixtureClaimabilityFlagIDs(includeFixtureClaimability bool, paymentFactApplicationFixtureID, paymentDomainOutboxFixtureID int64) error {
	if !includeFixtureClaimability {
		return nil
	}
	if paymentFactApplicationFixtureID <= 0 {
		return fmt.Errorf("payment-fact-application-fixture-id must be a positive integer")
	}
	if paymentDomainOutboxFixtureID <= 0 {
		return fmt.Errorf("payment-domain-outbox-fixture-id must be a positive integer")
	}
	return nil
}

func checkFixtureClaimabilityInRollbackTx(config util.Config, opts releasereadiness.FixtureClaimabilityOptions) (releasereadiness.Report, error) {
	if strings.TrimSpace(config.DBSource) == "" {
		return releasereadiness.Report{}, fmt.Errorf("DB_SOURCE is required for fixture claimability readiness")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, config.DBSource)
	if err != nil {
		return releasereadiness.Report{}, fmt.Errorf("connect db: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return releasereadiness.Report{}, fmt.Errorf("ping db: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return releasereadiness.Report{}, fmt.Errorf("begin rollback-only fixture tx: %w", err)
	}
	queries := db.New(tx)
	report := releasereadiness.CheckFixtureClaimability(ctx, queries, opts)
	if err := tx.Rollback(ctx); err != nil {
		return releasereadiness.Report{}, fmt.Errorf("rollback fixture tx: %w", err)
	}
	return report, nil
}
