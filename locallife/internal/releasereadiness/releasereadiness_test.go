package releasereadiness

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/alicebob/miniredis/v2"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
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
		"dine-in-checkout-recovery",
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

func TestCheckProductionConfigReadiness(t *testing.T) {
	base := validProductionBaofuReadinessConfig()

	report := CheckConfig(base)
	require.Equal(t, StatusPass, report.Status)
	require.Equal(t, StatusPass, requireCheck(t, report, "config:production_allowed_origins").Status)
	require.Equal(t, StatusPass, requireCheck(t, report, "config:production_redis_address").Status)
	require.Equal(t, StatusPass, requireCheck(t, report, "config:production_data_encryption_key").Status)
	require.Equal(t, StatusPass, requireCheck(t, report, "config:production_payment_runtime").Status)

	missing := base
	missing.AllowedOrigins = []string{"*"}
	missing.RedisAddress = ""
	missing.DataEncryptionKey = ""
	missing.BaofuWithdrawNotifyURL = ""
	missing.BaofuNotifyBaseURL = ""

	report = CheckConfig(missing)
	require.Equal(t, StatusFail, report.Status)
	require.Equal(t, StatusFail, requireCheck(t, report, "config:production_allowed_origins").Status)
	require.Equal(t, StatusFail, requireCheck(t, report, "config:production_redis_address").Status)
	require.Equal(t, StatusFail, requireCheck(t, report, "config:production_data_encryption_key").Status)
	require.Equal(t, StatusFail, requireCheck(t, report, "config:production_payment_runtime").Status)
}

func TestMergeReportsKeepsFailStatus(t *testing.T) {
	merged := MergeReports(
		Report{Status: StatusPass, Checks: []CheckResult{{ID: "a", Status: StatusPass}}},
		Report{Status: StatusFail, Checks: []CheckResult{{ID: "b", Status: StatusFail}}},
	)
	require.Equal(t, StatusFail, merged.Status)
	require.Equal(t, StatusPass, requireCheck(t, merged, "a").Status)
	require.Equal(t, StatusFail, requireCheck(t, merged, "b").Status)
}

func TestCheckConfigSkipsOutsideProduction(t *testing.T) {
	report := CheckConfig(util.Config{Environment: "test"})
	require.Equal(t, StatusPass, report.Status)
	require.Equal(t, "skipped outside production", requireCheck(t, report, "config:production_payment_runtime").Detail)
}

func TestCheckRedisAsynqReadiness(t *testing.T) {
	redisServer := miniredis.RunT(t)

	report := CheckRedisAsynq(RedisAsynqOptions{
		Address:        redisServer.Addr(),
		RequiredQueues: []string{"critical", "default"},
	})
	require.Equal(t, StatusPass, report.Status)
	require.Equal(t, StatusPass, requireCheck(t, report, "redis:connection").Status)
	require.Equal(t, StatusPass, requireCheck(t, report, "asynq:queue:critical").Status)
	require.Equal(t, StatusPass, requireCheck(t, report, "asynq:queue:default").Status)

	report = CheckRedisAsynq(RedisAsynqOptions{RequiredQueues: []string{"critical"}})
	require.Equal(t, StatusFail, report.Status)
	require.Equal(t, StatusFail, requireCheck(t, report, "redis:connection").Status)
}

func TestCheckBaofuProviderClientReadiness(t *testing.T) {
	config := validProductionBaofuReadinessConfig()

	report := CheckBaofuProviderClients(config)
	require.Equal(t, StatusPass, report.Status)
	require.Equal(t, StatusPass, requireCheck(t, report, "provider:baofu:root").Status)
	require.Equal(t, StatusPass, requireCheck(t, report, "provider:baofu:aggregate").Status)
	require.Equal(t, StatusPass, requireCheck(t, report, "provider:baofu:account").Status)
	require.Equal(t, StatusPass, requireCheck(t, report, "provider:baofu:merchant_report").Status)

	config.BaofuPublicKeyPEM = ""
	report = CheckBaofuProviderClients(config)
	require.Equal(t, StatusFail, report.Status)
	require.Equal(t, StatusFail, requireCheck(t, report, "provider:baofu:root").Status)
}

func TestCheckFixtureClaimabilityRequiresExplicitFixtureIDs(t *testing.T) {
	report := CheckFixtureClaimability(context.Background(), &fakeFixtureClaimer{}, FixtureClaimabilityOptions{})

	require.Equal(t, StatusFail, report.Status)
	require.Equal(t, StatusFail, requireCheck(t, report, "fixture:payment_fact_application").Status)
	require.Equal(t, StatusFail, requireCheck(t, report, "fixture:payment_domain_outbox").Status)
}

func TestCheckFixtureClaimabilityProvesClaimableRows(t *testing.T) {
	claimer := &fakeFixtureClaimer{
		application: db.ExternalPaymentFactApplication{ID: 101, Status: db.ExternalPaymentFactApplicationStatusProcessing},
		outbox:      db.PaymentDomainOutbox{ID: 202, Status: db.PaymentDomainOutboxStatusProcessing},
	}

	report := CheckFixtureClaimability(context.Background(), claimer, FixtureClaimabilityOptions{
		PaymentFactApplicationID: 101,
		PaymentDomainOutboxID:    202,
	})

	require.Equal(t, StatusPass, report.Status)
	require.Equal(t, StatusPass, requireCheck(t, report, "fixture:payment_fact_application").Status)
	require.Equal(t, StatusPass, requireCheck(t, report, "fixture:payment_domain_outbox").Status)
	require.Equal(t, int64(101), claimer.claimedApplicationID)
	require.Equal(t, int64(202), claimer.claimedOutboxID)
}

func TestCheckFixtureClaimabilityFailsWhenRowCannotBeClaimed(t *testing.T) {
	claimer := &fakeFixtureClaimer{
		applicationErr: db.ErrRecordNotFound,
		outbox:         db.PaymentDomainOutbox{ID: 202, Status: db.PaymentDomainOutboxStatusProcessing},
	}

	report := CheckFixtureClaimability(context.Background(), claimer, FixtureClaimabilityOptions{
		PaymentFactApplicationID: 101,
		PaymentDomainOutboxID:    202,
	})

	require.Equal(t, StatusFail, report.Status)
	require.Equal(t, StatusFail, requireCheck(t, report, "fixture:payment_fact_application").Status)
	require.Equal(t, StatusPass, requireCheck(t, report, "fixture:payment_domain_outbox").Status)
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

func validProductionBaofuReadinessConfig() util.Config {
	return util.Config{
		Environment:                    "production",
		AllowedOrigins:                 []string{"https://admin.example.com"},
		RedisAddress:                   "redis:6379",
		DataEncryptionKey:              "dummy-test-data-encryption-key",
		BaofuMainBusinessEnabled:       true,
		WechatMiniAppID:                "wx-app",
		BaofuEnvironment:               "sandbox",
		BaofuAccountGatewayBaseURL:     "https://vgw.baofoo.com/union-gw/api",
		BaofuAggregatePayBaseURL:       "https://mch-juhe.baofoo.com/api",
		BaofuMerchantReportBaseURL:     "https://mch-juhe.baofoo.com/mch-service/api",
		BaofuCollectMerchantID:         "collect-merchant",
		BaofuCollectTerminalID:         "collect-terminal",
		BaofuPayoutMerchantID:          "payout-merchant",
		BaofuPayoutTerminalID:          "payout-terminal",
		BaofuAppID:                     "baofu-app",
		BaofuPrivateKeyPEM:             testPrivateKeyPEM,
		BaofuPublicKeyPEM:              testPublicKeyPEM,
		BaofuSignSerialNo:              "1234567890",
		BaofuEncryptionSerialNo:        "1234567891",
		BaofuNotifyBaseURL:             "https://api.example.com/v1/webhooks/baofu",
		BaofuPaymentNotifyURL:          "https://api.example.com/v1/webhooks/baofu/payment",
		BaofuProfitSharingNotifyURL:    "https://api.example.com/v1/webhooks/baofu/share",
		BaofuRefundNotifyURL:           "https://api.example.com/v1/webhooks/baofu/refund",
		BaofuWithdrawNotifyURL:         "https://api.example.com/v1/webhooks/baofu/withdraw",
		BaofuHTTPTimeout:               5,
		BaofuAccountVerifyFeeFen:       1,
		BaofuBusinessIndustryID:        "industry",
		BaofuMerchantReportChannelID:   "channel",
		BaofuMerchantReportChannelName: "channel-name",
		BaofuMerchantReportBusiness:    "business",
	}
}

const testPrivateKeyPEM = `-----BEGIN RSA PRIVATE KEY-----
MIICXAIBAAKBgQC6PXUPj+GNjCRrKi9mggCmyUvFLpXIlfAYSKw2xo02L9WX0yyD
97PkAIQnIxsY4E6bg/Qm2LHDMXJtXKHga7yLKIKrwq6DzRL+7dUuTWqFQvpX6Vqy
Fb2tvVZnaI07uvdXf5mKMg1d2wo90z5n7p2tH/AqI49pHoN/g4pSAEhNjwIDAQAB
AoGAC3k/K8hW3q9XIV5qK9ZAKzZMLX7C6rt/IaTOofzY47vzbO7Cz2vwqS+ZKZH1
u+RRe97ygx+sLp+5K7DGkd+KtMll95r10SZlAzgkQBxN+8kkxbHT1/5Hf7DP4HOx
wP7B4/hMUZUU2WpNEg1QPzvqs6E5cFjqxO9tZKX2GDM0ZYECQQDsm0iJyqtYXJms
jv1LZfVDrm7t4uHGGwGwYlAuhcyrmaeNLwRUKaKzBT+Oa1vT41OP87Uy+px9DVai
jdhmbBTBAkEAyULzI4g1Pfxkl5gPG7T9xaqPiT9/urhqTX3K+YdLzNGRtBvS1oy/
PWWyPb77VuAL6GdoRhjhjw7Q3ZuR1EyGLwJBAMnuWzTAcBX4u/Ex8N0j+E7IBvfz
1MaI8OCqKPobWkUsrrdmkjviYrlwM5Ji+YPHFdFQ1EXl5kS9cTQAXe2IlgECQHfT
ykK7Eh6qQC0RU/7E98RpdcyFCGBcfU5WaCbP8qTMLyyVMs3HJub1gQyn+lss3PqD
CH9rsesF5mxoRuzTJ9UCQGYFYhqHWNcSUeEXN2iMBvLZvBWO1Nykrk81e39z3/HM
9v3KhcxxCiB26kXTpcvq+QZBKS1wXn3V+8fLtaJz4oY=
-----END RSA PRIVATE KEY-----`

const testPublicKeyPEM = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQC6PXUPj+GNjCRrKi9mggCmyUvF
LpXIlfAYSKw2xo02L9WX0yyD97PkAIQnIxsY4E6bg/Qm2LHDMXJtXKHga7yLKIKr
wq6DzRL+7dUuTWqFQvpX6VqyFb2tvVZnaI07uvdXf5mKMg1d2wo90z5n7p2tH/Aq
I49pHoN/g4pSAEhNjwIDAQAB
-----END PUBLIC KEY-----`

func writeFixture(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
}

type fakeFixtureClaimer struct {
	application          db.ExternalPaymentFactApplication
	applicationErr       error
	outbox               db.PaymentDomainOutbox
	outboxErr            error
	claimedApplicationID int64
	claimedOutboxID      int64
}

func (claimer *fakeFixtureClaimer) ClaimExternalPaymentFactApplication(ctx context.Context, id int64) (db.ExternalPaymentFactApplication, error) {
	claimer.claimedApplicationID = id
	return claimer.application, claimer.applicationErr
}

func (claimer *fakeFixtureClaimer) ClaimPaymentDomainOutbox(ctx context.Context, arg db.ClaimPaymentDomainOutboxParams) (db.PaymentDomainOutbox, error) {
	claimer.claimedOutboxID = arg.ID
	if !arg.NowAt.Valid || arg.NowAt.Time.IsZero() {
		return db.PaymentDomainOutbox{}, errors.New("missing now_at")
	}
	return claimer.outbox, claimer.outboxErr
}
