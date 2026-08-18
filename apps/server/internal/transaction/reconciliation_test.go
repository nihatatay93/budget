package transaction

import (
	"math"
	"testing"
)

func TestValidateReconciliation(t *testing.T) {
	tests := []struct {
		name        string
		kind        Kind
		entries     []int64
		allocations []int64
		wantError   bool
	}{
		{name: "expense", kind: KindStandard, entries: []int64{-35000}, allocations: []int64{-35000}},
		{name: "split expense", kind: KindStandard, entries: []int64{-150000}, allocations: []int64{-100000, -50000}},
		{name: "positive refund to expense category", kind: KindStandard, entries: []int64{10000}, allocations: []int64{10000}},
		{name: "transfer", kind: KindTransfer, entries: []int64{-1000000, 1000000}},
		{name: "single-entry transfer", kind: KindTransfer, entries: []int64{0}, wantError: true},
		{name: "transfer with allocation", kind: KindTransfer, entries: []int64{-1000000, 1000000}, allocations: []int64{0}, wantError: true},
		{name: "uncategorized standard", kind: KindStandard, entries: []int64{-35000}, wantError: true},
		{name: "empty adjustment", kind: KindAdjustment, wantError: true},
		{name: "entry sum overflow", kind: KindTransfer, entries: []int64{math.MaxInt64, 1}, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateReconciliation(test.kind, test.entries, test.allocations)
			if (err != nil) != test.wantError {
				t.Fatalf("ValidateReconciliation() error = %v, wantError %v", err, test.wantError)
			}
		})
	}
}
