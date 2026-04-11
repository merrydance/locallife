package algorithm

// Fraud pattern types
const (
	FraudPatternDeviceReuse        = "device-reuse"
	FraudPatternAddressCluster     = "address-cluster"
	FraudPatternCoordinatedClaims  = "coordinated-claims"
)

// FraudDetectionResult 欺诈检测结果
type FraudDetectionResult struct {
	IsFraud           bool    `json:"is_fraud"`
	PatternType       string  `json:"pattern_type"`
	Confidence        int     `json:"confidence"`
	RelatedUserIDs    []int64 `json:"related_user_ids,omitempty"`
	RelatedClaimIDs   []int64 `json:"related_claim_ids,omitempty"`
	Description       string  `json:"description,omitempty"`
	ShouldBlock       bool    `json:"should_block,omitempty"`
	MerchantSuspect   bool    `json:"merchant_suspect,omitempty"`
	SuspectMerchantID int64   `json:"suspect_merchant_id,omitempty"`
}