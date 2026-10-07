# Issue tracker: GitHub

Issues and PRDs for this repo live as GitHub issues. Use the `gh` CLI for all
operations.

## Conventions

- **Create an issue**: `gh issue create --title "..." --body "..."`. Use a
  heredoc for multi-line bodies.
- **Read an issue**: `gh issue view <number> --comments`, filtering comments by
  `jq` and also fetching labels.
- **List issues**: with appropriate `--label` and `--state` filters:

  ```bash
  gh issue list --state open --json number,title,body,labels,comments --jq '[.[] | {number, title, body, labels: [.labels[].name], comments: [.comments[].body]}]'
  ```

- **Comment on an issue**: `gh issue comment <number> --body "..."`
- **Apply / remove labels**: `gh issue edit <number> --add-label "..."` /
  `--remove-label "..."`
- **Close**: `gh issue close <number> --comment "..."`

Infer the repo from `git remote -v` — `gh` does this automatically when run
inside a clone.

## Pull requests as a triage surface

**PRs as a request surface: no.** _(Set to `yes` if this repo treats external
PRs as feature requests; `/triage` reads this flag.)_

When set to `yes`, PRs run through the same labels and states as issues, using
the `gh pr` equivalents:

- **Read a PR**: `gh pr view <number> --comments` and `gh pr diff <number>` for
  the diff.
- **List external PRs for triage**: run the command below, then keep only
  `authorAssociation` of `CONTRIBUTOR`, `FIRST_TIME_CONTRIBUTOR`, or `NONE`
  (drop `OWNER`/`MEMBER`/`COLLABORATOR`).

  ```bash
  gh pr list --state open --json number,title,body,labels,author,authorAssociation,comments
  ```

- **Comment / label / close**: `gh pr comment`, `gh pr edit
  --add-label`/`--remove-label`, `gh pr close`.

GitHub shares one number space across issues and PRs, so a bare `#42` may be
either — resolve with `gh pr view 42` and fall back to `gh issue view 42`.

## Design references

Agents can't open private links (a claude.ai artifact, a Figma file shared
only with the owner). When a spec or ticket rests on a mockup, put the images
where every agent can read them, and embed them in the issue body or a
comment:

- Push the images to an orphan branch named `assets/<spec>-design-reference`
  and embed them by raw URL,
  `https://raw.githubusercontent.com/grantlucas/inkwell/<branch>/<path>`.
  `gh` can't attach images to an issue, and the orphan branch keeps them out
  of `main`.
- Keep the private link beside them only as a pointer for the owner.
- Say on the image that mockups are drawn at twice the panel's resolution in
  a stand-in typeface: agents take structure from them, never pixel sizes.

## Integration branches

A spec built by several agents at once lands on one integration branch
(`feat/<spec>-<slug>`), and every ticket still gets its own PR, because the
release notes are built from merged PRs.

- A ticket PR targets the integration branch and is merged with
  `gh pr merge --merge`. Name its ticket with neutral wording, such as
  "Ticket: #N, which closes when the integration branch merges to main".
  Leave out `Closes`, `Fixes` and `Resolves`: GitHub ignores closing keywords
  on PRs into a non-default branch.
- The integration PR into `main` carries the `Closes #N` lines for every
  ticket it finishes, and is opened as a draft after the first ticket lands.
- A ticket done only in part (a step deliberately left for later) gets
  "Part of #N" and stays open with a comment listing what remains.

## When a skill says "publish to the issue tracker"

Create a GitHub issue.

## When a skill says "fetch the relevant ticket"

Run `gh issue view <number> --comments`.

## Wayfinding operations

Used by `/wayfinder`. The **map** is a single issue with **child** issues as
tickets.

- **Map**: a single issue labelled `wayfinder:map`, holding the Notes /
  Decisions-so-far / Fog body. `gh issue create --label wayfinder:map`.
- **Child ticket**: an issue linked to the map as a GitHub sub-issue (`gh api`
  on the sub-issues endpoint). Where sub-issues aren't enabled, add the child to
  a task list in the map body and put `Part of #<map>` at the top of the child
  body. Labels: `wayfinder:<type>` (`research`/`prototype`/`grilling`/`task`).
  Once claimed, the ticket is assigned to the driving dev.
- **Blocking**: GitHub's **native issue dependencies** — the canonical,
  UI-visible representation. Add an edge with the command below, where
  `<blocker-db-id>` is the blocker's numeric **database id**
  (`gh api repos/<owner>/<repo>/issues/<n> --jq .id`, _not_ the `#number` or
  `node_id`). GitHub reports `issue_dependencies_summary.blocked_by` (open
  blockers only — the live gate). Where dependencies aren't available, fall
  back to a `Blocked by: #<n>, #<n>` line at the top of the child body. A
  ticket is unblocked when every blocker is closed.

  ```bash
  gh api --method POST repos/<owner>/<repo>/issues/<child>/dependencies/blocked_by -F issue_id=<blocker-db-id>
  ```

- **Frontier query**: list the map's open children (`gh issue list --state
  open`, scoped to the map's sub-issues / task list), drop any with an open
  blocker (`issue_dependencies_summary.blocked_by > 0`, or an open issue in the
  `Blocked by` line) or an assignee; first in map order wins.
- **Claim**: `gh issue edit <n> --add-assignee @me` — the session's first
  write.
- **Resolve**: `gh issue comment <n> --body "<answer>"`, then
  `gh issue close <n>`, then append a context pointer (gist + link) to the
  map's Decisions-so-far.
