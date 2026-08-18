package money

import (
	"errors"
	"math/big"
	"testing"
)

func rate(t *testing.T, value string) *big.Rat {
	t.Helper()
	parsed, ok := new(big.Rat).SetString(value)
	if !ok {
		t.Fatalf("SetString(%q) failed", value)
	}
	return parsed
}

func TestConvert(t *testing.T) {
	tests := []struct {
		name        string
		amountMinor int64
		rate        string
		want        int64
	}{
		// 50,000.00 TRY at the published TRY->USD rate, the case ADR 0005 describes.
		{name: "budget to usd", amountMinor: 5000000, rate: "0.02088", want: 104400},
		{name: "zero stays zero", amountMinor: 0, rate: "0.02088", want: 0},
		{name: "negative amount", amountMinor: -35000, rate: "0.02088", want: -731},
		{name: "identity rate", amountMinor: 123456, rate: "1", want: 123456},
		// Exact halves round away from zero so a debit and its matching credit stay symmetric.
		{name: "positive tie rounds up", amountMinor: 1, rate: "0.5", want: 1},
		{name: "negative tie rounds down", amountMinor: -1, rate: "0.5", want: -1},
		{name: "just below tie rounds toward zero", amountMinor: 1, rate: "0.49", want: 0},
		{name: "just above tie rounds away", amountMinor: 1, rate: "0.51", want: 1},
		// A rate float64 cannot represent exactly; the result must not drift.
		{name: "repeating decimal rate", amountMinor: 300, rate: "1/3", want: 100},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Convert(test.amountMinor, rate(t, test.rate))
			if err != nil {
				t.Fatalf("Convert() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("Convert(%d, %s) = %d, want %d", test.amountMinor, test.rate, got, test.want)
			}
		})
	}
}

func TestConvertRejectsUnusableRate(t *testing.T) {
	for _, value := range []*big.Rat{nil, new(big.Rat), rate(t, "-1.5")} {
		if _, err := Convert(1000, value); !errors.Is(err, ErrInvalidRate) {
			t.Fatalf("Convert() error = %v, want ErrInvalidRate", err)
		}
	}
}

func TestConvertRejectsInt64Overflow(t *testing.T) {
	for _, amount := range []int64{int64(^uint64(0) >> 1), -int64(^uint64(0)>>1) - 1} {
		if _, err := Convert(amount, rate(t, "2")); !errors.Is(err, ErrAmountOverflow) {
			t.Fatalf("Convert(%d, 2) error = %v, want ErrAmountOverflow", amount, err)
		}
	}
}

func TestParseRejectsUnsupportedCurrency(t *testing.T) {
	if _, ok := Parse(" usd "); !ok {
		t.Fatal("Parse() rejected a supported currency")
	}
	for _, value := range []string{"XYZ", "", "US", "USDD", "GBP"} {
		if currency, ok := Parse(value); ok {
			t.Fatalf("Parse(%q) = %q, want rejection", value, currency)
		}
	}
}
