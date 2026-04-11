package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/websocket"
	"github.com/merrydance/locallife/wechat"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

// billReconciliationCron 每天上午 10:10 运行
// 微信账单在 T+1 日 9:00 后才可用，10:10 拉取昨天（T-1）账单，给微信留出额外生成缓冲
const billReconciliationCron = "10 10 * * *"

// reconciliationTimeout 单次对账超时（下载+解析+DB操作）
const reconciliationTimeout = 10 * time.Minute

const (
	billDownloadBaseRetryDelay = 15 * time.Second
	billDownloadMaxRetryDelay  = 2 * time.Minute
	billDownloadMaxAttempts    = 4
)

// missingRecord 微信有 / 本地无（或反之）时记录的差异条目
type missingRecord struct {
	OutTradeNo string `json:"out_trade_no"`
	Amount     int64  `json:"amount"`
}

// amountMismatchRecord 金额不一致时记录的差异条目
type amountMismatchRecord struct {
	OutTradeNo  string `json:"out_trade_no"`
	WxpayAmount int64  `json:"wxpay_amount"`
	LocalAmount int64  `json:"local_amount"`
}

// BillReconciliationScheduler 每日自动对账调度器
// 每天 10:00 拉取昨天微信支付账单与本地数据库比对，将差异写入 reconciliation_reports
// 发现差异时同时通过 Redis Pub/Sub 推送告警（AlertChannel）
type BillReconciliationScheduler struct {
	cron       *cron.Cron
	store      db.Store
	billClient wechat.BillClientInterface // nil 时调度器注册但不执行
	publisher  websocket.PubSubPublisher  // nil 时降级为只写日志
	retryWait  func(context.Context, time.Duration) error
}

// NewBillReconciliationScheduler 创建每日对账调度器
// billClient 和 publisher 均可传 nil（未配置支付或 Redis 时自动降级）
func NewBillReconciliationScheduler(
	store db.Store,
	billClient wechat.BillClientInterface,
	publisher websocket.PubSubPublisher,
) *BillReconciliationScheduler {
	return &BillReconciliationScheduler{
		cron: cron.New(cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
			cron.Recover(cron.DefaultLogger),
		)),
		store:      store,
		billClient: billClient,
		publisher:  publisher,
		retryWait:  waitForBillRetry,
	}
}

// Start 启动调度器
func (s *BillReconciliationScheduler) Start() error {
	if s.billClient == nil {
		log.Warn().Msg("bill reconciliation: payment client not configured, scheduler registered but will not run")
	}
	_, err := s.cron.AddFunc(billReconciliationCron, func() {
		s.runAll()
	})
	if err != nil {
		return err
	}
	s.cron.Start()
	log.Info().Msg("bill reconciliation scheduler started (daily 10:10)")
	return nil
}

// Stop 停止调度器
func (s *BillReconciliationScheduler) Stop() {
	s.cron.Stop()
	log.Info().Msg("bill reconciliation scheduler stopped")
}

// RunAll 手动触发全量对账（用于运维补跑）
func (s *BillReconciliationScheduler) RunAll() {
	s.runAll()
}

func (s *BillReconciliationScheduler) runAll() {
	if s.billClient == nil {
		return
	}
	// 微信账单 T+1 9:00 后可用，10:10 跑时拉取昨天（T-1）账单
	// 使用 time.Local 构造本地时区的昨日零点，与项目其他调度器保持一致（参见 data_cleanup.go）
	yesterdayLocal := time.Now().AddDate(0, 0, -1)
	billDate := time.Date(yesterdayLocal.Year(), yesterdayLocal.Month(), yesterdayLocal.Day(), 0, 0, 0, 0, time.Local)

	log.Info().Str("bill_date", billDate.Format("2006-01-02")).Msg("bill reconciliation started")

	s.reconcileTrade(billDate)
	s.reconcileEcommerceTrade(billDate)
	s.reconcileRefund(billDate)
	s.reconcileEcommerceRefund(billDate)

	log.Info().Str("bill_date", billDate.Format("2006-01-02")).Msg("bill reconciliation finished")
}

// reconcileTrade 对账小程序直连支付交易账单（payment_orders WHERE payment_type='miniprogram'）
func (s *BillReconciliationScheduler) reconcileTrade(billDate time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), reconciliationTimeout)
	defer cancel()

	const billType = "trade"
	reportID, err := s.createReport(ctx, billDate, billType)
	if err != nil {
		log.Error().Err(err).Str("bill_type", billType).Msg("reconciliation: create report failed")
		return
	}

	wxRecords, err := s.fetchBillRecords(ctx, billType, billDate, s.billClient.DownloadTradeBill)
	if err != nil {
		s.failReport(ctx, reportID, "download trade bill: "+err.Error())
		log.Error().Err(err).Str("bill_type", billType).Msg("reconciliation: download bill failed")
		return
	}

	start := pgtype.Timestamptz{Time: billDate, Valid: true}
	end := pgtype.Timestamptz{Time: billDate.AddDate(0, 0, 1), Valid: true}
	localRows, err := s.store.ListMiniprogramPaymentOrdersForReconciliation(ctx,
		db.ListMiniprogramPaymentOrdersForReconciliationParams{PaidAt: start, PaidAt_2: end})
	if err != nil {
		s.failReport(ctx, reportID, "query local records: "+err.Error())
		log.Error().Err(err).Str("bill_type", billType).Msg("reconciliation: query local records failed")
		return
	}

	localMap := make(map[string]int64, len(localRows))
	for _, r := range localRows {
		localMap[r.OutTradeNo] = r.Amount
	}

	missingLocal, missingWxpay, amountMismatch := compareRecords(wxRecords, localMap)
	s.saveReport(ctx, reportID, billDate, billType, len(wxRecords), len(localRows), missingLocal, missingWxpay, amountMismatch)
}

// reconcileEcommerceTrade 对账电商收付通合单交易账单（combined_payment_orders）
func (s *BillReconciliationScheduler) reconcileEcommerceTrade(billDate time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), reconciliationTimeout)
	defer cancel()

	const billType = "ecommerce_trade"
	reportID, err := s.createReport(ctx, billDate, billType)
	if err != nil {
		log.Error().Err(err).Str("bill_type", billType).Msg("reconciliation: create report failed")
		return
	}

	wxRecords, err := s.fetchBillRecords(ctx, billType, billDate, s.billClient.DownloadEcommerceTradeBill)
	if err != nil {
		s.failReport(ctx, reportID, "download ecommerce trade bill: "+err.Error())
		log.Error().Err(err).Str("bill_type", billType).Msg("reconciliation: download bill failed")
		return
	}

	start := pgtype.Timestamptz{Time: billDate, Valid: true}
	end := pgtype.Timestamptz{Time: billDate.AddDate(0, 0, 1), Valid: true}
	localRows, err := s.store.ListCombinedPaymentOrdersForReconciliation(ctx,
		db.ListCombinedPaymentOrdersForReconciliationParams{PaidAt: start, PaidAt_2: end})
	if err != nil {
		s.failReport(ctx, reportID, "query local records: "+err.Error())
		log.Error().Err(err).Str("bill_type", billType).Msg("reconciliation: query local records failed")
		return
	}

	localMap := make(map[string]int64, len(localRows))
	for _, r := range localRows {
		localMap[r.CombineOutTradeNo] = r.TotalAmount
	}

	missingLocal, missingWxpay, amountMismatch := compareRecords(wxRecords, localMap)
	s.saveReport(ctx, reportID, billDate, billType, len(wxRecords), len(localRows), missingLocal, missingWxpay, amountMismatch)
}

// reconcileRefund 对账小程序直连退款账单（refund_orders WHERE payment_type!='profit_sharing'）
func (s *BillReconciliationScheduler) reconcileRefund(billDate time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), reconciliationTimeout)
	defer cancel()

	const billType = "refund"
	reportID, err := s.createReport(ctx, billDate, billType)
	if err != nil {
		log.Error().Err(err).Str("bill_type", billType).Msg("reconciliation: create report failed")
		return
	}

	wxRecords, err := s.fetchBillRecords(ctx, billType, billDate, s.billClient.DownloadRefundBill)
	if err != nil {
		s.failReport(ctx, reportID, "download refund bill: "+err.Error())
		log.Error().Err(err).Str("bill_type", billType).Msg("reconciliation: download bill failed")
		return
	}

	start := pgtype.Timestamptz{Time: billDate, Valid: true}
	end := pgtype.Timestamptz{Time: billDate.AddDate(0, 0, 1), Valid: true}
	localRows, err := s.store.ListRefundOrdersForReconciliation(ctx,
		db.ListRefundOrdersForReconciliationParams{RefundedAt: start, RefundedAt_2: end})
	if err != nil {
		s.failReport(ctx, reportID, "query local records: "+err.Error())
		log.Error().Err(err).Str("bill_type", billType).Msg("reconciliation: query local records failed")
		return
	}

	localMap := make(map[string]int64, len(localRows))
	for _, r := range localRows {
		localMap[r.OutRefundNo] = r.RefundAmount
	}

	missingLocal, missingWxpay, amountMismatch := compareRecords(wxRecords, localMap)
	s.saveReport(ctx, reportID, billDate, billType, len(wxRecords), len(localRows), missingLocal, missingWxpay, amountMismatch)
}

// reconcileEcommerceRefund 对账电商收付通退款账单（refund_orders WHERE payment_type='profit_sharing'）
// 对应微信 /v3/ecommerce/bill/refundbill 账单
func (s *BillReconciliationScheduler) reconcileEcommerceRefund(billDate time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), reconciliationTimeout)
	defer cancel()

	const billType = "ecommerce_refund"
	reportID, err := s.createReport(ctx, billDate, billType)
	if err != nil {
		log.Error().Err(err).Str("bill_type", billType).Msg("reconciliation: create report failed")
		return
	}

	wxRecords, err := s.fetchBillRecords(ctx, billType, billDate, s.billClient.DownloadEcommerceRefundBill)
	if err != nil {
		s.failReport(ctx, reportID, "download ecommerce refund bill: "+err.Error())
		log.Error().Err(err).Str("bill_type", billType).Msg("reconciliation: download bill failed")
		return
	}

	start := pgtype.Timestamptz{Time: billDate, Valid: true}
	end := pgtype.Timestamptz{Time: billDate.AddDate(0, 0, 1), Valid: true}
	localRows, err := s.store.ListEcommerceRefundOrdersForReconciliation(ctx,
		db.ListEcommerceRefundOrdersForReconciliationParams{RefundedAt: start, RefundedAt_2: end})
	if err != nil {
		s.failReport(ctx, reportID, "query local records: "+err.Error())
		log.Error().Err(err).Str("bill_type", billType).Msg("reconciliation: query local records failed")
		return
	}

	localMap := make(map[string]int64, len(localRows))
	for _, r := range localRows {
		localMap[r.OutRefundNo] = r.RefundAmount
	}

	missingLocal, missingWxpay, amountMismatch := compareRecords(wxRecords, localMap)
	s.saveReport(ctx, reportID, billDate, billType, len(wxRecords), len(localRows), missingLocal, missingWxpay, amountMismatch)
}

// compareRecords 比对微信账单记录与本地记录，返回三类差异列表
func compareRecords(
	wxRecords map[string]wechat.BillRecord,
	localMap map[string]int64,
) (missingLocal, missingWxpay []missingRecord, amountMismatch []amountMismatchRecord) {
	for outTradeNo, wx := range wxRecords {
		localAmt, exists := localMap[outTradeNo]
		if !exists {
			missingLocal = append(missingLocal, missingRecord{OutTradeNo: outTradeNo, Amount: wx.Amount})
		} else if localAmt != wx.Amount {
			amountMismatch = append(amountMismatch, amountMismatchRecord{
				OutTradeNo:  outTradeNo,
				WxpayAmount: wx.Amount,
				LocalAmount: localAmt,
			})
		}
	}
	for outTradeNo, localAmt := range localMap {
		if _, exists := wxRecords[outTradeNo]; !exists {
			missingWxpay = append(missingWxpay, missingRecord{OutTradeNo: outTradeNo, Amount: localAmt})
		}
	}
	return
}

func (s *BillReconciliationScheduler) fetchBillRecords(
	ctx context.Context,
	billType string,
	billDate time.Time,
	fetch func(context.Context, time.Time) (map[string]wechat.BillRecord, error),
) (map[string]wechat.BillRecord, error) {
	retryWait := s.retryWait
	if retryWait == nil {
		retryWait = waitForBillRetry
	}

	for attempt := 1; attempt <= billDownloadMaxAttempts; attempt++ {
		records, err := fetch(ctx, billDate)
		if err == nil {
			return records, nil
		}

		if errors.Is(err, wechat.ErrBillNotFound) {
			log.Info().
				Str("bill_type", billType).
				Str("bill_date", billDate.Format("2006-01-02")).
				Msg("bill reconciliation: wechat bill not found, treating as empty statement")
			return map[string]wechat.BillRecord{}, nil
		}

		if !errors.Is(err, wechat.ErrBillNotReady) {
			return nil, err
		}

		if attempt == billDownloadMaxAttempts {
			return nil, fmt.Errorf("wechat bill still generating after %d attempts: %w", attempt, err)
		}

		delay := nextBillRetryDelay(attempt)

		log.Warn().
			Err(err).
			Str("bill_type", billType).
			Str("bill_date", billDate.Format("2006-01-02")).
			Int("attempt", attempt).
			Dur("retry_after", delay).
			Msg("bill reconciliation: wechat bill still generating, retrying")

		if err := retryWait(ctx, delay); err != nil {
			return nil, fmt.Errorf("wait for wechat bill retry: %w", err)
		}
	}

	return nil, fmt.Errorf("unreachable bill fetch state for bill_type=%s", billType)
}

func nextBillRetryDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return billDownloadBaseRetryDelay
	}

	delay := billDownloadBaseRetryDelay << (attempt - 1)
	if delay > billDownloadMaxRetryDelay {
		return billDownloadMaxRetryDelay
	}

	return delay
}

func waitForBillRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// createReport 在数据库创建或重置对账报告（ON CONFLICT 重新跑时覆盖）
func (s *BillReconciliationScheduler) createReport(ctx context.Context, billDate time.Time, billType string) (int64, error) {
	report, err := s.store.CreateReconciliationReport(ctx, db.CreateReconciliationReportParams{
		BillDate: pgtype.Date{Time: billDate, Valid: true},
		BillType: billType,
	})
	if err != nil {
		return 0, err
	}
	return report.ID, nil
}

// failReport 将对账报告标记为 failed 并记录错误信息
func (s *BillReconciliationScheduler) failReport(ctx context.Context, reportID int64, errMsg string) {
	emptyJSON, _ := json.Marshal([]struct{}{})
	_, err := s.store.UpdateReconciliationReport(ctx, db.UpdateReconciliationReportParams{
		ID:             reportID,
		Status:         "failed",
		WxpayCount:     0,
		LocalCount:     0,
		MismatchCount:  0,
		MissingLocal:   emptyJSON,
		MissingWxpay:   emptyJSON,
		AmountMismatch: emptyJSON,
		ErrorMessage:   pgtype.Text{String: errMsg, Valid: true},
	})
	if err != nil {
		log.Error().Err(err).Msg("reconciliation: update report to failed status failed")
	}
}

func safeReconciliationCount(value int) (int32, error) {
	if value < 0 {
		return 0, fmt.Errorf("count must be non-negative: %d", value)
	}
	if value > math.MaxInt32 {
		return 0, fmt.Errorf("count exceeds int32 max: %d", value)
	}
	return int32(value), nil
}

// saveReport 保存对账结果，有差异时额外通过 Redis Pub/Sub 推送告警
func (s *BillReconciliationScheduler) saveReport(
	ctx context.Context,
	reportID int64,
	billDate time.Time,
	billType string,
	wxCount, localCount int,
	missingLocal, missingWxpay []missingRecord,
	amountMismatch []amountMismatchRecord,
) {
	mismatchCount := len(missingLocal) + len(missingWxpay) + len(amountMismatch)
	wxpayCount32, err := safeReconciliationCount(wxCount)
	if err != nil {
		errMsg := fmt.Sprintf("reconciliation report wxpay count overflow for %s on %s: %v", billType, billDate.Format("2006-01-02"), err)
		log.Error().Str("bill_type", billType).Err(err).Msg("reconciliation: invalid wxpay count")
		s.failReport(ctx, reportID, errMsg)
		return
	}
	localCount32, err := safeReconciliationCount(localCount)
	if err != nil {
		errMsg := fmt.Sprintf("reconciliation report local count overflow for %s on %s: %v", billType, billDate.Format("2006-01-02"), err)
		log.Error().Str("bill_type", billType).Err(err).Msg("reconciliation: invalid local count")
		s.failReport(ctx, reportID, errMsg)
		return
	}
	mismatchCount32, err := safeReconciliationCount(mismatchCount)
	if err != nil {
		errMsg := fmt.Sprintf("reconciliation report mismatch count overflow for %s on %s: %v", billType, billDate.Format("2006-01-02"), err)
		log.Error().Str("bill_type", billType).Err(err).Msg("reconciliation: invalid mismatch count")
		s.failReport(ctx, reportID, errMsg)
		return
	}

	// 确保 nil slice 序列化为 "[]" 而非 "null"，防止 JSONB 列存储 null 导致前端操作失败
	if missingLocal == nil {
		missingLocal = []missingRecord{}
	}
	if missingWxpay == nil {
		missingWxpay = []missingRecord{}
	}
	if amountMismatch == nil {
		amountMismatch = []amountMismatchRecord{}
	}
	missingLocalJSON, _ := json.Marshal(missingLocal)
	missingWxpayJSON, _ := json.Marshal(missingWxpay)
	amountMismatchJSON, _ := json.Marshal(amountMismatch)

	_, err = s.store.UpdateReconciliationReport(ctx, db.UpdateReconciliationReportParams{
		ID:             reportID,
		Status:         "completed",
		WxpayCount:     wxpayCount32,
		LocalCount:     localCount32,
		MismatchCount:  mismatchCount32,
		MissingLocal:   missingLocalJSON,
		MissingWxpay:   missingWxpayJSON,
		AmountMismatch: amountMismatchJSON,
		ErrorMessage:   pgtype.Text{Valid: false},
	})
	if err != nil {
		log.Error().Err(err).Str("bill_type", billType).Msg("reconciliation: save report failed")
		return
	}

	if mismatchCount == 0 {
		log.Info().
			Str("bill_type", billType).
			Int("wxpay_count", wxCount).
			Int("local_count", localCount).
			Msg("bill reconciliation completed: no discrepancies")
		return
	}

	// 有差异：打 Warn 日志 + 推送告警到 Redis Pub/Sub
	log.Warn().
		Str("bill_type", billType).
		Int("wxpay_count", wxCount).
		Int("local_count", localCount).
		Int("missing_local", len(missingLocal)).
		Int("missing_wxpay", len(missingWxpay)).
		Int("amount_mismatch", len(amountMismatch)).
		Msg("bill reconciliation completed: discrepancies found")

	s.publishMismatchAlert(ctx, billDate, billType, mismatchCount, len(missingLocal), len(missingWxpay), len(amountMismatch))
}

// publishMismatchAlert 通过 Redis Pub/Sub 发布对账差异告警
// 前端/运营后台通过 AlertChannel 订阅接收
func (s *BillReconciliationScheduler) publishMismatchAlert(
	ctx context.Context,
	billDate time.Time,
	billType string,
	mismatchCount, missingLocal, missingWxpay, amountMismatch int,
) {
	if s.publisher == nil {
		return
	}

	alert := AlertData{
		AlertType: AlertTypeBillMismatch,
		Level:     AlertLevelWarning,
		Title:     "对账发现差异",
		Message:   billDate.Format("2006-01-02") + " 账单（" + billType + "）共 " + itoa(mismatchCount) + " 笔异常",
		Extra: map[string]interface{}{
			"bill_date":       billDate.Format("2006-01-02"),
			"bill_type":       billType,
			"missing_local":   missingLocal,
			"missing_wxpay":   missingWxpay,
			"amount_mismatch": amountMismatch,
		},
		Timestamp: time.Now(),
	}

	alertMsg := map[string]any{
		"type":      "alert",
		"data":      alert,
		"timestamp": alert.Timestamp,
	}
	payload, err := json.Marshal(alertMsg)
	if err != nil {
		log.Error().Err(err).Msg("reconciliation: marshal alert failed")
		return
	}
	if err := s.publisher.Publish(ctx, AlertChannel, payload); err != nil {
		log.Error().Err(err).Str("bill_type", billType).Msg("reconciliation: publish alert failed")
	} else {
		log.Info().Str("bill_type", billType).Int("mismatch_count", mismatchCount).Msg("reconciliation alert published")
	}
}

// itoa wraps strconv.Itoa for use in string concatenation
func itoa(n int) string {
	return strconv.Itoa(n)
}
