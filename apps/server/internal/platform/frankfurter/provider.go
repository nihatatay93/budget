// Package frankfurter reads European Central Bank reference rates from the Frankfurter API.
//
// The provider is optional: the application must behave normally when it is disabled or
// unreachable. See docs/decisions/0005-supported-currencies-and-display-conversion.md.
package frankfurter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nihatatay93/budget/internal/exchange"
	"github.com/nihatatay93/budget/internal/money"
)

// maxResponseBytes bounds what an untrusted endpoint can make the server buffer.
const maxResponseBytes = 1 << 20

type Provider struct {
	client  *http.Client
	baseURL string
}

func New(baseURL string, timeout time.Duration) *Provider {
	return &Provider{
		client:  &http.Client{Timeout: timeout},
		baseURL: strings.TrimSuffix(baseURL, "/"),
	}
}

// response mirrors the documented payload, for example:
//
//	{"amount":1.0,"base":"TRY","date":"2026-08-17","rates":{"EUR":0.01801,"USD":0.02088}}
//
// Rates decode into json.Number so the published decimal reaches big.Rat without passing
// through float64, which cannot represent a value such as 0.02088 exactly.
type response struct {
	Base  string                 `json:"base"`
	Date  string                 `json:"date"`
	Rates map[string]json.Number `json:"rates"`
}

func (p *Provider) LatestRates(
	ctx context.Context,
	base money.Currency,
	quotes []money.Currency,
) ([]exchange.Rate, error) {
	return p.rates(ctx, "latest", base, quotes)
}

// HistoricalRates reads the rate published for, or most recently before, a business date.
// Frankfurter returns the actual publication date in the response on weekends and holidays.
func (p *Provider) HistoricalRates(
	ctx context.Context,
	date time.Time,
	base money.Currency,
	quotes []money.Currency,
) ([]exchange.Rate, error) {
	if date.IsZero() {
		return nil, fmt.Errorf("fetch exchange rates: missing historical date")
	}
	rates, err := p.rates(ctx, date.Format(time.DateOnly), base, quotes)
	if err != nil {
		return nil, err
	}
	for _, rate := range rates {
		if rate.Date.After(date) {
			return nil, fmt.Errorf("historical exchange rate date %s follows requested date %s", rate.Date.Format(time.DateOnly), date.Format(time.DateOnly))
		}
	}
	return rates, nil
}

func (p *Provider) rates(
	ctx context.Context,
	path string,
	base money.Currency,
	quotes []money.Currency,
) ([]exchange.Rate, error) {
	if len(quotes) == 0 {
		return nil, nil
	}
	if !base.Valid() {
		return nil, fmt.Errorf("fetch exchange rates: unsupported base %q", base)
	}
	symbols := make([]string, 0, len(quotes))
	seen := make(map[money.Currency]bool, len(quotes))
	for _, quote := range quotes {
		if !quote.Valid() || quote == base || seen[quote] {
			return nil, fmt.Errorf("fetch exchange rates: unusable quote %q", quote)
		}
		seen[quote] = true
		symbols = append(symbols, quote.String())
	}
	endpoint := fmt.Sprintf(
		"%s/v1/%s?base=%s&symbols=%s",
		p.baseURL, path, url.QueryEscape(base.String()), url.QueryEscape(strings.Join(symbols, ",")),
	)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build exchange rate request: %w", err)
	}
	request.Header.Set("Accept", "application/json")

	httpResponse, err := p.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch exchange rates: %w", err)
	}
	defer func() { _ = httpResponse.Body.Close() }()
	if httpResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch exchange rates: unexpected status %d", httpResponse.StatusCode)
	}

	var decoded response
	decoder := json.NewDecoder(http.MaxBytesReader(nil, httpResponse.Body, maxResponseBytes))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode exchange rates: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("decode exchange rates: trailing content")
	}

	// The response date is the publication date, which is earlier than today on weekends and
	// holidays. Clients display it so a converted figure is never mistaken for a live one.
	date, err := time.Parse(time.DateOnly, decoded.Date)
	if err != nil {
		return nil, fmt.Errorf("parse exchange rate date %q: %w", decoded.Date, err)
	}
	if decoded.Base != base.String() {
		return nil, fmt.Errorf("exchange rate base mismatch: requested %s, received %q", base, decoded.Base)
	}

	rates := make([]exchange.Rate, 0, len(quotes))
	for _, quote := range quotes {
		value, ok := decoded.Rates[quote.String()]
		if !ok {
			return nil, fmt.Errorf("exchange rate response omitted requested quote %s", quote)
		}
		rate, err := exchange.ParseRate(value.String())
		if err != nil {
			return nil, err
		}
		rates = append(rates, exchange.Rate{Base: base, Quote: quote, Rate: rate, Date: date})
	}
	return rates, nil
}
