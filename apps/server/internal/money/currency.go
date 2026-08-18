// Package money holds the supported currency set and integer-safe amount handling shared by
// the domain packages. See docs/decisions/0005-supported-currencies-and-display-conversion.md.
package money

import "strings"

// Currency is a supported ISO 4217 code. The set is deliberately small: every member uses
// two minor-unit decimal places, which is what lets Exponent be a constant.
type Currency string

const (
	TRY Currency = "TRY"
	USD Currency = "USD"
	EUR Currency = "EUR"
)

// Exponent is the number of minor-unit decimal places shared by every supported currency.
// Adding a currency with a different exponent requires per-currency metadata first.
const Exponent = 2

// supported is ordered for stable presentation in clients.
var supported = []Currency{TRY, USD, EUR}

// Supported returns the supported currencies in presentation order.
func Supported() []Currency {
	return append([]Currency(nil), supported...)
}

// Valid reports whether the currency is part of the supported set.
func (c Currency) Valid() bool {
	for _, candidate := range supported {
		if c == candidate {
			return true
		}
	}
	return false
}

func (c Currency) String() string {
	return string(c)
}

// Parse normalizes user input and rejects anything outside the supported set. Trimming and
// upper-casing keep "usd" acceptable at the edges without loosening what is stored.
func Parse(value string) (Currency, bool) {
	currency := Currency(strings.ToUpper(strings.TrimSpace(value)))
	if !currency.Valid() {
		return "", false
	}
	return currency, true
}
