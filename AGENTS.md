# Inkwell Development Rules

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
