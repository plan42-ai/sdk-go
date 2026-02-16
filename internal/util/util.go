//nolint:revive
package util

import (
	"time"
)

func Pointer[T any](v T) *T {
	return &v
}

func EqualsMS(l time.Time, r time.Time) bool {
	return l.Truncate(time.Microsecond).Equal(r.Truncate(time.Microsecond))
}
