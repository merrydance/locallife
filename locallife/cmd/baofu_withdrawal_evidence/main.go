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
	factID := flag.Int64("fact-id", 0, "external_payment_facts id produced by Baofu withdrawal callback/query")
	withdrawalOrderID := flag.Int64("withdrawal-order-id", 0, "baofu_withdrawal_orders id for the Baofu withdrawal")
	commandID := flag.Int64("command-id", 0, "optional external_payment_commands id for create_baofu_withdraw")
	flag.Parse()

	if *factID <= 0 || *withdrawalOrderID <= 0 {
		fmt.Fprintln(os.Stderr, "fact-id and withdrawal-order-id are required")
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
	summary, err := loadWithdrawalEvidence(ctx, queries, *factID, *withdrawalOrderID, *commandID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load baofu withdrawal evidence:", err)
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

func renderCommandOutput(summary baofuevidence.WithdrawalSummary) (string, int, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(summary); err != nil {
		return "", 0, err
	}
	return buffer.String(), evidenceExitCode(summary), nil
}

func evidenceExitCode(summary baofuevidence.WithdrawalSummary) int {
	if summary.Status != baofuevidence.StatusPass {
		return 1
	}
	return 0
}

type withdrawalEvidenceReader interface {
	GetExternalPaymentFact(ctx context.Context, id int64) (db.ExternalPaymentFact, error)
	GetBaofuWithdrawalOrder(ctx context.Context, id int64) (db.BaofuWithdrawalOrder, error)
	GetExternalPaymentCommand(ctx context.Context, id int64) (db.ExternalPaymentCommand, error)
	GetExternalPaymentCommandByExternalObject(ctx context.Context, arg db.GetExternalPaymentCommandByExternalObjectParams) (db.ExternalPaymentCommand, error)
}

func loadWithdrawalEvidence(ctx context.Context, reader withdrawalEvidenceReader, factID, withdrawalOrderID, commandID int64) (baofuevidence.WithdrawalSummary, error) {
	fact, err := reader.GetExternalPaymentFact(ctx, factID)
	if err != nil {
		return baofuevidence.WithdrawalSummary{}, fmt.Errorf("get external payment fact: %w", err)
	}
	withdrawalOrder, err := reader.GetBaofuWithdrawalOrder(ctx, withdrawalOrderID)
	if err != nil {
		return baofuevidence.WithdrawalSummary{}, fmt.Errorf("get baofu withdrawal order: %w", err)
	}

	input := baofuevidence.WithdrawalInput{
		Fact:  fact,
		Order: withdrawalOrder,
	}
	command, err := loadWithdrawalCommand(ctx, reader, withdrawalOrder, commandID)
	if err != nil {
		return baofuevidence.WithdrawalSummary{}, err
	}
	input.Command = command

	return baofuevidence.BuildWithdrawalEvidence(input), nil
}

func loadWithdrawalCommand(ctx context.Context, reader withdrawalEvidenceReader, withdrawalOrder db.BaofuWithdrawalOrder, commandID int64) (*db.ExternalPaymentCommand, error) {
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
		Capability:         db.ExternalPaymentCapabilityBaofuWithdraw,
		CommandType:        db.ExternalPaymentCommandTypeCreateBaofuWithdraw,
		ExternalObjectType: db.ExternalPaymentObjectWithdraw,
		ExternalObjectKey:  withdrawalOrder.OutRequestNo,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get external payment command by external object: %w", err)
	}
	return &command, nil
}
