# ADR 0012: Sleep the panel after every push and re-init before the next

- **Status:** Accepted
- **Recorded:** 2026-09-19
- **Supersedes:** [ADR 0010](0010-re-init-before-every-gray4-push.md)
- **Amends:** [ADR 0007](0007-poll-the-busy-pin-in-waitidle.md)

## Context

Until this decision the render loop initialised the panel once at start-up and
then only re-ran `EPD.Init` when the planned waveform changed (BW) or on every
Gray4 push ([ADR 0010](0010-re-init-before-every-gray4-push.md)). The sleep
sequence (`0x50 F7` VCOM setting, `0x02` power off, `0x07 A5` deep sleep) ran
only from `EPD.Close` at process exit. Between refreshes the panel therefore
sat with its boosters, VCOM and ±20 V gate rails live, for hours in BW steady
state and for the whole life of the process overall.

The Waveshare wiki for this panel is explicit:

> When the screen is not refreshed, please set the screen to sleep mode or
> power off it. Otherwise, the screen will remain in a high voltage state for a
> long time, which will damage the e-Paper and cannot be repaired!

Its FAQ gives the same instruction as the answer to "after using for a period
of time, the screen has a serious afterimage problem that cannot be repaired".
The vendor demo sleeps only at its end because it runs for seconds; Inkwell
runs for months.

There is a second, more immediate reason that surfaced while investigating the
fading in issue #77 ([ADR 0014](0014-shield-the-tft-backplane-from-light.md)).
The panel's pixels are switched by amorphous-silicon transistors, which are
photoconductive. Light on the backplane makes them leak, but a leaking
transistor only moves charge if there is voltage behind it. Sleeping the panel
between refreshes removes that voltage, so a stray light source can degrade a
refresh while it runs but can no longer erode a settled image afterwards.

Finally, [ADR 0010](0010-re-init-before-every-gray4-push.md) justified its
per-push re-init by saying the upstream driver re-runs its 4-gray init before
every 4-gray display. It does not: upstream `display_4Gray()` sends only the
two planes and the refresh trigger, and the vendor example initialises once
per section. The observation behind that ADR (crisp after a re-init, faded on
the next routine push) is consistent with the light-leak explanation and did
not need a Gray4-specific theory.

Two timing details of the vendor busy handshake were also missing from
`waitIdle` ([ADR 0007](0007-poll-the-busy-pin-in-waitidle.md)). The reference
driver sleeps 100 ms after issuing a waveform trigger before its first BUSY
read (the C source marks it "necessary, 200uS at least"; the datasheet says
BUSY_N *becomes* low after the command, it is not low yet on the next
instruction), and sleeps 20 ms after BUSY releases before the next command.
Reading the pin immediately can see it still idle and turn the wait into a
no-op. That was tolerable while a full minute separated a refresh from the
next command; it is not once a power-off follows every refresh.

## Decision

Every frame pushed to the panel, in either colour mode, runs the vendor
lifecycle in `App.refresh` ([`app.go`](../../internal/inkwell/app.go)):

1. `EPD.Init(mode)`: hardware reset, then the waveform's init sequence.
2. `EPD.Display(buf)`: both planes, refresh trigger, busy wait.
3. `EPD.Sleep()`: VCOM setting, power off with busy wait, deep sleep.

A skipped cycle touches the hardware not at all. The planner
([`refresh.go`](../../internal/inkwell/refresh.go)) now returns only the
waveform to use; the `forceInit` signal and the render loop's `appliedMode`
tracking are gone because every push re-inits, and so is the start-up `Init`
in `Run`, which had double-initialised the panel on the first cycle.

`waitIdle` sleeps 100 ms before its first BUSY read and 20 ms after the pin
releases (`triggerSettle` and `idleSettle` on `EPD`, alongside the existing
poll interval and timeout). The SPI backend's `Close` waits the vendor's 2 s
between the deep-sleep command and dropping PWR.

## Consequences

Each push costs a reset, an init sequence, a power-on wait and a power-off
wait on top of the refresh itself: well under a second in total, against a
push cadence with a one-minute floor
([ADR 0011](0011-require-a-per-widget-refresh-cadence.md)).

The panel spends its idle time in deep sleep, which is what the vendor asks
of a long-running deployment, and what makes it insensitive to back-side
light between refreshes.

The shutdown path is unchanged in shape: `Init` wakes the sleeping controller
(a sleeping panel ignores frame data until it is reset and initialised again),
`Clear` pushes white, `Close` sleeps it once more and cuts power.
