package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSummarize(t *testing.T) {
	manifest := baselineManifest{
		DatasetName: "ocr-regression-v1",
		Version:     "2026-03-25",
		Samples: []baselineSampleSpec{
			{SampleID: "bl-1", DocumentType: "business_license", ExpectedFields: map[string]string{"credit_code": "91310000123456789A", "company_name": "示例餐饮有限公司"}},
			{SampleID: "id-1", DocumentType: "id_card", ExpectedFields: map[string]string{"name": "张三", "id_number": "110101199001011234"}},
			{SampleID: "hp-1", DocumentType: "health_cert", ExpectedFields: map[string]string{"holder_name": "李四"}},
		},
	}
	report := baselineRunReport{
		Provider:    "aliyun",
		GeneratedAt: "2026-03-25T19:00:00Z",
		QueueSnapshot: baselineQueueSnapshot{
			Pending:    2,
			Processing: 1,
		},
		Samples: []baselineSampleResult{
			{SampleID: "bl-1", Status: "succeeded", AttemptCount: 1, LatencyMS: 800, RecognizedFields: map[string]string{"credit_code": "91310000123456789A", "company_name": "示例餐饮有限公司"}},
			{SampleID: "id-1", Status: "failed", ErrorCode: "ocr_provider_forbidden", AttemptCount: 1, LatencyMS: 300, RecognizedFields: map[string]string{}},
			{SampleID: "hp-1", Status: "succeeded", AttemptCount: 2, LatencyMS: 1200, RecognizedFields: map[string]string{"holder_name": "李 五"}},
		},
	}

	summary, err := summarize(manifest, report)
	require.NoError(t, err)
	require.Equal(t, 3, summary.TotalSamples)
	require.Equal(t, 2, summary.SucceededSamples)
	require.Equal(t, 0.6667, summary.SuccessRate)
	require.Equal(t, 0.4, summary.FieldAccuracy)
	require.Equal(t, 1, summary.RetryVolume)
	require.Equal(t, 3, summary.BacklogCount)
	require.Equal(t, int64(800), summary.LatencyMS.P50)
	require.Equal(t, int64(1200), summary.LatencyMS.P95)
	require.Equal(t, 1, summary.ErrorCodeDistribution["ocr_provider_forbidden"])
	require.Equal(t, 1.0, summary.PerDocumentType["business_license"].SuccessRate)
	require.Equal(t, 0.0, summary.PerDocumentType["id_card"].SuccessRate)
	require.Equal(t, 1.0, summary.PerDocumentType["health_cert"].SuccessRate)
	require.Equal(t, []string{"bl-1", "hp-1", "id-1"}, summary.EvaluatedSampleIDs)
}

func TestNormalizeValue(t *testing.T) {
	require.Equal(t, normalizeValue("张 三-001"), normalizeValue("张三001"))
	require.Equal(t, normalizeValue("9131 0000 1234"), normalizeValue("913100001234"))
	if compareValues("示例公司", "另一家公司") {
		t.Fatal("expected different values to remain different after normalization")
	}
}
