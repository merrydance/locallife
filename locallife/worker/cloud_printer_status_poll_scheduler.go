package worker

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/cloudprint"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const (
	cloudPrinterStatusPollDefaultInterval     = time.Minute
	cloudPrinterStatusPollDefaultBatchSize    = int32(50)
	cloudPrinterStatusPollDefaultInitialDelay = 30 * time.Second
	cloudPrinterStatusPollDefaultMaxAge       = 12 * time.Hour
	cloudPrinterStatusPollProviderTimeout     = 5 * time.Second
	cloudPrinterStatusPollExpiredMessage      = "provider_print_status_expired"
)

type CloudPrinterStatusPollScheduler struct {
	cron       *cron.Cron
	wg         sync.WaitGroup
	stopCtx    context.Context
	stopCancel context.CancelFunc
	runMu      sync.Mutex
	store      db.Store
	manager    cloudprint.Manager
	config     cloudPrinterStatusPollConfig
}

type cloudPrinterStatusPollConfig struct {
	interval     time.Duration
	batchSize    int32
	initialDelay time.Duration
	maxAge       time.Duration
}

func NewCloudPrinterStatusPollScheduler(store db.Store, manager cloudprint.Manager, config util.Config) *CloudPrinterStatusPollScheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &CloudPrinterStatusPollScheduler{
		cron: cron.New(cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
			cron.Recover(cron.DefaultLogger),
		)),
		stopCtx:    stopCtx,
		stopCancel: stopCancel,
		store:      store,
		manager:    manager,
		config:     normalizeCloudPrinterStatusPollConfig(config),
	}
}

func (s *CloudPrinterStatusPollScheduler) Start() error {
	spec := "@every " + s.config.interval.String()
	_, err := s.cron.AddFunc(spec, func() {
		s.runOnce(s.stopCtx)
	})
	if err != nil {
		return err
	}
	s.cron.Start()
	log.Info().Dur("interval", s.config.interval).Msg("cloud printer status poll scheduler started")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOnce(s.stopCtx)
	}()
	return nil
}

func (s *CloudPrinterStatusPollScheduler) Stop() {
	s.stopCancel()
	s.cron.Stop()
	s.wg.Wait()
	log.Info().Msg("cloud printer status poll scheduler stopped")
}

func (s *CloudPrinterStatusPollScheduler) RunOnce() {
	s.runOnce(context.Background())
}

func (s *CloudPrinterStatusPollScheduler) runOnce(ctx context.Context) {
	if s == nil || s.store == nil || s.manager == nil {
		return
	}
	if !s.runMu.TryLock() {
		log.Warn().Msg("cloud printer status poll already running, skipping concurrent execution")
		return
	}
	defer s.runMu.Unlock()

	providerTypes := s.pollableProviderTypes()
	if len(providerTypes) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, cloudPrinterStatusPollProviderTimeout*time.Duration(s.config.batchSize+1))
	defer cancel()

	now := time.Now().UTC()
	expired, err := s.store.ExpireProviderStatusPrintLogs(ctx, db.ExpireProviderStatusPrintLogsParams{
		PrinterTypes:  providerTypes,
		ExpiredBefore: now.Add(-s.config.maxAge),
		ErrorMessage:  pgtype.Text{String: cloudPrinterStatusPollExpiredMessage, Valid: true},
		LimitCount:    s.config.batchSize,
	})
	if err != nil {
		log.Error().Err(err).Strs("printer_types", providerTypes).Msg("expire stale provider print logs failed")
		return
	}
	if len(expired) > 0 {
		log.Warn().
			Int("count", len(expired)).
			Strs("printer_types", providerTypes).
			Msg("expired stale provider print logs")
	}

	rows, err := s.store.ClaimPendingProviderStatusPrintLogs(ctx, db.ClaimPendingProviderStatusPrintLogsParams{
		PrinterTypes:  providerTypes,
		ReadyBefore:   now.Add(-s.config.initialDelay),
		CreatedAfter:  now.Add(-s.config.maxAge),
		CheckedBefore: pgtype.Timestamptz{Time: now.Add(-s.config.interval), Valid: true},
		LimitCount:    s.config.batchSize,
	})
	if err != nil {
		log.Error().Err(err).Strs("printer_types", providerTypes).Msg("claim pending provider print logs failed")
		return
	}
	for _, row := range rows {
		s.pollOne(ctx, row)
	}
}

func (s *CloudPrinterStatusPollScheduler) pollOne(ctx context.Context, row db.ClaimPendingProviderStatusPrintLogsRow) {
	vendorOrderID := strings.TrimSpace(row.VendorOrderID.String)
	if !row.VendorOrderID.Valid || vendorOrderID == "" {
		return
	}
	provider, ok := s.manager.Provider(row.PrinterType)
	if !ok || provider == nil || !printProviderAcceptanceRequiresStatusQuery(row.PrinterType) {
		return
	}

	providerCtx, cancel := context.WithTimeout(ctx, cloudPrinterStatusPollProviderTimeout)
	defer cancel()
	printed, err := provider.QueryOrderState(providerCtx, vendorOrderID)
	if err != nil {
		sanitizedError := sanitizeProviderStatusPollError(err.Error())
		if _, updateErr := s.store.RecordProviderStatusPollError(ctx, db.RecordProviderStatusPollErrorParams{
			ID: row.ID,
			ProviderStatusLastError: pgtype.Text{
				String: sanitizedError,
				Valid:  true,
			},
		}); updateErr != nil && updateErr != db.ErrRecordNotFound {
			log.Error().Err(updateErr).Int64("print_log_id", row.ID).Msg("record cloud printer status poll error failed")
		}
		log.Warn().
			Str("error", sanitizedError).
			Int64("print_log_id", row.ID).
			Str("printer_type", row.PrinterType).
			Msg("cloud printer status poll provider query failed")
		return
	}
	if !printed {
		log.Info().
			Int64("print_log_id", row.ID).
			Str("printer_type", row.PrinterType).
			Msg("cloud printer print job is still pending")
		return
	}

	if _, err := s.store.MarkProviderStatusPrintLogTerminal(ctx, db.MarkProviderStatusPrintLogTerminalParams{
		ID:     row.ID,
		Status: printLogStatusSuccess,
	}); err != nil && err != db.ErrRecordNotFound {
		log.Error().Err(err).Int64("print_log_id", row.ID).Msg("mark provider print log success failed")
		return
	}
	log.Info().
		Int64("print_log_id", row.ID).
		Str("printer_type", row.PrinterType).
		Msg("cloud printer print job reconciled as success")
}

func (s *CloudPrinterStatusPollScheduler) pollableProviderTypes() []string {
	candidates := []string{printerTypeShangpeng}
	providerTypes := make([]string, 0, len(candidates))
	for _, providerType := range candidates {
		if provider, ok := s.manager.Provider(providerType); ok && provider != nil {
			providerTypes = append(providerTypes, providerType)
		}
	}
	return providerTypes
}

func normalizeCloudPrinterStatusPollConfig(config util.Config) cloudPrinterStatusPollConfig {
	interval := config.CloudPrinterStatusPollInterval
	if interval <= 0 {
		interval = cloudPrinterStatusPollDefaultInterval
	}
	batchSize := int32(config.CloudPrinterStatusPollBatchSize)
	if batchSize <= 0 {
		batchSize = cloudPrinterStatusPollDefaultBatchSize
	}
	initialDelay := config.CloudPrinterStatusPollInitialDelay
	if initialDelay < 0 {
		initialDelay = cloudPrinterStatusPollDefaultInitialDelay
	}
	maxAge := config.CloudPrinterStatusPollMaxAge
	if maxAge <= 0 {
		maxAge = cloudPrinterStatusPollDefaultMaxAge
	}
	return cloudPrinterStatusPollConfig{
		interval:     interval,
		batchSize:    batchSize,
		initialDelay: initialDelay,
		maxAge:       maxAge,
	}
}

func sanitizeProviderStatusPollError(message string) string {
	return sanitizePrintProviderErrorWithDefault(message, "provider_status_query_failed")
}

func sanitizePrintProviderError(message string) string {
	return sanitizePrintProviderErrorWithDefault(message, "provider_print_failed")
}

func sanitizePrintProviderErrorWithDefault(message string, defaultMessage string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return defaultMessage
	}
	message = maskDelimitedSecrets(message, sensitiveProviderErrorKeys())
	message = maskJSONLikeSecrets(message, sensitiveProviderErrorKeys())
	return truncatePrintError(message)
}

func sensitiveProviderErrorKeys() []string {
	return []string{"appsecret", "client_secret", "access_token", "refresh_token", "pkey", "printer_key", "sign"}
}

func maskDelimitedSecrets(message string, keys []string) string {
	if message == "" {
		return message
	}
	var out strings.Builder
	start := 0
	for start < len(message) {
		end := start
		for end < len(message) && !isProviderErrorDelimiter(message[end]) {
			end++
		}
		token := message[start:end]
		out.WriteString(maskSecretToken(token, keys))
		for end < len(message) && isProviderErrorDelimiter(message[end]) {
			out.WriteByte(message[end])
			end++
		}
		start = end
	}
	return out.String()
}

func maskSecretToken(token string, keys []string) string {
	lower := strings.ToLower(token)
	for _, key := range keys {
		prefix := key + "="
		index := strings.Index(lower, prefix)
		if index >= 0 {
			return token[:index] + prefix + "[redacted]"
		}
	}
	return token
}

func maskJSONLikeSecrets(message string, keys []string) string {
	if message == "" {
		return message
	}
	var out strings.Builder
	for i := 0; i < len(message); {
		matched := false
		for _, key := range keys {
			prefix := `"` + key + `"`
			if !strings.HasPrefix(strings.ToLower(message[i:]), prefix) {
				continue
			}
			j := i + len(prefix)
			for j < len(message) && (message[j] == ' ' || message[j] == '\t' || message[j] == '\n' || message[j] == '\r') {
				j++
			}
			if j >= len(message) || message[j] != ':' {
				continue
			}
			j++
			for j < len(message) && (message[j] == ' ' || message[j] == '\t' || message[j] == '\n' || message[j] == '\r') {
				j++
			}
			out.WriteString(message[i:j])
			if j < len(message) && message[j] == '"' {
				out.WriteString(`"[redacted]"`)
				j++
				for j < len(message) {
					if message[j] == '"' {
						j++
						break
					}
					if message[j] == '\\' && j+1 < len(message) {
						j += 2
						continue
					}
					j++
				}
			} else {
				out.WriteString("[redacted]")
				for j < len(message) && message[j] != ',' && message[j] != '}' && message[j] != ']' && message[j] != ' ' && message[j] != ';' {
					j++
				}
			}
			i = j
			matched = true
			break
		}
		if matched {
			continue
		}
		out.WriteByte(message[i])
		i++
	}
	return out.String()
}

func isProviderErrorDelimiter(ch byte) bool {
	return ch == ' ' || ch == '&' || ch == ',' || ch == ';'
}
