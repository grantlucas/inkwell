# ADR 0014: Shield the panel's TFT backplane from light

- **Status:** Accepted
- **Recorded:** 2026-09-19

## Context

Issue #77 tracked a reproducible fade on the 7.5" V2: content rendered crisp
as the refresh waveform settled, then within about a second faded to patchy
gray, worst in the middle and lower part of the panel, with heavy-ink areas
(large digits, a solid logo band, 5 to 7 px precipitation bars) fading first
and sparse text holding. Three refresh-timing changes (ADRs 0007, 0009, 0010)
did not move it.

The decisive test was running Waveshare's own demo (`epd_7in5_V2_test.py`)
on the same Pi and panel with Inkwell stopped. Frame analysis of the
recording showed the vendor's full refresh crisp at 9.7 s and collapsed to
streaky gray by 10.0 s, header row unaffected, so the fault was not in
Inkwell's command stream at all.

The 3D-printed case had a half-moon cutout in its back, behind the panel.
Covering it opaquely removed most of the fade from the vendor demo and from
Inkwell alike; what remained was confined to the top band, where the case has
a second gap and where a lamp sits directly above.

The mechanism: an e-paper panel's pixels are switched by amorphous-silicon
thin-film transistors on the *back* of the glass. Amorphous silicon is
photoconductive. Light on the backplane makes the transistors leak, so lit
pixels receive less of the drive impulse during a refresh and bleed charge
afterwards while the panel is energised. Under-driven pixels then relax
("optical kickback": E Ink measured the remnant voltage decaying from ±3 V to
±1 V within a second of the pulse ending), which is the fade. Columns carrying
more black draw more current and so sit closer to the margin, which is why
dense content faded first and sparse content held. The front of the panel is
built to be lit; the back is normally covered precisely because of this.

## Decision

The panel's backplane must be opaque to light in any enclosure: a solid back
with no cutouts behind the active area, and no gaps at the edges that admit
light from a lamp above or a window behind. A liner of black card inside the
case is sufficient; the panel's own back sheet is thin.

Inkwell sleeps the panel between refreshes
([ADR 0012](0012-sleep-the-panel-between-refreshes.md)) so that a settled
image is no longer sensitive to stray light; that protects against the
symptom but does not license leaving the back open, since light still weakens
the refresh while it runs.

## Consequences

Region-specific fading that appears within a second or two of a refresh
settling, tracks how much ink a region carries, and reproduces with the vendor
demo is a light or power-delivery problem, not a driver bug. The ten-second
diagnostic: cover the back of the panel completely and refresh again. The
reverse test also works on an energised panel: hold a torch against the back
for a few seconds and watch a matching patch fade on the front.

The three earlier ADRs stand as a record of what was tried; 0009 and 0010 are
superseded by [ADR 0013](0013-restore-the-vendor-init-sequence.md) and
[ADR 0012](0012-sleep-the-panel-between-refreshes.md), which return the driver
to the vendor's sequences and lifecycle.
