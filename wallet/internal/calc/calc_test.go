package calc

import "testing"

// Worked example from docs/plans/day-zero-wallet.md §1:
// D=10,000 TRY (1,000,000 kuruş), r=0.12, n=12, t=1, m=0.10
// → Yp = 1,268.25 TRY (126,825 kuruş) → L0 = 1,141.42 TRY (114,142 kuruş).
func TestDayZeroWorkedExample(t *testing.T) {
	limit, yield := DayZero(1_000_000, 0.12, 12, 1, 0.10)

	if wantYield := int64(126_825); yield != wantYield {
		t.Errorf("yield = %d kuruş, want %d", yield, wantYield)
	}
	if wantLimit := int64(114_142); limit != wantLimit {
		t.Errorf("limit = %d kuruş, want %d", limit, wantLimit)
	}
}

func TestDayZeroFloorsDown(t *testing.T) {
	// Limit must never exceed yield·(1−m); flooring guarantees no over-grant.
	limit, yield := DayZero(1_000_000, 0.12, 12, 1, 0.10)
	if float64(limit) > float64(yield)*0.90 {
		t.Errorf("limit %d exceeds floored yield·0.90", limit)
	}
}
