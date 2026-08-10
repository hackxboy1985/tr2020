package common

import (
	"fmt"
	"math"

	"github.com/shopspring/decimal"
)

func GetTrustQuota() int {
	return int(10 * QuotaPerUnit)
}

func YuanToQuota(yuan float64) int {
	return QuotaFromFloat(yuan * QuotaPerUnit)
}

func QuotaToYuan(quota int) float64 {
	return float64(quota) / QuotaPerUnit
}

// QuotaFromFloat converts a float64 quota value to int, with overflow protection.
func QuotaFromFloat(f float64) int {
	if f > float64(math.MaxInt) {
		return math.MaxInt
	}
	if f < float64(math.MinInt) {
		return math.MinInt
	}
	return int(f)
}

// QuotaFromDecimalStrict converts a decimal quota value to int strictly.
// Returns an error if the value overflows int range.
func QuotaFromDecimalStrict(d decimal.Decimal) (int, error) {
	if !d.IsInteger() {
		// Round up for non-integer values
		d = d.Ceil()
	}

	maxInt := decimal.NewFromInt(int64(math.MaxInt))
	minInt := decimal.NewFromInt(int64(math.MinInt))

	if d.GreaterThan(maxInt) {
		return 0, fmt.Errorf("quota value %s exceeds maximum int", d.String())
	}
	if d.LessThan(minInt) {
		return 0, fmt.Errorf("quota value %s below minimum int", d.String())
	}

	i64 := d.IntPart()
	return int(i64), nil
}
