package calendar

// collapse keeps the first of any occurrences that share a title, start
// and end, so an event carried by two feeds — or by one feed twice —
// shows once. It runs on occurrences rather than series because two
// copies of a meeting need not agree on how they recur, only on when
// they happen, and after rules because two feeds may only agree on a
// title once their boilerplate is stripped.
//
// It filters events in place; callers pass a slice they own.
func collapse(events []Event) []Event {
	type key struct {
		summary    string
		start, end int64
	}
	seen := make(map[key]struct{}, len(events))
	out := events[:0]
	for _, e := range events {
		k := key{summary: e.Summary, start: e.Start.Unix(), end: e.End.Unix()}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, e)
	}
	return out
}
