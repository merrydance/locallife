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
	"github.com/merrydance/locallife/internal/baofuevidence"
	"github.com/merrydance/locallife/util"
)

func main() {
	root := flag.String("root", ".", "backend repository root containing app.env")
	dbSource := flag.String("db", "", "database connection string; defaults to DB_SOURCE from config")
	factID := flag.Int64("fact-id", 0, "external_payment_facts id produced by Baofu payment callback/query")
	applicationID := flag.Int64("application-id", 0, "external_payment_fact_applications id for the fact")
	paymentOrderID := flag.Int64("payment-order-id", 0, "payment_orders id for the LocalLife order payment")
	profitSharingOrderID := flag.Int64("profit-sharing-order-id", 0, "optional profit_sharing_orders id for the payment")
	flag.Parse()

	if *factID <= 0 || *applicationID <= 0 || *paymentOrderID <= 0 {
		fmt.Fprintln(os.Stderr, "fact-id, application-id, and payment-order-id are required")
		os.Exit(2)
	}

	config, err := util.LoadConfig(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load config:", err)
		os.Exit(2)
	}
	source := strings.TrimSpace(*dbSource)
	if source == "" {
		source = config.DBSource
	}
	if source == "" {
		fmt.Fprintln(os.Stderr, "DB_SOURCE is required")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, source)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect db:", err)
		os.Exit(2)
	}
	defer pool.Close()

	queries := db.New(pool)
	summary, err := loadAggregatePaymentEvidence(ctx, queries, *factID, *applicationID, *paymentOrderID, *profitSharingOrderID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load baofu payment evidence:", err)
		os.Exit(2)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(summary); err != nil {
		fmt.Fprintln(os.Stderr, "encode evidence:", err)
		os.Exit(2)
	}
	if summary.Status != baofuevidence.StatusPass {
		os.Exit(1)
	}
}

type aggregatePaymentEvidenceReader interface {
	GetExternalPaymentFact(ctx context.Context, id int64) (db.ExternalPaymentFact, error)
	GetExternalPaymentFactApplication(ctx context.Context, id int64) (db.ExternalPaymentFactApplication, error)
	GetPaymentOrder(ctx context.Context, id int64) (db.PaymentOrder, error)
	GetProfitSharingOrder(ctx context.Context, id int64) (db.ProfitSharingOrder, error)
}

func loadAggregatePaymentEvidence(ctx context.Context, reader aggregatePaymentEvidenceReader, factID, applicationID, paymentOrderID, profitSharingOrderID int64) (baofuevidence.AggregatePaymentSummary, error) {
	fact, err := reader.GetExternalPaymentFact(ctx, factID)
	if err != nil {
		return baofuevidence.AggregatePaymentSummary{}, fmt.Errorf("get external payment fact: %w", err)
	}
	application, err := reader.GetExternalPaymentFactApplication(ctx, applicationID)
	if err != nil {
		return baofuevidence.AggregatePaymentSummary{}, fmt.Errorf("get external payment fact application: %w", err)
	}
	paymentOrder, err := reader.GetPaymentOrder(ctx, paymentOrderID)
	if err != nil {
		return baofuevidence.AggregatePaymentSummary{}, fmt.Errorf("get payment order: %w", err)
	}

	input := baofuevidence.AggregatePaymentInput{
		Fact:         fact,
		Application:  application,
		PaymentOrder: paymentOrder,
	}
	if profitSharingOrderID > 0 {
		profitSharingOrder, err := reader.GetProfitSharingOrder(ctx, profitSharingOrderID)
		if err != nil {
			return baofuevidence.AggregatePaymentSummary{}, fmt.Errorf("get profit sharing order: %w", err)
		}
		input.ProfitSharingOrder = &profitSharingOrder
	}

	return baofuevidence.BuildAggregatePaymentEvidence(input), nil
}
