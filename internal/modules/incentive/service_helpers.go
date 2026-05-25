package incentive

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
)

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func validateCommissionPolicy(policy string, fixedAmount, rateBps int64) error {
	switch policy {
	case "fixed_amount":
		if fixedAmount <= 0 {
			return fmt.Errorf("%w: fixed_amount requires positive commission_fixed_amount", ErrInvalidCommissionPolicy)
		}
	case "percentage":
		if rateBps <= 0 {
			return fmt.Errorf("%w: percentage requires positive commission_rate_bps", ErrInvalidCommissionPolicy)
		}
	default:
		return fmt.Errorf("%w: unsupported policy %s", ErrInvalidCommissionPolicy, policy)
	}
	return nil
}

func parseOptionalTime(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func normalizeReferralCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func generateReferralCode() string {
	raw := strings.ToUpper(strings.ReplaceAll(utils.GenerateID(), "-", ""))
	if len(raw) > 10 {
		raw = raw[:10]
	}
	return raw
}

func programEffectiveNow(program models.ReferralProgram, now time.Time) bool {
	if program.EffectiveFrom != nil && now.Before(*program.EffectiveFrom) {
		return false
	}
	if program.EffectiveTo != nil && now.After(*program.EffectiveTo) {
		return false
	}
	return true
}

func calculateCommissionAmount(program models.ReferralProgram, baseAmount int64) int64 {
	switch program.CommissionPolicy {
	case "fixed_amount":
		return maxInt64(program.CommissionFixedAmount, 0)
	case "percentage":
		if baseAmount <= 0 {
			return 0
		}
		return maxInt64(baseAmount*program.CommissionRateBps/10000, 0)
	default:
		return 0
	}
}

func maxInt64(a, b int64) int64 {
	if a >= b {
		return a
	}
	return b
}

func parseMetadata(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return map[string]any{"raw": raw}
	}
	return out
}

func filterRedeemableCommissions(items []models.CommissionLedger, commissionIDs []string) []models.CommissionLedger {
	if len(commissionIDs) == 0 {
		return items
	}
	allowed := make(map[string]struct{}, len(commissionIDs))
	for _, id := range commissionIDs {
		if id != "" {
			allowed[id] = struct{}{}
		}
	}
	out := make([]models.CommissionLedger, 0, len(items))
	for _, item := range items {
		if _, ok := allowed[item.ID]; ok {
			out = append(out, item)
		}
	}
	return out
}

func (s *Service) issueRewardTx(tx *gorm.DB, item *models.RewardLedger) error {
	if err := tx.Create(item).Error; err != nil {
		return err
	}
	if item.Status != "issued" || item.AssetCode == "" || item.Amount <= 0 {
		return nil
	}
	return s.creditRewardToWalletTx(tx, item)
}

func deriveReferralConversionStatus(program models.ReferralProgram, commissionAmount int64) string {
	if commissionAmount <= 0 {
		return "tracked"
	}
	if program.SettlementDelayDays > 0 {
		return "pending_reward"
	}
	return "commission_earned"
}

func deriveCommissionStatus(program models.ReferralProgram, commissionAmount int64) string {
	if commissionAmount <= 0 {
		return "pending"
	}
	if program.SettlementDelayDays > 0 {
		return "pending"
	}
	return "earned"
}

func buildRewardPolicyDesc(program models.ReferralProgram) string {
	switch program.CommissionPolicy {
	case "fixed_amount":
		if program.SettlementDelayDays > 0 {
			return fmt.Sprintf("Complete %s to earn %d %s after %d day(s).", readableTrigger(program.TriggerType), program.CommissionFixedAmount, defaultString(program.CommissionCurrency, "CNY"), program.SettlementDelayDays)
		}
		return fmt.Sprintf("Complete %s to earn %d %s.", readableTrigger(program.TriggerType), program.CommissionFixedAmount, defaultString(program.CommissionCurrency, "CNY"))
	case "percentage":
		rate := float64(program.CommissionRateBps) / 100
		if program.SettlementDelayDays > 0 {
			return fmt.Sprintf("Complete %s to earn %.2f%% commission after %d day(s).", readableTrigger(program.TriggerType), rate, program.SettlementDelayDays)
		}
		return fmt.Sprintf("Complete %s to earn %.2f%% commission.", readableTrigger(program.TriggerType), rate)
	default:
		return fmt.Sprintf("Complete %s to unlock referral rewards.", readableTrigger(program.TriggerType))
	}
}

func readableTrigger(trigger string) string {
	switch trigger {
	case "signup":
		return "signup"
	case "first_paid_order":
		return "the first paid order"
	case "first_subscription":
		return "the first subscription"
	case "usage_settlement":
		return "qualified usage settlement"
	default:
		return trigger
	}
}
