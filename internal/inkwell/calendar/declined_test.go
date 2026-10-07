package calendar

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/testutil/fakehttp"
)

// declinedICS is a work calendar as Google exports it: every invitee is
// listed on its own ATTENDEE line, and the feed owner's answer lives only
// on theirs. Times are UTC on 5–6 October 2026.
const declinedICS = "BEGIN:VCALENDAR\r\n" +
	// A series the owner declined outright.
	"BEGIN:VEVENT\r\nUID:standup@example.com\r\nSUMMARY:Standup\r\n" +
	"DTSTART:20261005T080000Z\r\nDTEND:20261005T081500Z\r\nRRULE:FREQ=DAILY;COUNT=2\r\n" +
	"ATTENDEE;PARTSTAT=DECLINED:mailto:avery@example.com\r\nEND:VEVENT\r\n" +
	// A series the owner accepted, with one instance declined.
	"BEGIN:VEVENT\r\nUID:sync@example.com\r\nSUMMARY:Sync\r\n" +
	"DTSTART:20261005T090000Z\r\nDTEND:20261005T093000Z\r\nRRULE:FREQ=DAILY;COUNT=2\r\n" +
	"ATTENDEE;PARTSTAT=ACCEPTED:mailto:avery@example.com\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nUID:sync@example.com\r\nSUMMARY:Sync\r\nRECURRENCE-ID:20261006T090000Z\r\n" +
	"DTSTART:20261006T090000Z\r\nDTEND:20261006T093000Z\r\n" +
	"ATTENDEE;PARTSTAT=DECLINED:mailto:avery@example.com\r\nEND:VEVENT\r\n" +
	// A series the owner declined, with one instance accepted.
	"BEGIN:VEVENT\r\nUID:retro@example.com\r\nSUMMARY:Retro\r\n" +
	"DTSTART:20261005T160000Z\r\nDTEND:20261005T170000Z\r\nRRULE:FREQ=DAILY;COUNT=2\r\n" +
	"ATTENDEE;PARTSTAT=DECLINED:mailto:avery@example.com\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nUID:retro@example.com\r\nSUMMARY:Retro\r\nRECURRENCE-ID:20261006T160000Z\r\n" +
	"DTSTART:20261006T160000Z\r\nDTEND:20261006T170000Z\r\n" +
	"ATTENDEE;PARTSTAT=ACCEPTED:mailto:avery@example.com\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nUID:accepted@example.com\r\nSUMMARY:Accepted\r\n" +
	"DTSTART:20261005T100000Z\r\nDTEND:20261005T103000Z\r\n" +
	"ATTENDEE;PARTSTAT=ACCEPTED:mailto:avery@example.com\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nUID:tentative@example.com\r\nSUMMARY:Tentative\r\n" +
	"DTSTART:20261005T110000Z\r\nDTEND:20261005T113000Z\r\n" +
	"ATTENDEE;PARTSTAT=TENTATIVE:mailto:avery@example.com\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nUID:unanswered@example.com\r\nSUMMARY:Unanswered\r\n" +
	"DTSTART:20261005T120000Z\r\nDTEND:20261005T123000Z\r\n" +
	"ATTENDEE;PARTSTAT=NEEDS-ACTION:mailto:avery@example.com\r\nEND:VEVENT\r\n" +
	// Declined, with the address in another case than the feed URL's.
	"BEGIN:VEVENT\r\nUID:declined@example.com\r\nSUMMARY:Declined\r\n" +
	"DTSTART:20261005T130000Z\r\nDTEND:20261005T133000Z\r\n" +
	"ATTENDEE;CN=Avery Quinn;PARTSTAT=DECLINED:mailto:Avery@Example.COM\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nUID:coworker@example.com\r\nSUMMARY:Coworker declined\r\n" +
	"DTSTART:20261005T140000Z\r\nDTEND:20261005T143000Z\r\n" +
	"ATTENDEE;PARTSTAT=ACCEPTED:mailto:avery@example.com\r\n" +
	"ATTENDEE;PARTSTAT=DECLINED:mailto:coworker@example.com\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nUID:solo@example.com\r\nSUMMARY:No attendees\r\n" +
	"DTSTART:20261005T150000Z\r\nDTEND:20261005T153000Z\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nUID:uninvited@example.com\r\nSUMMARY:Owner not invited\r\n" +
	"DTSTART:20261005T153000Z\r\nDTEND:20261005T155000Z\r\n" +
	"ATTENDEE;PARTSTAT=DECLINED:mailto:coworker@example.com\r\nEND:VEVENT\r\n" +
	// A secondary calendar invited as a guest, with an answer of its own.
	"BEGIN:VEVENT\r\nUID:team@example.com\r\nSUMMARY:Team calendar declined\r\n" +
	"DTSTART:20261005T154500Z\r\nDTEND:20261005T155500Z\r\n" +
	"ATTENDEE;PARTSTAT=ACCEPTED:mailto:avery@example.com\r\n" +
	"ATTENDEE;PARTSTAT=DECLINED:mailto:team-x@group.calendar.google.com\r\nEND:VEVENT\r\n" +
	"END:VCALENDAR\r\n"

// On a Google iCal feed the owner is named by the URL, and the events
// they declined are never shown. Any other feed has no owner, so nothing
// on it counts as declined and its events are exactly what it carries.
func TestProvider_DeclinedEvents(t *testing.T) {
	everything := []string{
		"Standup", "Sync", "Accepted", "Tentative", "Unanswered", "Declined",
		"Coworker declined", "No attendees", "Owner not invited",
		"Team calendar declined", "Retro", "Standup", "Sync", "Retro",
	}
	ownerView := []string{
		"Sync", "Accepted", "Tentative", "Unanswered",
		"Coworker declined", "No attendees", "Owner not invited",
		"Team calendar declined", "Retro",
	}

	cases := []struct {
		label string
		url   string
		want  []string
	}{
		{
			label: "Google secret address names the owner",
			url:   "https://calendar.google.com/calendar/ical/avery%40example.com/private-0123abcd/basic.ics",
			want:  ownerView,
		},
		{
			label: "Google public address has no owner",
			url:   "https://calendar.google.com/calendar/ical/avery%40example.com/public/basic.ics",
			want:  everything,
		},
		{
			label: "Google host with another path has no owner",
			url:   "https://calendar.google.com/calendar/ical/avery%40example.com/private-0123abcd/extra/basic.ics",
			want:  everything,
		},
		{
			label: "an address in a query string names no owner",
			url:   "https://league.example/team.ics?coach=avery%40example.com",
			want:  everything,
		},
		{
			label: "Google's path on another host has no owner",
			url:   "https://calendar.google.com.league.example/calendar/ical/avery%40example.com/private-0123abcd/basic.ics",
			want:  everything,
		},
		{
			label: "Google secondary calendar names no person",
			url:   "https://calendar.google.com/calendar/ical/team-x%40group.calendar.google.com/private-0123abcd/basic.ics",
			want:  everything,
		},
		{
			label: "Google holiday calendar names no person",
			url:   "https://calendar.google.com/calendar/ical/en.canadian%23holiday%40group.v.calendar.google.com/private-0123abcd/basic.ics",
			want:  everything,
		},
		{
			label: "Google calendar ID that is not an address has no owner",
			url:   "https://calendar.google.com/calendar/ical/avery/private-0123abcd/basic.ics",
			want:  everything,
		},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			tr := fakehttp.New()
			tr.Serve(tc.url, declinedICS)
			start := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
			p := NewProvider(tr, func() time.Time { return start })

			events, err := p.Occurrences(context.Background(), []Feed{{URL: tc.url}}, start, start.AddDate(0, 0, 2), time.Hour)
			if err != nil {
				t.Fatalf("Occurrences: %v", err)
			}
			if got := summaries(events); !slices.Equal(got, tc.want) {
				t.Errorf("got  %q\nwant %q", got, tc.want)
			}
		})
	}
}
