# ADR 0012: Re-init before every push; keep the panel powered between refreshes

- **Status:** Accepted
- **Recorded:** 2026-09-19
- **Supersedes:** [ADR 0010](0010-re-init-before-every-gray4-push.md)
- **Amends:** [ADR 0007](0007-poll-the-busy-pin-in-waitidle.md)

## Context

Until this decision the render loop initialised the panel once at start-up and
then re-ran `EPD.Init` only when the planned waveform changed (BW) or on every
Gray4 push ([ADR 0010](0010-re-init-before-every-gray4-push.md)). ADR 0010
justified the Gray4 rule by saying the upstream driver re-runs its 4-gray init
before every 4-gray display. It does not: upstream `display_4Gray()` sends only
the two planes and the refresh trigger. The observation behind that ADR (crisp
after a re-init, faded on the next routine push) turned out to be light on the
backplane ([ADR 0014](0014-shield-the-tft-backplane-from-light.md)) and needed
no Gray4-specific theory.

The Waveshare wiki asks long-running deployments to sleep the panel between
refreshes:

> When the screen is not refreshed, please set the screen to sleep mode or
> power off it. Otherwise, the screen will remain in a high voltage state for a
> long time, which will damage the e-Paper and cannot be repaired!

Inkwell's sleep sequence (`0x50 F7` VCOM setting, `0x02` power off, `0x07 A5`
deep sleep) ran only from `EPD.Close` at process exit. An earlier revision of
this ADR added it after every push. **That was tried on the panel on
2026-09-19 and reverted the same day.** With the case sealed (so light was not
a factor), the first full refresh after start-up settled crisp and then, within
200 to 400 ms, the whole image collapsed to light gray edge to edge and stayed
there. The vendor demo with identical init bytes, which does not power off
after its displays, held the same content. The one difference was the power-off
and deep-sleep commands landing about 20 ms after BUSY released. The UC8179
datasheet notes that after the LUT finishes the VCOM driver still outputs two
frames of VCOM_DC before floating; cutting the boosters inside or right after
that tail is the most plausible way to undo a freshly written image everywhere
at once. Whether a *delayed* power-off (seconds after the refresh) would be
safe is untested and is the open follow-up below.

Two timing details of the vendor busy handshake were also missing from
`waitIdle` ([ADR 0007](0007-poll-the-busy-pin-in-waitidle.md)). The reference
driver sleeps 100 ms after issuing a waveform trigger before its first BUSY
read (the C source marks it "necessary, 200uS at least"; the datasheet says
BUSY_N *becomes* low after the command, it is not low yet on the next
instruction), and sleeps 20 ms after BUSY releases before the next command.
Reading the pin immediately can see it still idle and turn the wait into a
no-op.

## Decision

Every frame pushed to the panel, in either colour mode, starts from a fresh
init in `App.refresh` ([`app.go`](../../internal/inkwell/app.go)): hardware
reset, the waveform's init sequence, then the two planes and the refresh
trigger. Nothing follows the refresh. The panel stays powered between pushes;
the sleep sequence runs from `Close` only.

The planner ([`refresh.go`](../../internal/inkwell/refresh.go)) returns only
the waveform to use. The `forceInit` signal and the render loop's
`appliedMode` tracking are gone because every push re-inits, and so is the
start-up `Init` in `Run`, which had double-initialised the panel on the first
cycle.

`waitIdle` sleeps 100 ms before its first BUSY read and 20 ms after the pin
releases (`triggerSettle` and `idleSettle` on `EPD`, alongside the existing
poll interval and timeout). `EPD.Sleep` waits the vendor's 2 s after the
deep-sleep command ("important, at least 2s") so `Close` does not cut PWR while
the controller is still entering sleep, and `EPD.Close` releases the hardware
even when the sleep sequence fails.

The vendor Python driver sends the Get Status command (`0x71`) before each
BUSY read; that is deliberately not replicated. The datasheet lists BUSY_N as
a hardware flag asserted by the refresh command itself, and the vendor C
driver polls the pin without it.

## Consequences

Each push costs a reset, an init sequence and a power-on wait on top of the
refresh itself: well under a second against a push cadence with a one-minute
floor ([ADR 0011](0011-require-a-per-widget-refresh-cadence.md)).

The panel remains energised between refreshes, against the vendor wiki's
advice. That is a known trade: the alternative destroyed the image. Two
follow-ups are worth a hardware session each: a power-off issued several
seconds after BUSY releases rather than immediately, and the panel-setting
`SHD_N` bit (`0x00`), which turns off the charge pump without a full power-off.
Either would need the photo protocol in
[ADR 0014](0014-shield-the-tft-backplane-from-light.md) to sign off.

The shutdown path is unchanged in shape: `Init` loads the full-frame waveform,
`Clear` pushes white, `Close` sleeps the panel and cuts power.
