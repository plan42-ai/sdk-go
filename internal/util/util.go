//nolint:revive
package util

import (
	"io"
	"time"
)

func Pointer[T any](v T) *T {
	return &v
}

func EqualsMS(l time.Time, r time.Time) bool {
	return l.Truncate(time.Microsecond).Equal(r.Truncate(time.Microsecond))
}

func Close[T io.Closer](c T) {
	_ = c.Close()
}
