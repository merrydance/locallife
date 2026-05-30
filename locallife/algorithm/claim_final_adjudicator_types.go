package algorithm

const (
	FinalDecisionMerchantRecovery = DecisionModeMerchantRecovery
	FinalDecisionRiderRecovery    = DecisionModeRiderRecovery
	FinalDecisionUserRestricted   = DecisionModeUserRestricted
)

type ClaimFinalAdjudicatorConfig struct {
	MinUserOrders30d        int32
	MinUserClaims30d        int32
	UserClaimRate30d        float64
	UserClaims7d            int32
	UserClaimRate7d         float64
	MinRiderOrders30d       int32
	RiderAbnormalRate30d    float64
	MinMerchantOrders30d    int32
	MerchantAbnormalRate30d float64
}

func DefaultClaimFinalAdjudicatorConfig() ClaimFinalAdjudicatorConfig {
	return ClaimFinalAdjudicatorConfig{
		MinUserOrders30d:        5,
		MinUserClaims30d:        3,
		UserClaimRate30d:        0.5,
		UserClaims7d:            3,
		UserClaimRate7d:         0.5,
		MinRiderOrders30d:       10,
		RiderAbnormalRate30d:    0.06,
		MinMerchantOrders30d:    10,
		MerchantAbnormalRate30d: 0.08,
	}
}

type PartyWindowStats struct {
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`

	TotalOrders7d     int32 `json:"total_orders_7d"`
	AbnormalClaims7d  int32 `json:"abnormal_claims_7d"`
	TotalOrders30d    int32 `json:"total_orders_30d"`
	AbnormalClaims30d int32 `json:"abnormal_claims_30d"`

	MaliciousConfirmedClaims int64 `json:"malicious_confirmed_claims"`
	SharedDeviceOtherUsers   int32 `json:"shared_device_other_users"`
	SharedAddressOtherUsers  int32 `json:"shared_address_other_users"`
	NetAbnormalClaims30d     int32 `json:"net_abnormal_claims_30d"`
}

type ClaimFinalAdjudicationInput struct {
	RegionID  int64
	ClaimType string
	User      PartyWindowStats
	Rider     *PartyWindowStats
	Merchant  PartyWindowStats
}

type ClaimFinalAdjudicationResult struct {
	DecisionMode         string
	ResponsibleParty     string
	CompensationSource   string
	BehaviorStatus       string
	BaseResponsibleParty string
	Reason               string
	ReasonCodes          []string
	ScoreBreakdown       ClaimFinalScoreBreakdown
}

type ClaimFinalScoreBreakdown struct {
	Version              string               `json:"version"`
	RegionID             int64                `json:"region_id,omitempty"`
	ClaimType            string               `json:"claim_type"`
	BaseResponsibleParty string               `json:"base_responsible_party"`
	FinalDecisionMode    string               `json:"final_decision_mode"`
	Scores               ClaimFinalScores     `json:"scores"`
	Thresholds           ClaimFinalThresholds `json:"thresholds"`
}

type ClaimFinalScores struct {
	UserRisk          ClaimFinalScoreDetail `json:"user_risk"`
	RiderLiability    ClaimFinalScoreDetail `json:"rider_liability"`
	MerchantLiability ClaimFinalScoreDetail `json:"merchant_liability"`
	Confidence        ClaimFinalScoreDetail `json:"confidence"`
}

type ClaimFinalScoreDetail struct {
	Score   int32                   `json:"score"`
	Level   string                  `json:"level"`
	Signals []ClaimFinalScoreSignal `json:"signals,omitempty"`
}

type ClaimFinalScoreSignal struct {
	Code    string `json:"code"`
	Weight  int32  `json:"weight"`
	Message string `json:"message"`
}

type ClaimFinalThresholds struct {
	MinUserOrders30d        int32   `json:"min_user_orders_30d"`
	MinUserClaims30d        int32   `json:"min_user_claims_30d"`
	UserClaimRate30d        float64 `json:"user_claim_rate_30d"`
	UserClaims7d            int32   `json:"user_claims_7d"`
	UserClaimRate7d         float64 `json:"user_claim_rate_7d"`
	MinRiderOrders30d       int32   `json:"min_rider_orders_30d"`
	RiderAbnormalRate30d    float64 `json:"rider_abnormal_rate_30d"`
	MinMerchantOrders30d    int32   `json:"min_merchant_orders_30d"`
	MerchantAbnormalRate30d float64 `json:"merchant_abnormal_rate_30d"`
}
