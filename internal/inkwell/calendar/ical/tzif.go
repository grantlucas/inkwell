package ical

import (
	"cmp"
	"encoding/binary"
	"math"
	"slices"
	"time"
)

// A VTIMEZONE that defines both halves and says when they swap carries
// everything a real zone needs, but Go will only build a DST-aware
// *time.Location from TZif data — the zoneinfo binary format. So we
// synthesise one.
//
// The alternative, handing back a time.FixedZone chosen for the instant
// being parsed, gets single events right and recurrences wrong: the
// RRULE walkers step in whatever offset DTSTART landed in, so a weekly
// event starting in January kept January's offset all year and rendered
// an hour out from the spring switchover onward (issue #105). A real
// Location makes AddDate behave exactly as it does for an IANA zone.

const (
	// tzifFirstYear/tzifLastYear bound the transitions written into the
	// blob. Two per year over this span is a few KB — small enough to
	// build on every parse rather than cache, and wide enough that no
	// dashboard will reach an edge. The range is only safe because the
	// 64-bit v2 block carries it; the v1 block, whose times are signed
	// 32-bit, is truncated at 2038.
	tzifFirstYear = 1970
	tzifLastYear  = 2100

	// tzifHeaderSize is the fixed part: magic, version, 15 reserved
	// bytes, then six counts.
	tzifHeaderSize = 4 + 1 + 15 + 6*4

	// ttinfoSize is one type record: a 4-byte offset, an isdst flag,
	// and an index into the abbreviation table.
	ttinfoSize = 4 + 1 + 1
)

// buildLocation compiles the component's rules into a DST-aware
// Location, or reports false when they do not describe a switching zone
// (one half missing, or a rule this parser cannot read). The caller
// falls back to a fixed offset in that case.
func (v *vtimezone) buildLocation() (*time.Location, bool) {
	std, dst := v.pair()
	if std == nil || dst == nil || !std.hasRule || !dst.hasRule {
		return nil, false
	}

	// Transition instants, in UTC. Each switchover is written in the
	// outgoing zone's wall time, so the incoming rule's TZOFFSETFROM
	// is what converts it.
	// A rule with no readable TZOFFSETFROM takes the other half's
	// TZOFFSETTO, which is what the outgoing offset must be in a
	// two-rule component. Leaving it at zero would place every
	// switchover the zone's whole offset early.
	fromOf := func(r, other *tzRule) int {
		if r.hasFrom {
			return r.from
		}
		return other.offset
	}
	dstFrom, stdFrom := fromOf(dst, std), fromOf(std, dst)

	var transitions []tzTransition
	for year := tzifFirstYear; year <= tzifLastYear; year++ {
		for _, r := range []struct {
			rule *tzRule
			from int
		}{{dst, dstFrom}, {std, stdFrom}} {
			wall := r.rule.onsetIn(year)
			transitions = append(transitions, tzTransition{
				at:    wall.Unix() - int64(r.from),
				isDST: r.rule.daylight,
			})
		}
	}
	// TZif requires transitions in ascending order and Go's loader
	// rejects a blob where they are not. Appending dst-then-std per
	// year is already ascending in the northern hemisphere and is not
	// in the southern, where daylight starts in October and ends the
	// following April.
	slices.SortFunc(transitions, func(a, b tzTransition) int {
		return cmp.Compare(a.at, b.at)
	})

	name := func(r *tzRule) string {
		if r.name != "" {
			return r.name
		}
		return v.id
	}
	// Two types: index 0 standard, index 1 daylight.
	stdName, dstName := name(std), name(dst)
	abbrev := []byte(stdName + "\x00" + dstName + "\x00")
	dstNameIdx := len(stdName) + 1

	// A TZif v2 file: a version-1 block, then a version-2 block whose
	// transition times are 64-bit, then a footer. The v1 block exists
	// only because the format requires it — v1 times are *signed
	// 32-bit*, so anything past 2038-01-19 wraps negative, the table
	// stops being ascending and the zone silently resolves to the
	// wrong offset rather than failing to load. Writing the real range
	// only in the 64-bit block is what makes tzifLastYear safe.
	std32 := transitionsBefore(transitions, math.MaxInt32)

	buf := append([]byte(nil), tzifBlock(1, std32, std, dst, abbrev, dstNameIdx)...)
	buf = append(buf, tzifBlock(2, transitions, std, dst, abbrev, dstNameIdx)...)
	// The footer's POSIX TZ string extends the zone past the last
	// transition. Left empty: the table already runs to tzifLastYear,
	// and an empty extend string is what the format asks for when
	// there is nothing to add.
	buf = append(buf, '\n', '\n')

	loc, err := loadTZData(v.id, buf)
	if err != nil {
		// A rule shaped in a way that produces unusable data falls
		// back to the fixed offset rather than failing the parse: a
		// zone an hour out beats a feed that will not load.
		return nil, false
	}
	return loc, true
}

// tzifBlock writes one header-plus-data block. size 1 writes 32-bit
// transition times, size 2 writes 64-bit ones; both carry the same two
// type records and abbreviation table.
func tzifBlock(size int, transitions []tzTransition, std, dst *tzRule, abbrev []byte, dstNameIdx int) []byte {
	buf := make([]byte, 0, tzifHeaderSize+len(transitions)*9+2*ttinfoSize+len(abbrev)+2)
	buf = append(buf, 'T', 'Z', 'i', 'f')
	buf = append(buf, '2')
	buf = append(buf, make([]byte, 15)...) // reserved
	buf = appendU32(buf, 0)                // isutcnt
	buf = appendU32(buf, 0)                // isstdcnt
	buf = appendU32(buf, 0)                // leapcnt
	buf = appendU32(buf, uint32(len(transitions)))
	buf = appendU32(buf, 2) // typecnt: standard, then daylight
	buf = appendU32(buf, uint32(len(abbrev)))

	for _, t := range transitions {
		if size == 1 {
			buf = appendU32(buf, uint32(int32(t.at)))
			continue
		}
		buf = binary.BigEndian.AppendUint64(buf, uint64(t.at))
	}
	for _, t := range transitions {
		if t.isDST {
			buf = append(buf, 1)
			continue
		}
		buf = append(buf, 0)
	}
	// ttinfo records, in the order the indices above refer to.
	buf = appendU32(buf, uint32(int32(std.offset)))
	buf = append(buf, 0, 0)
	buf = appendU32(buf, uint32(int32(dst.offset)))
	buf = append(buf, 1, byte(dstNameIdx))
	return append(buf, abbrev...)
}

// transitionsBefore returns the leading run of transitions at or below
// limit, which is what the 32-bit block can represent without wrapping.
func transitionsBefore(transitions []tzTransition, limit int64) []tzTransition {
	for i, t := range transitions {
		if t.at > limit {
			return transitions[:i]
		}
	}
	return transitions
}

// tzTransition is one switchover: the UTC instant it happens at, and
// whether the zone is on daylight time afterwards.
type tzTransition struct {
	at    int64
	isDST bool
}

func appendU32(b []byte, v uint32) []byte {
	return binary.BigEndian.AppendUint32(b, v)
}

// loadTZData is indirected through a var so the failure branch above
// stays reachable from tests: the blob this file builds is well formed
// by construction, so nothing a feed can say will make the real loader
// reject it.
var loadTZData = time.LoadLocationFromTZData
