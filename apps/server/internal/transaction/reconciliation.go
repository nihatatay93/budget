package transaction

import "errors"

type Kind string

const (
	KindStandard   Kind = "standard"
	KindTransfer   Kind = "transfer"
	KindAdjustment Kind = "adjustment"
)

var ErrDoesNotReconcile = errors.New("transaction entries and allocations do not reconcile")

func ValidateReconciliation(kind Kind, entries, allocations []int64) error {
	if len(entries) == 0 {
		return ErrDoesNotReconcile
	}
	entryTotal, ok := sum(entries)
	if !ok {
		return ErrDoesNotReconcile
	}
	allocationTotal, ok := sum(allocations)
	if !ok {
		return ErrDoesNotReconcile
	}

	switch kind {
	case KindTransfer:
		if len(entries) < 2 || len(allocations) != 0 || entryTotal != 0 {
			return ErrDoesNotReconcile
		}
	case KindStandard:
		if len(allocations) == 0 || allocationTotal != entryTotal {
			return ErrDoesNotReconcile
		}
	case KindAdjustment:
		if len(allocations) > 0 && allocationTotal != entryTotal {
			return ErrDoesNotReconcile
		}
	default:
		return ErrDoesNotReconcile
	}
	return nil
}

func sum(values []int64) (int64, bool) {
	var total int64
	for _, value := range values {
		next := total + value
		if (value > 0 && next < total) || (value < 0 && next > total) {
			return 0, false
		}
		total = next
	}
	return total, true
}
