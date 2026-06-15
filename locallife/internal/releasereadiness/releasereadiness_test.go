package releasereadiness

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckDefaultReportCoversReleaseCriticalSchedulersAndWorkerHandlers(t *testing.T) {
	report, err := Check(Options{
		Root:         filepath.Join("..", ".."),
		Expectations: DefaultExpectations(),
	})
	require.NoError(t, err)
	require.Equal(t, StatusPass, report.Status)

	for _, name := range []string{
		"payment-recovery",
		"payment-fact-application",
		"payment-domain-outbox",
		"baofu-payment-recovery",
		"baofu-account-opening-recovery",
		"baofu-withdrawal-recovery",
		"baofu-merchant-report-recovery",
		"refund-recovery",
		"order-timeout",
		"takeout-auto-complete",
		"merchant-open-status",
	} {
		check := requireCheck(t, report, "scheduler:"+name)
		require.Equal(t, StatusPass, check.Status)
	}

	for _, taskType := range []string{
		"payment:process_fact_application",
		"payment:process_domain_outbox",
		"baofu:process_profit_sharing",
		"baofu:process_account_opening",
		"baofu:process_withdrawal_fact_application",
		"baofu:process_withdrawal_command_dispatch",
	} {
		check := requireCheck(t, report, "worker:"+taskType)
		require.Equal(t, StatusPass, check.Status)
	}
}

func TestCheckReportsMissingRequiredRegistrations(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "main.go", `package main

func main() {
	schedulerManager.Register("payment-recovery", nil)
}
`)
	writeFixture(t, root, "worker/processor.go", `package worker

const TaskProcessPaymentFactApplication = "payment:process_fact_application"

func (processor *RedisTaskProcessor) Start() error {
	mux.HandleFunc(TaskProcessPaymentFactApplication, processor.ProcessTaskPaymentFactApplication)
	return nil
}
`)

	report, err := Check(Options{
		Root: root,
		Expectations: Expectations{
			RequiredSchedulers: []string{
				"payment-recovery",
				"refund-recovery",
			},
			RequiredWorkerHandlers: []WorkerHandlerExpectation{
				{
					TaskConst: "TaskProcessPaymentFactApplication",
					TaskType:  "payment:process_fact_application",
					Handler:   "ProcessTaskPaymentFactApplication",
				},
				{
					TaskConst: "TaskProcessPaymentDomainOutbox",
					TaskType:  "payment:process_domain_outbox",
					Handler:   "ProcessTaskPaymentDomainOutbox",
				},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, StatusFail, report.Status)
	require.Equal(t, StatusPass, requireCheck(t, report, "scheduler:payment-recovery").Status)
	require.Equal(t, StatusFail, requireCheck(t, report, "scheduler:refund-recovery").Status)
	require.Equal(t, StatusPass, requireCheck(t, report, "worker:payment:process_fact_application").Status)
	require.Equal(t, StatusFail, requireCheck(t, report, "worker:payment:process_domain_outbox").Status)
}

func requireCheck(t *testing.T, report Report, id string) CheckResult {
	t.Helper()
	for _, check := range report.Checks {
		if check.ID == id {
			return check
		}
	}
	t.Fatalf("missing check %q in report: %#v", id, report.Checks)
	return CheckResult{}
}

func writeFixture(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
}
