package money

import (
	"errors"
	"math/big"
)

var (
	// ErrInvalidRate reports a rate that cannot produce a meaningful amount.
	ErrInvalidRate = errors.New("exchange rate must be a positive finite value")
	// ErrAmountOverflow reports a converted result that cannot be represented in the
	// ledger's signed 64-bit minor-unit type.
	ErrAmountOverflow = errors.New("converted amount exceeds int64 minor-unit range")
)

// Convert converts an amount in minor units using an exact rational rate, rounding once at
// the end with ties away from zero.
//
// Every supported currency shares the same exponent, so a minor-unit amount converts by
// multiplying by the rate alone. Rate arrives as a *big.Rat rather than a float so the
// provider's decimal value survives untouched; float64 cannot represent a rate such as
// 0.02088 exactly, and rounding a already-rounded product is how money drifts.
func Convert(amountMinor int64, rate *big.Rat) (int64, error) {
	if rate == nil || rate.Sign() <= 0 {
		return 0, ErrInvalidRate
	}
	product := new(big.Rat).Mul(new(big.Rat).SetInt64(amountMinor), rate)
	return roundRat(product)
}

// roundRat returns the nearest integer to value, rounding halves away from zero so that a
// converted debit and its matching credit round symmetrically.
func roundRat(value *big.Rat) (int64, error) {
	quotient, remainder := new(big.Int).QuoRem(value.Num(), value.Denom(), new(big.Int))
	// QuoRem truncates toward zero, so remainder carries the sign of the numerator.
	doubled := new(big.Int).Abs(remainder)
	doubled.Lsh(doubled, 1)
	if doubled.Cmp(value.Denom()) >= 0 {
		if value.Sign() < 0 {
			quotient.Sub(quotient, big.NewInt(1))
		} else {
			quotient.Add(quotient, big.NewInt(1))
		}
	}
	if !quotient.IsInt64() {
		return 0, ErrAmountOverflow
	}
	return quotient.Int64(), nil
}
