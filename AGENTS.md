# Inkwell Development Rules

## Target Hardware — Read This Before Touching Rendering

**Inkwell drives a Waveshare 7.5" V2 e-paper panel.** It supports two
modes; both are wired end-to-end and selectable via `color_mode` in
config. Forgetting which mode is active (and what it actually shows)
leads to PRs that look great in the preview but worse than baseline on
real hardware.

What the panel can show:

- **`gray4` mode (default):** 2 bits per pixel — white, light gray,
  dark gray, black. Driven by the `Init4Gray` command sequence and a
  split-plane SPI write. Slower refresh and a larger framebuffer than
  BW; no partial refresh. This is the recommended mode and the default
  in `DefaultConfig()`.
- **`bw` mode:** 1 bit per pixel — pure black or pure white via a
  `Y<=128` threshold (any pixel at least half covered is inked).
  Faster refresh, smaller framebuffer.

There is **no native 8-level or 12-level grayscale, and there is no
dithering.** Both packers collapse the compositor's frame straight to
the device's bit depth:

- `packBW` (`internal/inkwell/buffer.go`) — pure threshold. Anything
  with luminance > 128 becomes white; `Y <= 128` (at least half
  covered) becomes black. No Bayer / Floyd-Steinberg stipple anywhere;
  soft grays don't "survive" — they collapse all-or-nothing.
- `packGray4` — 4-level luminance buckets via the boundaries baked
  into `gray4Palette`: `Y > 192` → white, `> 128` → light gray,
  `> 64` → dark gray, else black. Used by the `Init4Gray` device
  path.

The compositor still draws into a 12-level `PaperPalette` so `packGray4`
can express two real gray buckets and so gray *fills* (e.g. precip bars)
and the anti-aliased weather-**icon** font have intermediate shades to
land on. **Body text no longer relies on those grays:** Inkwell renders a
true bitmap typeface (Tamzen, via the BDF parser in
`internal/inkwell/fonts/bdf.go`) whose glyphs are pure 1-bit masks — no
anti-aliasing, so nothing for the threshold to drop (see inkwell-5yh /
inkwell-qd8). Just don't mistake the source canvas for what the device
will show.

### Hard rules when adding visual elements

1. **Always reason about what the *device* will show, not what the
   preview shows.** Open `http://localhost:8080/` and look at the
   default view — that is the post-pack device buffer (BW threshold or
   Gray4 quantized, depending on `color_mode`). Use the
   `Source (design intent)` toggle (or `?source=1`) only for design
   review; do not rely on it for visual sign-off.
2. **Soft accents must be expressed as solid `PaperBlack` strokes,
   not as gray fills.** A `PaperGray20` background tint vanishes in
   Gray4's light bucket and snaps to white under the BW threshold —
   it reads on neither mode. Use inversion (`PaperBlack` fill +
   `PaperWhite` text) for highlights and 1–2 px `PaperBlack` strokes
   for indicators. `PaperGrayNN` fills are only useful when the region
   is large enough for the Gray4 bucket to read (precip-bar
   interiors are the canonical case — they land `PaperGray70` so
   Gray4 gets dark gray and BW gets solid black).
3. **For text, use `PaperBlack` as the source color.** Glyphs are 1-bit
   bitmap masks, so a black source paints solid-black pixels that read on
   both modes. A gray source still works (the mask is binary but painted
   in the chosen gray, e.g. `PaperGray70` for a secondary label), but on
   BW that gray then obeys the `Y<=128` threshold — so only use it where a
   Gray4 dark bucket is the point. Carry visual hierarchy with font
   weight + size, not color.
4. **Text is a bitmap font (Tamzen) — `fonts.Regular` is safe at every
   shipped size.** Because glyphs are pixel-perfect 1-bit masks there is
   no anti-aliasing to fragment, so the old "use SemiBold below ~14 pt"
   workaround is gone. Note `fonts.Face(weight, sizePt)` snaps the point
   size to the nearest embedded Tamzen pixel tier (12/16/20px →
   ~10/12/16 pt); pick sizes near those tiers and verify on the device
   view. Use `fonts.SemiBold`/`fonts.Bold` for *emphasis*, not legibility.
5. **Don't add new `PaperGrayNN` entries.** The palette is pinned by
   `TestPaperPalette_BWBucket` which records exactly which shades
   collapse to black vs white under the BW threshold; adding a new
   entry doesn't help unless it lands in a Gray4 bucket nothing else
   occupies, and the two device-real shades (`PaperGray70` for the
   dark-gray bucket, anything `PaperGray50`+ for the black side of
   the threshold) already cover the design space.

### When in doubt

Generate two screenshots — the default device view and `?source=1` —
and compare. If a visual decision only reads in the source view and
disappears in the device view, the design has to change, not the
preview. Swap `color_mode` in `inkwell.yaml` between `gray4` and `bw`
to verify both paths; what looks fine on Gray4 can still fragment on
BW (and vice versa for thin gray fills that survive only because of
the Gray4 bucket).

### Refresh mode (flashing)

How often the panel flashes is a separate axis from how a frame is
rendered. The render loop picks a refresh waveform per cycle
(`refreshPlanner` in `refresh.go`): BW does a full-screen fast refresh on
every changed cycle (a single flash) plus a periodic full refresh to clear
ghosting, while Gray4 has no fast waveform and only skips refreshing when
the frame is unchanged. A windowed, flicker-free per-change refresh was
tried and abandoned — the force-drive it needs to redraw changed pixels
cleanly settles the box inverted under the partial waveform on real
hardware (inkwell-6jq). The flash is hardware-only — the web preview can't
show it.

*When* a change is allowed to push is a further axis. The burn-in/waveform
cadence is fixed internally (`defaultFullEvery` in `refresh.go`), not user
config. What the config controls is each widget's
**required** top-level `refresh:` — a duration (>= 1m) or `"static"` — parsed
into `WidgetConfig.Refresh`; there is no widget-code cadence interface and no
default (LoadConfig errors if a widget omits it). A per-screen `refreshSchedule`
(`refresh_queue.go`) gates the planner — a frame change only pushes when a
widget is *due* this minute (wall-clock aligned, so equal cadences coalesce;
static widgets never open the gate). Don't confuse a widget's top-level
`refresh` (render cadence) with `weekly-calendar`'s nested `config.refresh`
(data cache TTL). See
[`docs/tech-specs/08-refresh-strategy.md`](docs/tech-specs/08-refresh-strategy.md).

## Workflow

- All feature and bug fix work **must** use the `/tdd` skill
  (red-green-refactor loop).
- After tests go green, **check coverage** and ensure **100% statement
  coverage** before committing. Add missing tests if coverage is below 100%.
  <!-- markdownlint-disable MD013 -->
  `go test ./... -coverprofile=/tmp/coverage.out && go tool cover -func=/tmp/coverage.out | grep total`
  <!-- markdownlint-enable MD013 -->
- After tests go green and coverage is 100%, **commit immediately** to
  checkpoint progress before moving on to the next task.
- Run `go fix ./...` frequently during development to modernize code
  (e.g., `interface{}` → `any`, if/else → `min`/`max`, loop
  modernization). Run it **before creating any pull request**. Use
  `go fix -diff ./...` first to preview changes when unsure.

## Execution Plan

Task sequencing and acceptance criteria live in
[`inkwell-execution-plan.md`](inkwell-execution-plan.md). Use it as the source
of truth for what to build next and mark tasks complete as you go.

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:ca08a54f -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime`
to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite,
  TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

## Session Completion

**When ending a work session**, you MUST complete ALL steps
below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, `go fix ./...`, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:

   ```bash
   git pull --rebase
   bd dolt push
   git push
   git status  # MUST show "up to date with origin"
   ```

5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**

- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
<!-- END BEADS INTEGRATION -->
