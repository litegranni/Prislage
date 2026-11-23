package notify

import (
	"github.com/shopspring/decimal"

	"prislage/internal/models"
)

// Decide returns which notification types to emit for a newly recorded price.
//
// target_reached: only when price crosses the target (previous > target, new <= target).
// new_low: only when new price is at least 1% below the previous all-time low.
func Decide(prevPrice *decimal.Decimal, prevLow *decimal.Decimal, newPrice, target decimal.Decimal) []models.NotificationType {
	var out []models.NotificationType

	if prevPrice != nil && prevPrice.GreaterThan(target) && !newPrice.GreaterThan(target) {
		out = append(out, models.NotifyTargetReached)
	}

	if prevLow != nil {
		// new <= prevLow * 0.99
		threshold := prevLow.Mul(decimal.NewFromFloat(0.99))
		if !newPrice.GreaterThan(threshold) && newPrice.LessThan(*prevLow) {
			out = append(out, models.NotifyNewLow)
		}
	}

	return out
}
