# Refresh mode (flashing)

How and when the panel refreshes, and why the driver is the way it is.
Read this before touching refresh waveforms, panel sleep, the init
sequences in `profile.go`, or the per-widget refresh gate.

How often the panel flashes is a separate axis from how a frame is rendered.
The render loop picks a refresh waveform per cycle (`refreshPlanner` in
`refresh.go`): BW does a full-screen fast refresh on every changed cycle (a
single flash), plus a full refresh on the first cycle and periodically after
that to clear ghosting. Gray4 has no fast waveform: it refreshes in
grayscale on those same first and periodic cycles and on every changed
cycle, and skips only when the frame is unchanged. A windowed, flicker-free
per-change refresh was tried and abandoned — the force-drive it needs to
redraw changed pixels cleanly settles the box inverted under the partial
waveform on real hardware (inkwell-6jq). The flash is hardware-only — the
web preview can't show it.

Every push runs a hardware reset + the waveform's init sequence, the frame,
then `EPD.Sleep`: a settle of a few seconds, the sleep sequence (power off
and deep sleep), and the deep-sleep settle. The panel is not left energised
between refreshes — the vendor says that damages it, and an energised panel
is what lets light on the TFT backplane fade a settled image. The settle
before the power-off is load-bearing: issued 20 ms after BUSY released, the
power-off wiped a freshly written image edge to edge on real hardware
(ADR 0012). Don't shorten it without the photo protocol in ADR 0014. The init
sequences in `profile.go` are pinned byte for byte by test — `InitFull`
deliberately keeps the controller's reset-default drive rails instead of
the vendor's `0x01` bytes, which under-drove this panel (ADR 0013). Do not
"tune" them against the preview. If the panel fades region-by-region within
a second of a refresh settling, suspect light reaching the back of the panel
before suspecting the driver (ADR 0014).

*When* a change is allowed to push is a further axis. The burn-in/waveform
cadence is fixed internally (`defaultFullEvery` in `refresh.go`), not user
config. What the config controls is each widget's
**required** top-level `refresh:` — a duration (>= 1m) or `"static"` — parsed
into `WidgetConfig.Refresh`; there is no widget-code cadence interface and no
default (LoadConfig errors if a widget omits it). A per-screen `refreshSchedule`
(`refresh_queue.go`) gates the planner — a frame change only pushes when a
widget is *due* this minute (wall-clock aligned, so equal cadences coalesce;
static widgets never open the gate). A screen **rotation** is the one thing
that opens the gate on its own: `Dashboard.Advance` reports when it rotated
and `App.nextCycle` ORs that into *due*, so a rotation reaches the panel on
the cycle it happens rather than waiting for the new screen's next due
widget. That flag is consume-once, so only the render loop may call
`Advance`; anything else wanting the current screen calls the read-only
`CurrentScreen`. Don't confuse a widget's top-level
`refresh` (render cadence) with a calendar widget's nested `config.refresh`
(data cache TTL). See the
[architecture decision records](../adrs/), 0008 through 0014.
