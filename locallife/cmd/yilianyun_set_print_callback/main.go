package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/merrydance/locallife/cloudprint"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		configPath  = flag.String("config", ".", "config path containing app.env")
		dbURL       = flag.String("db", "", "database connection string (default: DB_SOURCE from config)")
		merchantID  = flag.Int64("merchant-id", 0, "LocalLife merchant id that owns the Yilianyun authorization")
		machineCode = flag.String("machine-code", "", "Yilianyun printer terminal number")
		callbackURL = flag.String("callback-url", "", "callback URL; defaults to YILIANYUN_PRINT_CALLBACK_URL")
		status      = flag.String("status", "open", "push status: open or close")
		dryRun      = flag.Bool("dry-run", false, "validate local inputs and authorization without calling Yilianyun")
	)
	flag.Parse()

	cfg, err := util.LoadConfig(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.ValidateCloudPrinterProviderConfig(); err != nil {
		return fmt.Errorf("validate cloud printer provider config: %w", err)
	}

	input, err := buildCommandInput(cfg, *dbURL, *merchantID, *machineCode, *callbackURL, *status)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, input.dbURL)
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	store := db.New(pool)
	authorization, err := store.GetCloudPrinterProviderAuthorizationByMerchantAndMachineCode(ctx, db.GetCloudPrinterProviderAuthorizationByMerchantAndMachineCodeParams{
		MerchantID:   input.merchantID,
		ProviderType: db.CloudPrinterProviderYilianyun,
		MachineCode:  input.machineCode,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return fmt.Errorf("yilianyun authorization not found for merchant_id=%d machine_code=%s", input.merchantID, mask(input.machineCode))
		}
		return fmt.Errorf("load yilianyun authorization: %w", err)
	}
	if authorization.Status != db.CloudPrinterAuthorizationStatusActive {
		return fmt.Errorf("yilianyun authorization is not active: status=%s", authorization.Status)
	}
	if !authorization.AccessTokenExpiresAt.After(time.Now().UTC()) {
		return fmt.Errorf("yilianyun access token is expired; refresh or re-authorize before registering callback")
	}

	var encryptor util.DataEncryptor
	if strings.TrimSpace(cfg.DataEncryptionKey) != "" {
		encryptor, err = util.NewAESEncryptor(cfg.DataEncryptionKey)
		if err != nil {
			return fmt.Errorf("create data encryptor: %w", err)
		}
	} else if cfg.Environment == "production" {
		return fmt.Errorf("DATA_ENCRYPTION_KEY is required in production")
	}
	accessToken, err := util.DecryptSensitiveField(encryptor, authorization.AccessTokenCiphertext)
	if err != nil {
		return fmt.Errorf("decrypt yilianyun access token: %w", err)
	}
	if strings.TrimSpace(accessToken) == "" {
		return fmt.Errorf("yilianyun access token is empty")
	}

	fmt.Printf("yilianyun print callback registration target merchant_id=%d machine_code=%s status=%s callback_url=%s dry_run=%v\n",
		input.merchantID,
		mask(input.machineCode),
		input.status,
		input.callbackURL,
		*dryRun,
	)
	if *dryRun {
		fmt.Println("dry_run: local authorization and config validated; provider was not called")
		return nil
	}

	client := cloudprint.NewYilianyunClientFromConfig(cfg)
	if client == nil {
		return fmt.Errorf("yilianyun client is not configured")
	}
	if err := client.SetPrintCallbackURL(ctx, cloudprint.YilianyunSetPrintCallbackURLInput{
		AccessToken: accessToken,
		MachineCode: input.machineCode,
		CallbackURL: input.callbackURL,
		Enabled:     input.status == "open",
	}); err != nil {
		return fmt.Errorf("set yilianyun print callback url: %w", err)
	}
	fmt.Println("yilianyun print callback registration succeeded")
	return nil
}

type commandInput struct {
	dbURL       string
	merchantID  int64
	machineCode string
	callbackURL string
	status      string
}

func buildCommandInput(cfg util.Config, dbURL string, merchantID int64, machineCode string, callbackURL string, status string) (commandInput, error) {
	connStr := strings.TrimSpace(dbURL)
	if connStr == "" {
		connStr = strings.TrimSpace(cfg.DBSource)
	}
	if connStr == "" {
		return commandInput{}, fmt.Errorf("db connection string is empty; pass -db or set DB_SOURCE")
	}
	if merchantID <= 0 {
		return commandInput{}, fmt.Errorf("-merchant-id must be a positive integer")
	}
	machineCode = strings.TrimSpace(machineCode)
	if machineCode == "" {
		return commandInput{}, fmt.Errorf("-machine-code is required")
	}
	if callbackURL = strings.TrimSpace(callbackURL); callbackURL == "" {
		callbackURL = strings.TrimSpace(cfg.YilianyunPrintCallbackURL)
	}
	if err := validateAbsoluteHTTPURL(callbackURL); err != nil {
		return commandInput{}, fmt.Errorf("callback url is invalid: %w", err)
	}
	status = strings.TrimSpace(strings.ToLower(status))
	if status != "open" && status != "close" {
		return commandInput{}, fmt.Errorf("-status must be open or close")
	}
	return commandInput{
		dbURL:       connStr,
		merchantID:  merchantID,
		machineCode: machineCode,
		callbackURL: callbackURL,
		status:      status,
	}, nil
}

func validateAbsoluteHTTPURL(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https")
	}
	if parsed.Host == "" {
		return fmt.Errorf("host is required")
	}
	return nil
}

func mask(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	if len(value) <= 8 {
		return value
	}
	return value[:4] + "***" + value[len(value)-4:]
}
