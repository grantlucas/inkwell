# ADR 0007: Block on the BUSY pin with a polling `waitIdle`, not a single read

- **Status:** Accepted
- **Amended by:** [ADR 0012](0012-sleep-the-panel-between-refreshes.md)
  (settle delays around the poll)
- **Recorded:** 2026-09-18

## Context

Once a waveform is triggered (`RefreshCmd` 0x12, or a no-data init command like
Power On), the controller runs it autonomously. It needs no further SPI traffic,
it pulls BUSY low for the whole pass (up to about 5 s for a full refresh), and it
raises BUSY high only when the pass is finished.

Sending anything during that window, a following init or display call and above
all a hardware `Reset()`, aborts whatever part of the pass hadn't run yet. The
symptom is a reproducible, position-dependent fade or ghost, for instance rows
the controller hadn't reached before being interrupted. It looks like a
rendering bug, not a timing bug.

`Hardware.ReadBusy()` is a single instantaneous pin read. It does not block, so
calling it once and discarding the result reads as synchronization while
providing none.

## Decision

`EPD.waitIdle()` ([`epd.go`](../../internal/inkwell/epd.go)) polls `ReadBusy()`
every 10 ms, matching the Waveshare reference driver's cadence, until the pin
reports idle or a 10 s timeout elapses, at which point it returns an error rather
than hanging the render loop on a stuck or miswired pin.

Every call site that triggers a waveform goes through it: `Display()`,
`DisplayPartial()`, and `execSequence()`'s no-data commands.

## Consequences

A miswired BUSY pin surfaces as an error from the render loop within 10 s instead
of a hang, and the loop keeps running.

The 10 s ceiling is roughly twice the panel's worst-case full refresh, so it has
room for a slow pass without masking a genuinely stuck pin. `busyPollInterval`
and `busyTimeout` are fields on `EPD` so tests can drive the loop without
real delays.
