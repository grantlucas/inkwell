package testcal

import (
	"fmt"
	"time"
)

// Generate returns valid ICS content with a variety of calendar events
// spanning 7 days starting from the date of now.
func Generate(now time.Time) string {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	var b []byte
	b = append(b, "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Inkwell//TestCal//EN\r\n"...)

	uid := 0
	event := func(summary string, start, end time.Time, allDay bool, location string) {
		uid++
		b = append(b, "BEGIN:VEVENT\r\n"...)
		b = append(b, fmt.Sprintf("UID:testcal-%d@inkwell\r\n", uid)...)
		b = append(b, fmt.Sprintf("SUMMARY:%s\r\n", summary)...)
		if allDay {
			b = append(b, fmt.Sprintf("DTSTART;VALUE=DATE:%s\r\n", start.Format("20060102"))...)
			b = append(b, fmt.Sprintf("DTEND;VALUE=DATE:%s\r\n", end.Format("20060102"))...)
		} else {
			b = append(b, fmt.Sprintf("DTSTART:%sZ\r\n", start.Format("20060102T150405"))...)
			b = append(b, fmt.Sprintf("DTEND:%sZ\r\n", end.Format("20060102T150405"))...)
		}
		if location != "" {
			b = append(b, fmt.Sprintf("LOCATION:%s\r\n", location)...)
		}
		b = append(b, "END:VEVENT\r\n"...)
	}

	// Day 0 (today): busy workday
	d := today
	event("Team Standup", d.Add(9*time.Hour), d.Add(9*time.Hour+15*time.Minute), false, "")
	event("Sprint Planning", d.Add(10*time.Hour), d.Add(11*time.Hour+30*time.Minute), false, "Board Room")
	event("Lunch with Alex", d.Add(12*time.Hour), d.Add(13*time.Hour), false, "Cafe on Main")
	event("Design Review", d.Add(14*time.Hour), d.Add(15*time.Hour), false, "Room 204")

	// Day 1: overlapping meetings + early morning
	d = today.AddDate(0, 0, 1)
	event("Early Gym", d.Add(6*time.Hour), d.Add(7*time.Hour), false, "YMCA")
	event("Standup", d.Add(9*time.Hour), d.Add(9*time.Hour+15*time.Minute), false, "")
	event("Project Sync", d.Add(13*time.Hour), d.Add(14*time.Hour+30*time.Minute), false, "Room 101")
	event("1:1 with Manager", d.Add(14*time.Hour), d.Add(14*time.Hour+30*time.Minute), false, "")

	// Day 2: all-day event + evening
	d = today.AddDate(0, 0, 2)
	event("Company Offsite", d, d.AddDate(0, 0, 1), true, "")
	event("Dinner Reservation", d.Add(20*time.Hour), d.Add(22*time.Hour), false, "The Keg")

	// Day 3: light day
	d = today.AddDate(0, 0, 3)
	event("Dentist", d.Add(10*time.Hour), d.Add(10*time.Hour+30*time.Minute), false, "123 Health St")
	event("Focus Time", d.Add(13*time.Hour), d.Add(16*time.Hour), false, "")

	// Day 4: multi-day event starts + meetings
	d = today.AddDate(0, 0, 4)
	event("Conference", d, d.AddDate(0, 0, 2), true, "Metro Convention Centre")
	event("Standup", d.Add(9*time.Hour), d.Add(9*time.Hour+15*time.Minute), false, "")
	event("Late Movie", d.Add(21*time.Hour), d.Add(23*time.Hour+30*time.Minute), false, "Cineplex")

	// Day 5: conference continues + social
	d = today.AddDate(0, 0, 5)
	event("Team Retro", d.Add(11*time.Hour), d.Add(12*time.Hour), false, "Room 301")
	event("Happy Hour", d.Add(17*time.Hour), d.Add(19*time.Hour), false, "The Pub")

	// Day 6: weekend, relaxed
	d = today.AddDate(0, 0, 6)
	event("Farmers Market", d.Add(8*time.Hour), d.Add(10*time.Hour), false, "City Square")
	event("BBQ at Tom's", d.Add(15*time.Hour), d.Add(19*time.Hour), false, "42 Oak Ave")

	b = append(b, "END:VCALENDAR\r\n"...)
	return string(b)
}
