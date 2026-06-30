// Package calc implements the Day-Zero Yield math. Pure functions, no I/O.
//
//	FV = D · (1 + r/n)^(n·t)
//	Yp = D · [ (1 + r/n)^(n·t) − 1 ]
//	L0 = Yp · (1 − m)
//
// All money is integer minor units (kuruş, ADR-0003). Results are floored — never
// grant more credit than the math yields (docs/plans/day-zero-wallet.md §1).
package calc

import "math"

// DayZero returns the spendable Day-Zero credit limit (L0) and the projected yield
// (Yp), both in minor units, floored to whole kuruş.
//
//	depositMinor — D, in kuruş
//	apy          — r (e.g. 0.12)
//	compounding  — n, periods per year
//	lockupYears  — t
//	margin       — m, risk margin (0.10–0.15 fixed pool, ADR-0001)
func DayZero(depositMinor int64, apy float64, compounding, lockupYears uint32, margin float64) (limitMinor, yieldMinor int64) {
	n := float64(compounding)
	t := float64(lockupYears)
	factor := math.Pow(1+apy/n, n*t)
	yieldMinor = int64(math.Floor(float64(depositMinor) * (factor - 1)))
	limitMinor = int64(math.Floor(float64(yieldMinor) * (1 - margin)))
	return limitMinor, yieldMinor
}
