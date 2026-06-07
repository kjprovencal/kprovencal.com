package main

import (
	"errors"
	"strings"
	"time"
)

// parseLogTimeQuery parses since/until query values (RFC3339 or YYYY-MM-DD).
// Date-only until is inclusive through end of that UTC day.
func parseLogTimeQuery(raw string, asUntil bool) (time.Time, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	if t, err := time.ParseInLocation("2006-01-02", s, time.UTC); err == nil {
		if asUntil {
			return t.Add(24*time.Hour - time.Nanosecond), nil
		}
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04", s, time.UTC); err == nil {
		return t, nil
	}
	return time.Time{}, errInvalidLogTime
}

var errInvalidLogTime = errors.New("invalid log time")
