# Coding standards

Rules the reviewer applies to a diff. They are judgement calls: tooling
already enforces formatting, vet and the 100% coverage gate. The hardware
and visual rules in [`AGENTS.md`](AGENTS.md) apply as well.

## Day widgets share one kit

A widget that shows days of calendar events or weather (the day-timeline,
today-weather, weather-ahead, event-list and similar) is built on
`internal/inkwell/widgets/daydata`, not beside it. Flag a new widget that
writes its own version of any of these:

- **Factory.** `daydata.Factory(name, parse, build)` parses the settings,
  builds the day data and calls the constructor. A widget with only the
  shared settings passes `daydata.Parser(spec)`.
- **Its own settings.** Read them with `daydata.ReadKeys` and the `Keys`
  methods (`Int`, `Positive`, `Bool`, `String`), with `daydata.Choice`
  for a named option. Every widget then rejects bad values with the same
  wording.
- **Which sources it reads.** Set `daydata.Spec.Reads` to `WeatherOnly` or
  `CalendarOnly`; the shared parser then rejects the other side's keys with a
  reason.
- **Bounds too small to draw in.** `daydata.Fits` logs and reports it, and
  the widget draws nothing.
- **Missing data.** A widget draws a defined empty state (for weather,
  `daydata.NoForecast`) and never returns an error from `Render`: the
  compositor drops the whole frame on the first error.

## Test helpers live in `testutil`

Pixel assertions shared between packages belong in
`internal/inkwell/testutil`: `Inked`, `SameIn`, `PaintOutside`,
`HasSolidSquare` and `AssertGoldenPNG`. Flag a test file that defines its own
copy of one of them, or a new cross-package helper written in a single
package's tests.

Within a package, tests that differ only in inputs and expected outputs are
one table-driven test, not several tests sharing a helper.
