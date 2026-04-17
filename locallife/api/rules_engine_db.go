package api

import (
	"context"
	"encoding/json"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/rules"
)

// DBRulesEngine evaluates rules stored in database.
type DBRulesEngine struct {
	store db.Store
}

// NewDBRulesEngine creates a DB-backed rules engine.
func NewDBRulesEngine(store db.Store) *DBRulesEngine {
	return &DBRulesEngine{store: store}
}

// Evaluate executes active rule versions and returns the first matching decision.
func (e *DBRulesEngine) Evaluate(ctx context.Context, input rules.Context) (rules.Decision, error) {
	versions, err := e.store.ListActiveRuleVersions(ctx)
	if err != nil {
		return rules.Decision{}, err
	}

	for _, version := range versions {
		scope := map[string]interface{}{}
		condition := map[string]interface{}{}
		action := map[string]interface{}{}
		grayConfig := map[string]interface{}{}
		_ = json.Unmarshal(version.Scope, &scope)
		_ = json.Unmarshal(version.Condition, &condition)
		_ = json.Unmarshal(version.Action, &action)
		_ = json.Unmarshal(version.GrayConfig, &grayConfig)

		if !matchRuleScope(scope, input) {
			continue
		}

		if !matchRuleGray(grayConfig, input) {
			continue
		}

		ok, err := matchRuleCondition(ctx, e.store, condition, input)
		if err != nil {
			return rules.Decision{}, err
		}
		if !ok {
			continue
		}

		decision := buildDecisionFromAction(action)
		decision.RuleID = version.RuleID
		decision.RuleVersionID = version.ID
		return decision, nil
	}

	return rules.Decision{Allow: true, Action: "allow"}, nil
}

func matchRuleScope(scope map[string]interface{}, input rules.Context) bool {
	if v, ok := scope["domain"]; ok {
		if s, ok := v.(string); ok && s != "" && s != string(input.Domain) {
			return false
		}
	}
	if v, ok := scope["order_type"]; ok {
		if s, ok := v.(string); ok && s != "" && input.OrderType != s {
			return false
		}
	}
	if v, ok := scope["region_id"]; ok {
		if !matchIDScope(v, input.RegionID) {
			return false
		}
	}
	if v, ok := scope["merchant_id"]; ok {
		if !matchIDScope(v, input.MerchantID) {
			return false
		}
	}
	return true
}

func matchRuleGray(grayConfig map[string]interface{}, input rules.Context) bool {
	if len(grayConfig) == 0 {
		return true
	}
	if v, ok := grayConfig["region_id"]; ok {
		if !matchIDScope(v, input.RegionID) {
			return false
		}
	}
	if v, ok := grayConfig["merchant_id"]; ok {
		if !matchIDScope(v, input.MerchantID) {
			return false
		}
	}
	if v, ok := grayConfig["user_id"]; ok {
		if !matchIDScope(v, input.UserID) {
			return false
		}
	}
	return true
}

func matchIDScope(value interface{}, target int64) bool {
	if target == 0 {
		return false
	}
	switch v := value.(type) {
	case float64:
		return int64(v) == target
	case int64:
		return v == target
	case []interface{}:
		for _, item := range v {
			if f, ok := item.(float64); ok && int64(f) == target {
				return true
			}
			if i, ok := item.(int64); ok && i == target {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func matchRuleCondition(ctx context.Context, store db.Store, condition map[string]interface{}, input rules.Context) (bool, error) {
	if v, ok := condition["behavior_blocklist"]; ok {
		if enabled, ok := v.(bool); ok && enabled {
			blocked, err := checkActiveBehaviorBlocklist(ctx, store, input.UserID)
			if err != nil {
				return false, err
			}
			if !blocked {
				return false, nil
			}
		}
	}
	if v, ok := condition["claim_type"]; ok {
		if s, ok := v.(string); ok && s != "" {
			if metaString(input.Metadata, "claim_type") != s {
				return false, nil
			}
		}
	}
	if v, ok := condition["claim_type_in"]; ok {
		if !matchStringSlice(v, metaString(input.Metadata, "claim_type")) {
			return false, nil
		}
	}
	if v, ok := condition["amount_gte"]; ok {
		if !matchNumberGte(v, float64(input.Amount)) {
			return false, nil
		}
	}
	if v, ok := condition["amount_lte"]; ok {
		if !matchNumberLte(v, float64(input.Amount)) {
			return false, nil
		}
	}
	if v, ok := condition["claims_7d_gte"]; ok {
		if !matchNumberGte(v, metaNumber(input.Metadata, "claims_7d")) {
			return false, nil
		}
	}
	if v, ok := condition["claims_30d_gte"]; ok {
		if !matchNumberGte(v, metaNumber(input.Metadata, "claims_30d")) {
			return false, nil
		}
	}
	if v, ok := condition["claim_rate_7d_gte"]; ok {
		if !matchNumberGte(v, metaNumber(input.Metadata, "claim_rate_7d")) {
			return false, nil
		}
	}
	if v, ok := condition["claim_rate_30d_gte"]; ok {
		if !matchNumberGte(v, metaNumber(input.Metadata, "claim_rate_30d")) {
			return false, nil
		}
	}
	if v, ok := condition["user_claims_7d_exceeded"]; ok {
		if want, ok := v.(bool); ok {
			if want != boolMeta(input.Metadata, "user_claims_7d_exceeded") {
				return false, nil
			}
		}
	}
	if v, ok := condition["user_claims_30d_exceeded"]; ok {
		if want, ok := v.(bool); ok {
			if want != boolMeta(input.Metadata, "user_claims_30d_exceeded") {
				return false, nil
			}
		}
	}
	if v, ok := condition["user_claim_rate_7d_exceeded"]; ok {
		if want, ok := v.(bool); ok {
			if want != boolMeta(input.Metadata, "user_claim_rate_7d_exceeded") {
				return false, nil
			}
		}
	}
	if v, ok := condition["user_claim_rate_30d_exceeded"]; ok {
		if want, ok := v.(bool); ok {
			if want != boolMeta(input.Metadata, "user_claim_rate_30d_exceeded") {
				return false, nil
			}
		}
	}
	if v, ok := condition["merchant_abnormal_rate_30d_exceeded"]; ok {
		if want, ok := v.(bool); ok {
			if want != boolMeta(input.Metadata, "merchant_abnormal_rate_30d_exceeded") {
				return false, nil
			}
		}
	}
	if v, ok := condition["rider_abnormal_rate_30d_exceeded"]; ok {
		if want, ok := v.(bool); ok {
			if want != boolMeta(input.Metadata, "rider_abnormal_rate_30d_exceeded") {
				return false, nil
			}
		}
	}
	if v, ok := condition["use_balance"]; ok {
		if enabled, ok := v.(bool); ok && enabled {
			useBalance, ok := input.Metadata["use_balance"].(bool)
			if !ok || !useBalance {
				return false, nil
			}
		}
	}
	if v, ok := condition["balance_scene_allowed"]; ok {
		allowed, err := checkBalanceSceneAllowed(ctx, store, input.MerchantID, input.OrderType)
		if err != nil {
			return false, err
		}
		if want, ok := v.(bool); ok {
			if want != allowed {
				return false, nil
			}
		}
	}
	if v, ok := condition["health_cert_uploaded"]; ok {
		if want, ok := v.(bool); ok {
			if want != boolMeta(input.Metadata, "health_cert_uploaded") {
				return false, nil
			}
		}
	}
	if v, ok := condition["idcard_ocr_valid"]; ok {
		if want, ok := v.(bool); ok {
			if want != boolMeta(input.Metadata, "idcard_ocr_valid") {
				return false, nil
			}
		}
	}
	if v, ok := condition["health_ocr_valid"]; ok {
		if want, ok := v.(bool); ok {
			if want != boolMeta(input.Metadata, "health_ocr_valid") {
				return false, nil
			}
		}
	}
	if v, ok := condition["idcard_not_expired"]; ok {
		if want, ok := v.(bool); ok {
			if want != boolMeta(input.Metadata, "idcard_not_expired") {
				return false, nil
			}
		}
	}
	if v, ok := condition["name_match"]; ok {
		if want, ok := v.(bool); ok {
			if want != boolMeta(input.Metadata, "name_match") {
				return false, nil
			}
		}
	}
	return true, nil
}

func boolMeta(meta map[string]interface{}, key string) bool {
	if meta == nil {
		return false
	}
	if v, ok := meta[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func metaString(meta map[string]interface{}, key string) string {
	if meta == nil {
		return ""
	}
	if v, ok := meta[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func metaNumber(meta map[string]interface{}, key string) float64 {
	if meta == nil {
		return 0
	}
	if v, ok := meta[key]; ok {
		switch n := v.(type) {
		case float64:
			return n
		case float32:
			return float64(n)
		case int:
			return float64(n)
		case int64:
			return float64(n)
		case int32:
			return float64(n)
		case uint:
			return float64(n)
		case uint64:
			return float64(n)
		case uint32:
			return float64(n)
		}
	}
	return 0
}

func matchNumberGte(value interface{}, actual float64) bool {
	want, ok := toFloat64(value)
	if !ok {
		return true
	}
	return actual >= want
}

func matchNumberLte(value interface{}, actual float64) bool {
	want, ok := toFloat64(value)
	if !ok {
		return true
	}
	return actual <= want
}

func toFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint64:
		return float64(v), true
	case uint32:
		return float64(v), true
	default:
		return 0, false
	}
}

func matchStringSlice(value interface{}, target string) bool {
	if target == "" {
		return false
	}
	switch v := value.(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok && s == target {
				return true
			}
		}
		return false
	case []string:
		for _, item := range v {
			if item == target {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func checkActiveBehaviorBlocklist(ctx context.Context, store db.Store, userID int64) (bool, error) {
	if userID == 0 {
		return false, nil
	}
	block, err := store.GetActiveBehaviorBlocklist(ctx, db.GetActiveBehaviorBlocklistParams{
		EntityType: "user",
		EntityID:   userID,
	})
	if err != nil {
		if isNotFoundError(err) || err == db.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}
	if block.BlockUntil.Valid && time.Now().After(block.BlockUntil.Time) {
		_ = store.UpdateBehaviorBlocklistStatus(ctx, db.UpdateBehaviorBlocklistStatusParams{
			ID:     block.ID,
			Status: "expired",
		})
		return false, nil
	}
	return true, nil
}

func checkBalanceSceneAllowed(ctx context.Context, store db.Store, merchantID int64, orderType string) (bool, error) {
	if merchantID == 0 || orderType == "" {
		return false, nil
	}
	if !logic.IsMembershipBalanceSupportedOrderType(orderType) {
		return false, nil
	}
	settings, err := store.GetMerchantMembershipSettings(ctx, merchantID)
	if err != nil {
		if isNotFoundError(err) || err == db.ErrRecordNotFound {
			// 未配置时默认允许堂食/自提
			return true, nil
		}
		return false, err
	}
	for _, scene := range settings.BalanceUsableScenes {
		if !logic.IsMembershipBalanceSupportedOrderType(scene) {
			continue
		}
		if scene == orderType {
			return true, nil
		}
	}
	return false, nil
}

func buildDecisionFromAction(action map[string]interface{}) rules.Decision {
	typeValue, _ := action["type"].(string)
	reason, _ := action["reason"].(string)
	if reason == "" {
		reason, _ = action["message"].(string)
	}
	meta, _ := action["meta"].(map[string]interface{})

	decision := rules.Decision{
		Allow:  true,
		Action: "allow",
		Reason: reason,
		Meta:   meta,
	}
	if typeValue != "" {
		decision.Action = typeValue
	}
	if typeValue == "deny" {
		decision.Allow = false
	}
	return decision
}
