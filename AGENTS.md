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
   it reads on neither mode. Use 1–2 px `PaperBlack` strokes for
   indicators. **Burn-in:** don't fill a large area in a fixed position
   — a black block that lands in the same place every refresh invites
   ghosting on the panel. Today is shown by position (first column or
   first row), never by a fill or outline. A filled shape is fine when
   it moves with the content, such as an upcoming-event block on the
   day-timeline. `PaperGrayNN` fills are only useful when the region
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
5. **Above the 20 px tier, scale the mask — don't reach for a bigger
   font.** `fonts.ScaledDrawer` blows a glyph's 1-bit mask up by an
   integer factor, which is still a 1-bit mask, so both packers stay
   safe at any size. Size and weight are separate axes: scaling leaves
   Tamzen Bold's 20% stem ratio untouched, so a 60 px numeral has the
   stroke weight of body text unless it is also dilated. Take the
   radius from `fonts.GrowFor(scale)` (2 from 3x up, 1 at 2x, 0 at 1x)
   — the counter runs out before the stem does, and the slashed zero
   fills in first. `internal/inkwell/fonts/testdata/` has a golden per
   scale/grow pair if you need to see one.
6. **Don't add new `PaperGrayNN` entries.** The palette is pinned by
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

Before touching refresh waveforms, panel sleep, the init sequences in
`profile.go`, or the per-widget `refresh:` gate, read
[`docs/guides/refresh.md`](docs/guides/refresh.md). Several of those choices
look tunable against the preview and are not.

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

## Agent skills

### Issue tracker

Issues live in GitHub Issues for `grantlucas/inkwell`, driven by the `gh` CLI.
See `docs/agents/issue-tracker.md`.

### Triage labels

The five default triage roles, each label named after its role (`needs-triage`,
`needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See
`docs/agents/triage-labels.md`.

### Domain docs

Single-context: one root `GLOSSARY.md` (created lazily) plus the ADRs in
`docs/adrs/`. See `docs/agents/domain.md`.

## Session Completion

**When ending a work session**, you MUST complete ALL steps
below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **Note remaining work** - Open a GitHub issue for anything that needs
   follow-up
2. **Run quality gates** (if code changed) - Tests, linters, `go fix ./...`, builds
3. **PUSH TO REMOTE** - This is MANDATORY:

   ```bash
   git pull --rebase
   git push
   git status  # MUST show "up to date with origin"
   ```

4. **Clean up** - Clear stashes, prune remote branches
5. **Verify** - All changes committed AND pushed
6. **Hand off** - Provide context for next session

**CRITICAL RULES:**

- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
