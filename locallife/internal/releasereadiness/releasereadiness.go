package releasereadiness

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/baofu"
	baofuaccount "github.com/merrydance/locallife/baofu/account"
	"github.com/merrydance/locallife/baofu/aggregatepay"
	"github.com/merrydance/locallife/baofu/merchantreport"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
)

const (
	StatusPass = "pass"
	StatusFail = "fail"
)

type Options struct {
	Root         string
	Expectations Expectations
}

type Expectations struct {
	RequiredSchedulers     []string
	RequiredWorkerHandlers []WorkerHandlerExpectation
}

type WorkerHandlerExpectation struct {
	TaskConst string
	TaskType  string
	Handler   string
}

type Report struct {
	Status string        `json:"status"`
	Checks []CheckResult `json:"checks"`
}

type CheckResult struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Detail  string `json:"detail,omitempty"`
	Source  string `json:"source,omitempty"`
	Line    int    `json:"line,omitempty"`
	Handler string `json:"handler,omitempty"`
}

type RedisAsynqOptions struct {
	Address        string
	Password       string
	RequiredQueues []string
}

type FixtureClaimabilityOptions struct {
	PaymentFactApplicationID int64
	PaymentDomainOutboxID    int64
}

type FixtureClaimStore interface {
	ClaimExternalPaymentFactApplication(ctx context.Context, id int64) (db.ExternalPaymentFactApplication, error)
	ClaimPaymentDomainOutbox(ctx context.Context, arg db.ClaimPaymentDomainOutboxParams) (db.PaymentDomainOutbox, error)
}

func DefaultExpectations() Expectations {
	return Expectations{
		RequiredSchedulers: []string{
			"payment-recovery",
			"payment-fact-application",
			"payment-domain-outbox",
			"baofu-payment-recovery",
			"baofu-account-opening-recovery",
			"baofu-withdrawal-recovery",
			"baofu-merchant-report-recovery",
			"refund-recovery",
			"claim-payout-recovery",
			"claim-behavior-action-recovery",
			"claim-recovery",
			"order-timeout",
			"takeout-auto-complete",
			"data-cleanup",
			"merchant-open-status",
		},
		RequiredWorkerHandlers: []WorkerHandlerExpectation{
			{TaskConst: "TaskPaymentOrderTimeout", TaskType: "payment_order:timeout", Handler: "ProcessTaskPaymentOrderTimeout"},
			{TaskConst: "TaskReservationPaymentTimeout", TaskType: "reservation:payment_timeout", Handler: "ProcessTaskReservationPaymentTimeout"},
			{TaskConst: "TaskOrderPaymentTimeout", TaskType: "order:payment_timeout", Handler: "ProcessTaskOrderPaymentTimeout"},
			{TaskConst: "TaskReservationNoShowAlert", TaskType: "reservation:no_show_alert", Handler: "ProcessTaskReservationNoShowAlert"},
			{TaskConst: "TaskReservationFoodSafetyAlert", TaskType: "reservation:food_safety_alert", Handler: "ProcessTaskReservationFoodSafetyAlert"},
			{TaskConst: "TaskProcessRefund", TaskType: "payment:initiate_refund", Handler: "ProcessTaskInitiateRefund"},
			{TaskConst: "TaskProcessRefundResult", TaskType: "payment:process_refund", Handler: "ProcessTaskRefundResult"},
			{TaskConst: "TaskSendNotification", TaskType: "notification:send", Handler: "ProcessTaskSendNotification"},
			{TaskConst: "TaskOperatorPendingDispatchAlert", TaskType: "operator:pending_dispatch_alert", Handler: "ProcessTaskOperatorPendingDispatchAlert"},
			{TaskConst: "TaskProcessAnomalyRefund", TaskType: "payment:process_anomaly_refund", Handler: "ProcessTaskAnomalyRefund"},
			{TaskConst: "TaskPrintOrder", TaskType: "order:print", Handler: "ProcessTaskPrintOrder"},
			{TaskConst: "TypeCheckMerchantForeignObject", TaskType: "risk:check_merchant_foreign_object", Handler: "HandleCheckMerchantForeignObject"},
			{TaskConst: "TypeCheckRiderDamage", TaskType: "risk:check_rider_damage", Handler: "HandleCheckRiderDamage"},
			{TaskConst: "TaskAutomaticRecoveryDisputeResolution", TaskType: "recovery_dispute:automatic_resolution", Handler: "ProcessTaskAutomaticRecoveryDisputeResolution"},
			{TaskConst: "TaskProcessRecoveryDisputeResult", TaskType: "recovery_dispute:process_result", Handler: "ProcessTaskRecoveryDisputeResult"},
			{TaskConst: "TaskClaimBehaviorAction", TaskType: "task:claim_behavior_action", Handler: "ProcessTaskClaimBehaviorAction"},
			{TaskConst: "TaskClaimPayout", TaskType: "task:claim_payout", Handler: "ProcessTaskClaimPayout"},
			{TaskConst: "TaskProcessPaymentFactApplication", TaskType: "payment:process_fact_application", Handler: "ProcessTaskPaymentFactApplication"},
			{TaskConst: "TaskProcessPaymentDomainOutbox", TaskType: "payment:process_domain_outbox", Handler: "ProcessTaskPaymentDomainOutbox"},
			{TaskConst: "TaskProcessBaofuProfitSharing", TaskType: "baofu:process_profit_sharing", Handler: "ProcessTaskBaofuProfitSharing"},
			{TaskConst: "TaskProcessBaofuAccountOpening", TaskType: "baofu:process_account_opening", Handler: "ProcessTaskBaofuAccountOpening"},
			{TaskConst: "TaskProcessBaofuWithdrawalFactApplication", TaskType: "baofu:process_withdrawal_fact_application", Handler: "ProcessTaskBaofuWithdrawalFactApplication"},
			{TaskConst: "TaskProcessBaofuWithdrawalCommandDispatch", TaskType: "baofu:process_withdrawal_command_dispatch", Handler: "ProcessTaskBaofuWithdrawalCommandDispatch"},
			{TaskConst: "TaskMerchantApplicationBusinessLicenseOCR", TaskType: "merchant_application:ocr_business_license", Handler: "ProcessTaskMerchantApplicationBusinessLicenseOCR"},
			{TaskConst: "TaskMerchantApplicationFoodPermitOCR", TaskType: "merchant_application:ocr_food_permit", Handler: "ProcessTaskMerchantApplicationFoodPermitOCR"},
			{TaskConst: "TaskMerchantApplicationIDCardOCR", TaskType: "merchant_application:ocr_id_card", Handler: "ProcessTaskMerchantApplicationIDCardOCR"},
			{TaskConst: "TaskOperatorApplicationBusinessLicenseOCR", TaskType: "operator_application:ocr_business_license", Handler: "ProcessTaskOperatorApplicationBusinessLicenseOCR"},
			{TaskConst: "TaskOperatorApplicationIDCardOCR", TaskType: "operator_application:ocr_id_card", Handler: "ProcessTaskOperatorApplicationIDCardOCR"},
			{TaskConst: "TaskRiderApplicationIDCardOCR", TaskType: "rider_application:ocr_id_card", Handler: "ProcessTaskRiderApplicationIDCardOCR"},
			{TaskConst: "TaskRiderApplicationHealthCertOCR", TaskType: "rider_application:ocr_health_cert", Handler: "ProcessTaskRiderApplicationHealthCertOCR"},
			{TaskConst: "TaskOnboardingReview", TaskType: "onboarding:review", Handler: "ProcessTaskOnboardingReview"},
			{TaskConst: "TaskGroupApplicationBusinessLicenseOCR", TaskType: "group_application:ocr_business_license", Handler: "ProcessTaskGroupApplicationBusinessLicenseOCR"},
			{TaskConst: "TaskGroupApplicationIDCardOCR", TaskType: "group_application:ocr_id_card", Handler: "ProcessTaskGroupApplicationIDCardOCR"},
		},
	}
}

func Check(opts Options) (Report, error) {
	root := strings.TrimSpace(opts.Root)
	if root == "" {
		root = "."
	}
	if err := EnsureRoot(root); err != nil {
		return Report{}, err
	}
	expectations := opts.Expectations
	if len(expectations.RequiredSchedulers) == 0 && len(expectations.RequiredWorkerHandlers) == 0 {
		expectations = DefaultExpectations()
	}

	registrations, err := scanSchedulerRegistrations(root)
	if err != nil {
		return Report{}, err
	}
	workerRegistrations, err := scanWorkerRegistrations(root)
	if err != nil {
		return Report{}, err
	}
	workerTaskTypes, err := scanWorkerTaskConstants(root)
	if err != nil {
		return Report{}, err
	}

	report := Report{Status: StatusPass}
	for _, name := range expectations.RequiredSchedulers {
		id := "scheduler:" + name
		if found, ok := registrations[name]; ok {
			report.Checks = append(report.Checks, CheckResult{
				ID:     id,
				Status: StatusPass,
				Detail: "registered",
				Source: found.source,
				Line:   found.line,
			})
			continue
		}
		report.Status = StatusFail
		report.Checks = append(report.Checks, CheckResult{
			ID:     id,
			Status: StatusFail,
			Detail: "missing scheduler registration",
		})
	}
	for _, expected := range expectations.RequiredWorkerHandlers {
		id := "worker:" + expected.TaskType
		found, ok := workerRegistrations[expected.TaskConst]
		actualTaskType, constOK := workerTaskTypes[expected.TaskConst]
		if ok && constOK && found.handler == expected.Handler && actualTaskType == expected.TaskType {
			report.Checks = append(report.Checks, CheckResult{
				ID:      id,
				Status:  StatusPass,
				Detail:  expected.TaskConst,
				Source:  found.source,
				Line:    found.line,
				Handler: found.handler,
			})
			continue
		}
		report.Status = StatusFail
		detail := "missing worker task registration"
		if !constOK {
			detail = "missing worker task constant"
		} else if actualTaskType != expected.TaskType {
			detail = fmt.Sprintf("task constant value %s, expected %s", actualTaskType, expected.TaskType)
		} else if ok {
			detail = fmt.Sprintf("registered with handler %s, expected %s", found.handler, expected.Handler)
		}
		report.Checks = append(report.Checks, CheckResult{
			ID:      id,
			Status:  StatusFail,
			Detail:  detail,
			Handler: expected.Handler,
		})
	}
	sort.SliceStable(report.Checks, func(i, j int) bool {
		return report.Checks[i].ID < report.Checks[j].ID
	})
	return report, nil
}

func MergeReports(reports ...Report) Report {
	merged := Report{Status: StatusPass}
	for _, report := range reports {
		if report.Status == StatusFail {
			merged.Status = StatusFail
		}
		merged.Checks = append(merged.Checks, report.Checks...)
	}
	sort.SliceStable(merged.Checks, func(i, j int) bool {
		return merged.Checks[i].ID < merged.Checks[j].ID
	})
	return merged
}

func CheckConfig(config util.Config) Report {
	report := Report{Status: StatusPass}
	if strings.TrimSpace(config.Environment) != "production" {
		report.Checks = append(report.Checks,
			CheckResult{ID: "config:production_allowed_origins", Status: StatusPass, Detail: "skipped outside production"},
			CheckResult{ID: "config:production_redis_address", Status: StatusPass, Detail: "skipped outside production"},
			CheckResult{ID: "config:production_data_encryption_key", Status: StatusPass, Detail: "skipped outside production"},
			CheckResult{ID: "config:production_payment_runtime", Status: StatusPass, Detail: "skipped outside production"},
		)
		return report
	}

	addConfigCheck := func(id string, ok bool, passDetail string, failDetail string) {
		status := StatusPass
		detail := passDetail
		if !ok {
			report.Status = StatusFail
			status = StatusFail
			detail = failDetail
		}
		report.Checks = append(report.Checks, CheckResult{
			ID:     id,
			Status: status,
			Detail: detail,
		})
	}

	addConfigCheck(
		"config:production_allowed_origins",
		hasExplicitAllowedOrigins(config.AllowedOrigins),
		"explicit allowed origins configured",
		"ALLOWED_ORIGINS must be non-empty and must not contain wildcard in production",
	)
	addConfigCheck(
		"config:production_redis_address",
		strings.TrimSpace(config.RedisAddress) != "",
		"redis address configured",
		"REDIS_ADDRESS is required in production",
	)
	addConfigCheck(
		"config:production_data_encryption_key",
		strings.TrimSpace(config.DataEncryptionKey) != "",
		"data encryption key configured",
		"DATA_ENCRYPTION_KEY is required in production",
	)
	if config.BaofuMainBusinessEnabled {
		if err := config.ValidateBaofuConfig(); err != nil {
			addConfigCheck("config:production_payment_runtime", false, "", err.Error())
		} else {
			addConfigCheck("config:production_payment_runtime", true, "baofu main business runtime config valid", "")
		}
	} else {
		addConfigCheck(
			"config:production_payment_runtime",
			false,
			"",
			"baofu main business runtime config is required in production for main-business payments",
		)
	}
	sort.SliceStable(report.Checks, func(i, j int) bool {
		return report.Checks[i].ID < report.Checks[j].ID
	})
	return report
}

func CheckRedisAsynq(opts RedisAsynqOptions) Report {
	report := Report{Status: StatusPass}
	address := strings.TrimSpace(opts.Address)
	queues := opts.RequiredQueues
	if len(queues) == 0 {
		queues = []string{"critical", "default"}
	}
	if address == "" {
		return Report{
			Status: StatusFail,
			Checks: []CheckResult{
				{ID: "redis:connection", Status: StatusFail, Detail: "REDIS_ADDRESS is required for redis/asynq readiness"},
			},
		}
	}

	redisClient := redis.NewClient(&redis.Options{Addr: address, Password: opts.Password})
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		return Report{
			Status: StatusFail,
			Checks: []CheckResult{
				{ID: "redis:connection", Status: StatusFail, Detail: err.Error()},
			},
		}
	}
	_ = redisClient.Close()

	report.Checks = append(report.Checks, CheckResult{
		ID:     "redis:connection",
		Status: StatusPass,
		Detail: "redis ping ok",
	})

	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: address, Password: opts.Password})
	defer inspector.Close()
	for _, queue := range queues {
		queue = strings.TrimSpace(queue)
		if queue == "" {
			continue
		}
		info, err := inspector.GetQueueInfo(queue)
		if err != nil {
			report.Checks = append(report.Checks, CheckResult{
				ID:     "asynq:queue:" + queue,
				Status: StatusPass,
				Detail: "queue absent; redis/asynq namespace readable",
			})
			continue
		}
		report.Checks = append(report.Checks, CheckResult{
			ID:     "asynq:queue:" + queue,
			Status: StatusPass,
			Detail: fmt.Sprintf("size=%d pending=%d active=%d paused=%t", info.Size, info.Pending, info.Active, info.Paused),
		})
	}
	sort.SliceStable(report.Checks, func(i, j int) bool {
		return report.Checks[i].ID < report.Checks[j].ID
	})
	return report
}

func CheckBaofuProviderClients(config util.Config) Report {
	report := Report{Status: StatusPass}
	if !config.BaofuMainBusinessEnabled && !config.HasBaofuRuntimeConfig() {
		report.Checks = append(report.Checks,
			CheckResult{ID: "provider:baofu:root", Status: StatusPass, Detail: "skipped: baofu runtime config disabled"},
			CheckResult{ID: "provider:baofu:aggregate", Status: StatusPass, Detail: "skipped: baofu runtime config disabled"},
			CheckResult{ID: "provider:baofu:account", Status: StatusPass, Detail: "skipped: baofu runtime config disabled"},
			CheckResult{ID: "provider:baofu:merchant_report", Status: StatusPass, Detail: "skipped: baofu runtime config disabled"},
		)
		return report
	}

	root, err := baofu.NewClient(config.ToBaofuConfig(), nil)
	if err != nil {
		return Report{
			Status: StatusFail,
			Checks: []CheckResult{
				{ID: "provider:baofu:root", Status: StatusFail, Detail: err.Error()},
			},
		}
	}
	report.Checks = append(report.Checks, CheckResult{
		ID:     "provider:baofu:root",
		Status: StatusPass,
		Detail: "baofu root client constructed",
	})
	if aggregateClient := aggregatepay.NewClient(root); aggregateClient == nil {
		report.Status = StatusFail
		report.Checks = append(report.Checks, CheckResult{ID: "provider:baofu:aggregate", Status: StatusFail, Detail: "aggregate client constructor returned nil"})
	} else {
		report.Checks = append(report.Checks, CheckResult{ID: "provider:baofu:aggregate", Status: StatusPass, Detail: "aggregate client constructed"})
	}
	if accountClient := baofuaccount.NewClient(root); accountClient == nil {
		report.Status = StatusFail
		report.Checks = append(report.Checks, CheckResult{ID: "provider:baofu:account", Status: StatusFail, Detail: "account client constructor returned nil"})
	} else {
		report.Checks = append(report.Checks, CheckResult{ID: "provider:baofu:account", Status: StatusPass, Detail: "account client constructed"})
	}
	if merchantReportClient := merchantreport.NewClient(root); merchantReportClient == nil {
		report.Status = StatusFail
		report.Checks = append(report.Checks, CheckResult{ID: "provider:baofu:merchant_report", Status: StatusFail, Detail: "merchant report client constructor returned nil"})
	} else {
		report.Checks = append(report.Checks, CheckResult{ID: "provider:baofu:merchant_report", Status: StatusPass, Detail: "merchant report client constructed"})
	}
	sort.SliceStable(report.Checks, func(i, j int) bool {
		return report.Checks[i].ID < report.Checks[j].ID
	})
	return report
}

func CheckFixtureClaimability(ctx context.Context, store FixtureClaimStore, opts FixtureClaimabilityOptions) Report {
	report := Report{Status: StatusPass}
	addCheck := func(id string, ok bool, detail string) {
		status := StatusPass
		if !ok {
			report.Status = StatusFail
			status = StatusFail
		}
		report.Checks = append(report.Checks, CheckResult{ID: id, Status: status, Detail: detail})
	}

	if store == nil {
		addCheck("fixture:payment_fact_application", false, "fixture claim store is required")
		addCheck("fixture:payment_domain_outbox", false, "fixture claim store is required")
		sort.SliceStable(report.Checks, func(i, j int) bool {
			return report.Checks[i].ID < report.Checks[j].ID
		})
		return report
	}

	if opts.PaymentFactApplicationID <= 0 {
		addCheck("fixture:payment_fact_application", false, "explicit payment fact application fixture id is required")
	} else {
		application, err := store.ClaimExternalPaymentFactApplication(ctx, opts.PaymentFactApplicationID)
		if err != nil {
			addCheck("fixture:payment_fact_application", false, err.Error())
		} else if application.Status != db.ExternalPaymentFactApplicationStatusProcessing {
			addCheck("fixture:payment_fact_application", false, fmt.Sprintf("claim returned status %s, expected %s", application.Status, db.ExternalPaymentFactApplicationStatusProcessing))
		} else {
			addCheck("fixture:payment_fact_application", true, fmt.Sprintf("payment fact application %d claimable", application.ID))
		}
	}

	if opts.PaymentDomainOutboxID <= 0 {
		addCheck("fixture:payment_domain_outbox", false, "explicit payment domain outbox fixture id is required")
	} else {
		outbox, err := store.ClaimPaymentDomainOutbox(ctx, db.ClaimPaymentDomainOutboxParams{
			ID:    opts.PaymentDomainOutboxID,
			NowAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		})
		if err != nil {
			addCheck("fixture:payment_domain_outbox", false, err.Error())
		} else if outbox.Status != db.PaymentDomainOutboxStatusProcessing {
			addCheck("fixture:payment_domain_outbox", false, fmt.Sprintf("claim returned status %s, expected %s", outbox.Status, db.PaymentDomainOutboxStatusProcessing))
		} else {
			addCheck("fixture:payment_domain_outbox", true, fmt.Sprintf("payment domain outbox %d claimable", outbox.ID))
		}
	}

	sort.SliceStable(report.Checks, func(i, j int) bool {
		return report.Checks[i].ID < report.Checks[j].ID
	})
	return report
}

func hasExplicitAllowedOrigins(origins []string) bool {
	if len(origins) == 0 {
		return false
	}
	for _, origin := range origins {
		if strings.TrimSpace(origin) == "*" {
			return false
		}
	}
	return true
}

type schedulerRegistration struct {
	source string
	line   int
}

type workerRegistration struct {
	handler string
	source  string
	line    int
}

func scanSchedulerRegistrations(root string) (map[string]schedulerRegistration, error) {
	files := []string{filepath.Join(root, "main.go")}
	registrations := map[string]schedulerRegistration{}
	fset := token.NewFileSet()
	for _, path := range files {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("parse scheduler registration source %s: %w", path, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isSelector(call.Fun, "Register") || len(call.Args) == 0 {
				return true
			}
			name, ok := stringLiteral(call.Args[0])
			if !ok {
				return true
			}
			pos := fset.Position(call.Pos())
			registrations[name] = schedulerRegistration{
				source: cleanSource(root, pos.Filename),
				line:   pos.Line,
			}
			return true
		})
	}
	return registrations, nil
}

func scanWorkerRegistrations(root string) (map[string]workerRegistration, error) {
	path := filepath.Join(root, "worker", "processor.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("parse worker processor source %s: %w", path, err)
	}
	registrations := map[string]workerRegistration{}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isSelector(call.Fun, "HandleFunc") || len(call.Args) < 2 {
			return true
		}
		taskConst, ok := identName(call.Args[0])
		if !ok {
			return true
		}
		handler, ok := selectorName(call.Args[1])
		if !ok {
			return true
		}
		pos := fset.Position(call.Pos())
		registrations[taskConst] = workerRegistration{
			handler: handler,
			source:  cleanSource(root, pos.Filename),
			line:    pos.Line,
		}
		return true
	})
	return registrations, nil
}

func scanWorkerTaskConstants(root string) (map[string]string, error) {
	workerDir := filepath.Join(root, "worker")
	entries, err := os.ReadDir(workerDir)
	if err != nil {
		return nil, fmt.Errorf("read worker dir: %w", err)
	}
	fset := token.NewFileSet()
	constants := map[string]string{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(workerDir, entry.Name())
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("parse worker source %s: %w", path, err)
		}
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.CONST {
				continue
			}
			for _, spec := range genDecl.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok || len(valueSpec.Values) == 0 {
					continue
				}
				value, ok := stringLiteral(valueSpec.Values[0])
				if !ok {
					continue
				}
				for _, name := range valueSpec.Names {
					if strings.HasPrefix(name.Name, "Task") || strings.HasPrefix(name.Name, "TypeCheck") {
						constants[name.Name] = value
					}
				}
			}
		}
	}
	return constants, nil
}

func isSelector(expr ast.Expr, name string) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == name
}

func selectorName(expr ast.Expr) (string, bool) {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	return selector.Sel.Name, true
}

func identName(expr ast.Expr) (string, bool) {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return "", false
	}
	return ident.Name, true
}

func stringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value := strings.Trim(lit.Value, `"`)
	return value, true
}

func cleanSource(root, filename string) string {
	rel, err := filepath.Rel(root, filename)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(filename)
	}
	return filepath.ToSlash(rel)
}

func WriteText(report Report, sb *strings.Builder) {
	sb.WriteString("release readiness smoke: ")
	sb.WriteString(report.Status)
	sb.WriteString("\n")
	for _, check := range report.Checks {
		sb.WriteString("- ")
		sb.WriteString(check.Status)
		sb.WriteString(" ")
		sb.WriteString(check.ID)
		if check.Detail != "" {
			sb.WriteString(" (")
			sb.WriteString(check.Detail)
			sb.WriteString(")")
		}
		if check.Source != "" {
			sb.WriteString(" ")
			sb.WriteString(check.Source)
			if check.Line > 0 {
				sb.WriteString(fmt.Sprintf(":%d", check.Line))
			}
		}
		sb.WriteString("\n")
	}
}

func EnsureRoot(root string) error {
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("stat root: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("root is not a directory: %s", root)
	}
	return nil
}
