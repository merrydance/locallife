package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"unicode"
)

type baselineManifest struct {
	DatasetName string               `json:"dataset_name"`
	Version     string               `json:"version"`
	Samples     []baselineSampleSpec `json:"samples"`
}

type baselineSampleSpec struct {
	SampleID       string            `json:"sample_id"`
	DocumentType   string            `json:"document_type"`
	OwnerType      string            `json:"owner_type"`
	Side           string            `json:"side,omitempty"`
	Scenario       string            `json:"scenario,omitempty"`
	Sensitivity    string            `json:"sensitivity,omitempty"`
	ExpectedFields map[string]string `json:"expected_fields"`
}

type baselineRunReport struct {
	Provider      string                 `json:"provider"`
	GeneratedAt   string                 `json:"generated_at"`
	QueueSnapshot baselineQueueSnapshot  `json:"queue_snapshot"`
	Samples       []baselineSampleResult `json:"samples"`
}

type baselineQueueSnapshot struct {
	Pending    int `json:"pending"`
	Processing int `json:"processing"`
}

type baselineSampleResult struct {
	SampleID         string            `json:"sample_id"`
	Status           string            `json:"status"`
	ErrorCode        string            `json:"error_code,omitempty"`
	AttemptCount     int               `json:"attempt_count"`
	LatencyMS        int64             `json:"latency_ms"`
	RecognizedFields map[string]string `json:"recognized_fields"`
}

type baselineSummary struct {
	DatasetName             string                         `json:"dataset_name"`
	DatasetVersion          string                         `json:"dataset_version"`
	Provider                string                         `json:"provider"`
	GeneratedAt             string                         `json:"generated_at"`
	TotalSamples            int                            `json:"total_samples"`
	CompletedSamples        int                            `json:"completed_samples"`
	SucceededSamples        int                            `json:"succeeded_samples"`
	MissingSamples          int                            `json:"missing_samples"`
	SuccessRate             float64                        `json:"success_rate"`
	FieldAccuracy           float64                        `json:"field_accuracy"`
	RetryVolume             int                            `json:"retry_volume"`
	BacklogCount            int                            `json:"backlog_count"`
	LatencyMS               latencySummary                 `json:"latency_ms"`
	ErrorCodeDistribution   map[string]int                 `json:"error_code_distribution"`
	PerDocumentType         map[string]documentTypeSummary `json:"per_document_type"`
	MissingSampleIDs        []string                       `json:"missing_sample_ids"`
	EvaluatedSampleIDs      []string                       `json:"evaluated_sample_ids"`
	ComparisonNormalization string                         `json:"comparison_normalization"`
}

type latencySummary struct {
	P50 int64 `json:"p50"`
	P95 int64 `json:"p95"`
	P99 int64 `json:"p99"`
}

type documentTypeSummary struct {
	TotalSamples     int     `json:"total_samples"`
	SucceededSamples int     `json:"succeeded_samples"`
	SuccessRate      float64 `json:"success_rate"`
	FieldAccuracy    float64 `json:"field_accuracy"`
	RetryVolume      int     `json:"retry_volume"`
}

type aggregate struct {
	totalSamples     int
	succeededSamples int
	completedSamples int
	matchedFields    int
	expectedFields   int
	retryVolume      int
	latencies        []int64
}

func main() {
	manifestPath := flag.String("manifest", "", "path to OCR baseline manifest JSON")
	runPath := flag.String("run", "", "path to OCR baseline run report JSON")
	outPath := flag.String("out", "", "optional output path for summary JSON")
	flag.Parse()

	if err := run(*manifestPath, *runPath, *outPath, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "ocr_baseline_eval: %v\n", err)
		os.Exit(1)
	}
}

func run(manifestPath string, runPath string, outPath string, stdout io.Writer) error {
	if strings.TrimSpace(manifestPath) == "" || strings.TrimSpace(runPath) == "" {
		return errors.New("manifest and run flags are required")
	}

	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return err
	}
	report, err := loadRunReport(runPath)
	if err != nil {
		return err
	}
	summary, err := summarize(manifest, report)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal summary: %w", err)
	}
	encoded = append(encoded, '\n')
	if strings.TrimSpace(outPath) != "" {
		if err := os.WriteFile(outPath, encoded, 0o644); err != nil {
			return fmt.Errorf("write summary: %w", err)
		}
		return nil
	}
	_, err = stdout.Write(encoded)
	return err
}

func loadManifest(path string) (baselineManifest, error) {
	var manifest baselineManifest
	content, err := os.ReadFile(path)
	if err != nil {
		return manifest, fmt.Errorf("read manifest: %w", err)
	}
	if err := json.Unmarshal(content, &manifest); err != nil {
		return manifest, fmt.Errorf("decode manifest: %w", err)
	}
	if strings.TrimSpace(manifest.DatasetName) == "" {
		return manifest, errors.New("manifest dataset_name is required")
	}
	if len(manifest.Samples) == 0 {
		return manifest, errors.New("manifest samples is required")
	}
	return manifest, nil
}

func loadRunReport(path string) (baselineRunReport, error) {
	var report baselineRunReport
	content, err := os.ReadFile(path)
	if err != nil {
		return report, fmt.Errorf("read run report: %w", err)
	}
	if err := json.Unmarshal(content, &report); err != nil {
		return report, fmt.Errorf("decode run report: %w", err)
	}
	return report, nil
}

func summarize(manifest baselineManifest, report baselineRunReport) (baselineSummary, error) {
	manifestIndex := make(map[string]baselineSampleSpec, len(manifest.Samples))
	for _, sample := range manifest.Samples {
		if strings.TrimSpace(sample.SampleID) == "" {
			return baselineSummary{}, errors.New("manifest sample_id is required")
		}
		if _, exists := manifestIndex[sample.SampleID]; exists {
			return baselineSummary{}, fmt.Errorf("duplicate manifest sample_id: %s", sample.SampleID)
		}
		manifestIndex[sample.SampleID] = sample
	}

	reportIndex := make(map[string]baselineSampleResult, len(report.Samples))
	for _, sample := range report.Samples {
		if strings.TrimSpace(sample.SampleID) == "" {
			return baselineSummary{}, errors.New("run report sample_id is required")
		}
		if _, exists := reportIndex[sample.SampleID]; exists {
			return baselineSummary{}, fmt.Errorf("duplicate run report sample_id: %s", sample.SampleID)
		}
		reportIndex[sample.SampleID] = sample
	}

	overall := aggregate{}
	perDocument := map[string]*aggregate{}
	missingSampleIDs := make([]string, 0)
	evaluatedSampleIDs := make([]string, 0, len(manifest.Samples))
	errorCodeDistribution := map[string]int{}

	for _, sample := range manifest.Samples {
		docAggregate := perDocument[sample.DocumentType]
		if docAggregate == nil {
			docAggregate = &aggregate{}
			perDocument[sample.DocumentType] = docAggregate
		}
		overall.totalSamples++
		docAggregate.totalSamples++

		result, ok := reportIndex[sample.SampleID]
		if !ok {
			missingSampleIDs = append(missingSampleIDs, sample.SampleID)
			continue
		}
		evaluatedSampleIDs = append(evaluatedSampleIDs, sample.SampleID)

		if isCompletedStatus(result.Status) {
			overall.completedSamples++
			docAggregate.completedSamples++
			if result.LatencyMS > 0 {
				overall.latencies = append(overall.latencies, result.LatencyMS)
				docAggregate.latencies = append(docAggregate.latencies, result.LatencyMS)
			}
		}
		if result.Status == "succeeded" {
			overall.succeededSamples++
			docAggregate.succeededSamples++
		}
		if result.AttemptCount > 1 {
			retries := result.AttemptCount - 1
			overall.retryVolume += retries
			docAggregate.retryVolume += retries
		}
		if strings.TrimSpace(result.ErrorCode) != "" {
			errorCodeDistribution[result.ErrorCode]++
		}

		expectedFieldCount := len(sample.ExpectedFields)
		matchedFieldCount := 0
		for fieldName, expectedValue := range sample.ExpectedFields {
			if compareValues(expectedValue, result.RecognizedFields[fieldName]) {
				matchedFieldCount++
			}
		}
		overall.expectedFields += expectedFieldCount
		overall.matchedFields += matchedFieldCount
		docAggregate.expectedFields += expectedFieldCount
		docAggregate.matchedFields += matchedFieldCount
	}

	sort.Strings(missingSampleIDs)
	sort.Strings(evaluatedSampleIDs)

	perDocumentSummary := make(map[string]documentTypeSummary, len(perDocument))
	for documentType, agg := range perDocument {
		perDocumentSummary[documentType] = documentTypeSummary{
			TotalSamples:     agg.totalSamples,
			SucceededSamples: agg.succeededSamples,
			SuccessRate:      rate(agg.succeededSamples, agg.totalSamples),
			FieldAccuracy:    rate(agg.matchedFields, agg.expectedFields),
			RetryVolume:      agg.retryVolume,
		}
	}

	return baselineSummary{
		DatasetName:             manifest.DatasetName,
		DatasetVersion:          manifest.Version,
		Provider:                report.Provider,
		GeneratedAt:             report.GeneratedAt,
		TotalSamples:            overall.totalSamples,
		CompletedSamples:        overall.completedSamples,
		SucceededSamples:        overall.succeededSamples,
		MissingSamples:          len(missingSampleIDs),
		SuccessRate:             rate(overall.succeededSamples, overall.totalSamples),
		FieldAccuracy:           rate(overall.matchedFields, overall.expectedFields),
		RetryVolume:             overall.retryVolume,
		BacklogCount:            report.QueueSnapshot.Pending + report.QueueSnapshot.Processing,
		LatencyMS:               latencySummary{P50: percentile(overall.latencies, 0.50), P95: percentile(overall.latencies, 0.95), P99: percentile(overall.latencies, 0.99)},
		ErrorCodeDistribution:   errorCodeDistribution,
		PerDocumentType:         perDocumentSummary,
		MissingSampleIDs:        missingSampleIDs,
		EvaluatedSampleIDs:      evaluatedSampleIDs,
		ComparisonNormalization: "lowercase + trim spaces + remove punctuation and unicode separators before field comparison",
	}, nil
}

func isCompletedStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "succeeded", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func compareValues(expected string, actual string) bool {
	return normalizeValue(expected) == normalizeValue(actual)
}

func normalizeValue(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	for _, r := range value {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

func rate(numerator int, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return math.Round((float64(numerator)/float64(denominator))*10000) / 10000
}

func percentile(values []int64, q float64) int64 {
	if len(values) == 0 {
		return 0
	}
	cloned := append([]int64(nil), values...)
	sort.Slice(cloned, func(i int, j int) bool { return cloned[i] < cloned[j] })
	if q <= 0 {
		return cloned[0]
	}
	if q >= 1 {
		return cloned[len(cloned)-1]
	}
	index := int(math.Ceil(q*float64(len(cloned)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(cloned) {
		index = len(cloned) - 1
	}
	return cloned[index]
}
