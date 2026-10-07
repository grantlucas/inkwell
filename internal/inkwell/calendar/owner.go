package calendar

import (
	"net/url"
	"strings"
)

// googleICalPrefix starts every Google Calendar iCal address.
const googleICalPrefix = "https://calendar.google.com/calendar/ical/"

// owner returns the address of the person whose calendar f is, or "" when
// the feed doesn't say. Only an owned feed has declined events: every
// invitee is listed on an event, so hiding declines needs to know whose
// answer counts.
//
// Only a Google secret address names its owner, as
// .../calendar/ical/<id>/private-<key>/basic.ics, where <id> is the
// owner's URL-encoded email for a primary calendar. Nothing else in a URL
// is trusted: a league site may carry a coach's address in a query
// string, and taking it as the owner would hide events by the coach's
// answers.
func (f Feed) owner() string {
	rest, ok := strings.CutPrefix(f.URL, googleICalPrefix)
	parts := strings.Split(rest, "/")
	if !ok || len(parts) != 3 || !strings.HasPrefix(parts[1], "private-") || parts[2] != "basic.ics" {
		return ""
	}
	if id, err := url.PathUnescape(parts[0]); err == nil && strings.Contains(id, "@") {
		return id
	}
	return ""
}
