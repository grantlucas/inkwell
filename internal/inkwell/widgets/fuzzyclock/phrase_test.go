// This file lives in package fuzzyclock_test on purpose: the point of the
// exported Phrase is that a widget in another package (today-hero) can borrow
// the wording without borrowing the widget's white-background rendering. An
// in-package test could not tell a truly exported surface from an unexported
// one, so this compiles only if the surface is genuinely reachable.
package fuzzyclock_test

import (
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widgets/fuzzyclock"
)

func TestPhrase_IsReachableFromAnotherPackage(t *testing.T) {
	when := time.Date(2026, 6, 19, 14, 35, 0, 0, time.UTC)
	got := fuzzyclock.Phrase(when, fuzzyclock.Options{
		Style:        fuzzyclock.StyleSentence,
		NoonMidnight: true,
	})
	if want := "Just after half past two"; got != want {
		t.Errorf("Phrase = %q, want %q", got, want)
	}
}
