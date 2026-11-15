package core

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

var errMessageTimestamp = errors.New("invalid message timestamp")

const timestampFractionDigits = 6 // Slack timestamps preserve microsecond precision.

// MessageTime parses a Slack message identity without floating-point rounding.
func MessageTime(ts string) (time.Time, error) {
	sec, frac, ok := strings.Cut(ts, ".")
	if !ok || len(frac) != timestampFractionDigits {
		return time.Time{}, errMessageTimestamp
	}
	for _, c := range sec + frac {
		if c < '0' || c > '9' {
			return time.Time{}, errMessageTimestamp
		}
	}
	s, err := strconv.ParseInt(sec, 10, 64)
	if err != nil {
		return time.Time{}, errMessageTimestamp
	}
	us, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return time.Time{}, errMessageTimestamp
	}
	return time.Unix(s, us*int64(time.Microsecond)).UTC(), nil
}
