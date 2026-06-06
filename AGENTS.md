# Inkwell Development Rules

## Target Hardware — Read This Before Touching Rendering

**Inkwell drives a Waveshare 7.5" V2 e-paper panel. The panel is
fundamentally a 1-bit display.** Forgetting this leads to PRs that look
great in the preview but worse than baseline on real hardware.

What the panel can show:

- **`BW` mode (default, active):** 1 bit per pixel — pure black or pure white.
- **`Init4Gray` mode (supported but not yet wired end-to-end):** four
  levels — white, light gray, dark gray, black. Requires running the
  `Init4Gray` command sequence + 2-bits-per-pixel buffer.

There is **no native 8-level or 12-level grayscale.** The compositor uses
a 12-level `PaperPalette` purely as a *rendering canvas* so widgets can
draw with anti-aliased edges and intermediate grays — but the **packer
collapses that down to what the device actually supports** before any
data leaves the host:

- `packBW` (`internal/inkwell/buffer.go`) — Bayer 4×4 ordered dithering
  to 1 bit per pixel. Soft grays survive as halftone stipple patterns.
- `packGray4` — 4-level luminance buckets. Reserved for the Init4Gray
  device path once it's wired through `EPD`/`SPI`.

### Hard rules when adding visual elements

1. **Always reason about what the *device* will show, not what the
   preview shows.** Open `http://localhost:8080/` and look at the
   default view — that is the post-dither device buffer. Use the
   `Source (design intent)` toggle (or `?source=1`) for the smooth
   grayscale design view; do not rely on it for visual sign-off.
2. **Pure black and pure white are the only luminance levels guaranteed
   to render as-is.** Everything in between becomes stipple patterns
   on hardware. Use that intentionally — but don't expect a flat
   `PaperGray20` fill on a small region (under ~12 px on a side) to
   look like a clean gray, because there aren't enough pixels for the
   pattern to read.
3. **Anti-aliased glyph edges turn into edge stippling on the device.**
   That's fine for body text; for tiny labels (≤10 pt) it can look
   fuzzy. Use `HintingFull` (no AA) when crispness matters more than
   smoothness; `HintingVertical` (the default) keeps vertical stems
   sharp while letting horizontal edges anti-alias.
4. **`PaperBlack` / `PaperWhite` are the only safe choices for 1-px
   strokes.** A 1-px line at `PaperGray40` becomes a dashed dotted
   line on the device after dithering. If you need a soft hairline,
   make it at least 2 px tall (so the dither has room to express a
   gray) or accept the dotted appearance.
5. **Don't add new `PaperGrayNN` entries without testing on the device
   view.** Each new entry must yield a visibly distinct halftone
   pattern in the dithered output, otherwise you're adding palette
   bloat for no on-device benefit.

### When in doubt

Generate two screenshots — the default device view and `?source=1` — and
compare. If a visual decision only reads in the source view and
disappears in the device view, the design has to change, not the
preview.

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
