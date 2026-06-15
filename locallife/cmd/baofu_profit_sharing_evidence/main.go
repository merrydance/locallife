package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/internal/baofuevidence"
	"github.com/merrydance/locallife/util"
)

func main() {
	root := flag.String("root", ".", "backend repository root containing app.env")
	dbSource := flag.String("db", "", "database connection string; defaults to DB_SOURCE from config")
	factID := flag.Int64("fact-id", 0, "external_payment_facts id produced by Baofu profit-sharing callback/query")
	applicationID := flag.Int64("application-id", 0, "external_payment_fact_applications id for the fact")
	profitSharingOrderID := flag.Int64("profit-sharing-order-id", 0, "profit_sharing_orders id for the Baofu share order")
	commandID := flag.Int64("command-id", 0, "optional external_payment_commands id for create_profit_sharing")
	flag.Parse()

	if *factID <= 0 || *applicationID <= 0 || *profitSharingOrderID <= 0 {
		fmt.Fprintln(os.Stderr, "fact-id, application-id, and profit-sharing-order-id are required")
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
	summary, err := loadProfitSharingEvidence(ctx, queries, *factID, *applicationID, *profitSharingOrderID, *commandID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load baofu profit sharing evidence:", err)
		os.Exit(2)
	}

	output, exitCode, err := renderCommandOutput(summary)
	if err != nil {
		fmt.Fprintln(os.Stderr, "encode evidence:", err)
		os.Exit(2)
	}
	fmt.Print(output)
	os.Exit(exitCode)
}

func renderCommandOutput(summary baofuevidence.ProfitSharingSummary) (string, int, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(summary); err != nil {
		return "", 0, err
	}
	return buffer.String(), evidenceExitCode(summary), nil
}

func evidenceExitCode(summary baofuevidence.ProfitSharingSummary) int {
	if summary.Status != baofuevidence.StatusPass {
		return 1
	}
	return 0
}

type profitSharingEvidenceReader interface {
	GetExternalPaymentFact(ctx context.Context, id int64) (db.ExternalPaymentFact, error)
	GetExternalPaymentFactApplication(ctx context.Context, id int64) (db.ExternalPaymentFactApplication, error)
	GetProfitSharingOrder(ctx context.Context, id int64) (db.ProfitSharingOrder, error)
	GetExternalPaymentCommand(ctx context.Context, id int64) (db.ExternalPaymentCommand, error)
	GetExternalPaymentCommandByExternalObject(ctx context.Context, arg db.GetExternalPaymentCommandByExternalObjectParams) (db.ExternalPaymentCommand, error)
}

func loadProfitSharingEvidence(ctx context.Context, reader profitSharingEvidenceReader, factID, applicationID, profitSharingOrderID, commandID int64) (baofuevidence.ProfitSharingSummary, error) {
	fact, err := reader.GetExternalPaymentFact(ctx, factID)
	if err != nil {
		return baofuevidence.ProfitSharingSummary{}, fmt.Errorf("get external payment fact: %w", err)
	}
	application, err := reader.GetExternalPaymentFactApplication(ctx, applicationID)
	if err != nil {
		return baofuevidence.ProfitSharingSummary{}, fmt.Errorf("get external payment fact application: %w", err)
	}
	profitSharingOrder, err := reader.GetProfitSharingOrder(ctx, profitSharingOrderID)
	if err != nil {
		return baofuevidence.ProfitSharingSummary{}, fmt.Errorf("get profit sharing order: %w", err)
	}

	input := baofuevidence.ProfitSharingInput{
		Fact:        fact,
		Application: application,
		Order:       profitSharingOrder,
	}
	command, err := loadProfitSharingCommand(ctx, reader, profitSharingOrder, commandID)
	if err != nil {
		return baofuevidence.ProfitSharingSummary{}, err
	}
	input.Command = command

	return baofuevidence.BuildProfitSharingEvidence(input), nil
}

func loadProfitSharingCommand(ctx context.Context, reader profitSharingEvidenceReader, order db.ProfitSharingOrder, commandID int64) (*db.ExternalPaymentCommand, error) {
	if commandID > 0 {
		command, err := reader.GetExternalPaymentCommand(ctx, commandID)
		if err != nil {
			return nil, fmt.Errorf("get external payment command: %w", err)
		}
		return &command, nil
	}

	command, err := reader.GetExternalPaymentCommandByExternalObject(ctx, db.GetExternalPaymentCommandByExternalObjectParams{
		Provider:           db.ExternalPaymentProviderBaofu,
		Channel:            db.PaymentChannelBaofuAggregate,
		Capability:         db.ExternalPaymentCapabilityBaofuProfitSharing,
		CommandType:        db.ExternalPaymentCommandTypeCreateProfitSharing,
		ExternalObjectType: db.ExternalPaymentObjectProfitSharing,
		ExternalObjectKey:  order.OutOrderNo,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get external payment command by external object: %w", err)
	}
	return &command, nil
}
