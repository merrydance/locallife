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
	factID := flag.Int64("fact-id", 0, "external_payment_facts id produced by Baofu refund callback/query")
	applicationID := flag.Int64("application-id", 0, "external_payment_fact_applications id for the fact")
	refundOrderID := flag.Int64("refund-order-id", 0, "refund_orders id for the Baofu refund")
	paymentOrderID := flag.Int64("payment-order-id", 0, "payment_orders id for the refunded payment")
	commandID := flag.Int64("command-id", 0, "optional external_payment_commands id for create_refund")
	ledgerRow := flag.Bool("ledger-row", false, "include a SANDBOX_EVIDENCE_LEDGER.md row candidate in the JSON output")
	ledgerDate := flag.String("ledger-date", "", "evidence row date, required with -ledger-row")
	ledgerEnv := flag.String("ledger-env", "", "evidence environment, required with -ledger-row")
	ledgerEndpoint := flag.String("ledger-endpoint", "", "evidence endpoint or callback URL, required with -ledger-row")
	ledgerACK := flag.String("ledger-ack", "", "callback ACK observed for callback evidence")
	ledgerCommit := flag.String("ledger-commit", "", "commit SHA for the evidence row, required with -ledger-row")
	ledgerNotes := flag.String("ledger-notes", "", "operator notes for the evidence row, required with -ledger-row")
	flag.Parse()

	if *factID <= 0 || *applicationID <= 0 || *refundOrderID <= 0 || *paymentOrderID <= 0 {
		fmt.Fprintln(os.Stderr, "fact-id, application-id, refund-order-id, and payment-order-id are required")
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
	summary, err := loadRefundEvidence(ctx, queries, *factID, *applicationID, *refundOrderID, *paymentOrderID, *commandID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load baofu refund evidence:", err)
		os.Exit(2)
	}

	output, exitCode, err := renderCommandOutput(summary, commandOutputOptions{
		LedgerRow: *ledgerRow,
		LedgerContext: baofuevidence.EvidenceLedgerRowContext{
			Date:     *ledgerDate,
			Env:      *ledgerEnv,
			Endpoint: *ledgerEndpoint,
			ACK:      *ledgerACK,
			Commit:   *ledgerCommit,
			Notes:    *ledgerNotes,
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "encode evidence:", err)
		os.Exit(2)
	}
	fmt.Print(output)
	os.Exit(exitCode)
}

type commandOutputOptions struct {
	LedgerRow     bool
	LedgerContext baofuevidence.EvidenceLedgerRowContext
}

type commandOutput struct {
	Summary   baofuevidence.RefundSummary      `json:"summary"`
	LedgerRow *baofuevidence.EvidenceLedgerRow `json:"ledger_row,omitempty"`
}

func renderCommandOutput(summary baofuevidence.RefundSummary, options commandOutputOptions) (string, int, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", "  ")
	if !options.LedgerRow {
		if err := encoder.Encode(summary); err != nil {
			return "", 0, err
		}
		return buffer.String(), evidenceExitCode(summary), nil
	}

	row, err := baofuevidence.RenderRefundLedgerRow(summary, options.LedgerContext)
	if err != nil {
		return "", 0, err
	}
	if err := encoder.Encode(commandOutput{
		Summary:   summary,
		LedgerRow: &row,
	}); err != nil {
		return "", 0, err
	}
	return buffer.String(), evidenceExitCode(summary), nil
}

func evidenceExitCode(summary baofuevidence.RefundSummary) int {
	if summary.Status != baofuevidence.StatusPass {
		return 1
	}
	return 0
}

type refundEvidenceReader interface {
	GetExternalPaymentFact(ctx context.Context, id int64) (db.ExternalPaymentFact, error)
	GetExternalPaymentFactApplication(ctx context.Context, id int64) (db.ExternalPaymentFactApplication, error)
	GetRefundOrder(ctx context.Context, id int64) (db.RefundOrder, error)
	GetPaymentOrder(ctx context.Context, id int64) (db.PaymentOrder, error)
	GetExternalPaymentCommand(ctx context.Context, id int64) (db.ExternalPaymentCommand, error)
	GetExternalPaymentCommandByExternalObject(ctx context.Context, arg db.GetExternalPaymentCommandByExternalObjectParams) (db.ExternalPaymentCommand, error)
}

func loadRefundEvidence(ctx context.Context, reader refundEvidenceReader, factID, applicationID, refundOrderID, paymentOrderID, commandID int64) (baofuevidence.RefundSummary, error) {
	fact, err := reader.GetExternalPaymentFact(ctx, factID)
	if err != nil {
		return baofuevidence.RefundSummary{}, fmt.Errorf("get external payment fact: %w", err)
	}
	application, err := reader.GetExternalPaymentFactApplication(ctx, applicationID)
	if err != nil {
		return baofuevidence.RefundSummary{}, fmt.Errorf("get external payment fact application: %w", err)
	}
	refundOrder, err := reader.GetRefundOrder(ctx, refundOrderID)
	if err != nil {
		return baofuevidence.RefundSummary{}, fmt.Errorf("get refund order: %w", err)
	}
	paymentOrder, err := reader.GetPaymentOrder(ctx, paymentOrderID)
	if err != nil {
		return baofuevidence.RefundSummary{}, fmt.Errorf("get payment order: %w", err)
	}

	input := baofuevidence.RefundInput{
		Fact:         fact,
		Application:  application,
		RefundOrder:  refundOrder,
		PaymentOrder: paymentOrder,
	}
	command, err := loadRefundCommand(ctx, reader, refundOrder, commandID)
	if err != nil {
		return baofuevidence.RefundSummary{}, err
	}
	input.Command = command

	return baofuevidence.BuildRefundEvidence(input), nil
}

func loadRefundCommand(ctx context.Context, reader refundEvidenceReader, refundOrder db.RefundOrder, commandID int64) (*db.ExternalPaymentCommand, error) {
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
		Capability:         db.ExternalPaymentCapabilityBaofuRefund,
		CommandType:        db.ExternalPaymentCommandTypeCreateRefund,
		ExternalObjectType: db.ExternalPaymentObjectRefund,
		ExternalObjectKey:  refundOrder.OutRefundNo,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get external payment command by external object: %w", err)
	}
	return &command, nil
}
