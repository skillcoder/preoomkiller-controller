package cronparser

import (
	"fmt"
	"strings"
	"time"

	cron "github.com/netresearch/go-cron"
)

var _parser = cron.MustNewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
)

// Parser computes next cron occurrences using go-cron.
type Parser struct{}

// New creates a new cron parser.
func New() *Parser {
	return &Parser{}
}

// NextAfter returns the next cron occurrence strictly after `after`.
// If tz is non-empty and the spec has no CRON_TZ=/TZ= prefix, it prepends CRON_TZ=<tz>.
// Defaults to UTC when no tz is given.
func (p *Parser) NextAfter(
	spec,
	tz string,
	after time.Time,
) (time.Time, error) {
	fullSpec := buildSpec(spec, tz)

	schedule, err := _parser.Parse(fullSpec)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse cron spec %q: %w", spec, err)
	}

	return schedule.Next(after), nil
}

func buildSpec(spec, tz string) string {
	hasTZPrefix := strings.HasPrefix(spec, "CRON_TZ=") ||
		strings.HasPrefix(spec, "TZ=")

	if tz != "" && !hasTZPrefix {
		return "CRON_TZ=" + tz + " " + spec
	}

	if !hasTZPrefix {
		return "CRON_TZ=UTC " + spec
	}

	return spec
}
