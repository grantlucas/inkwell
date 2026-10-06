# Domain Docs

How the engineering skills should consume this repo's domain documentation when
exploring the codebase.

## Before exploring, read these

- **`GLOSSARY.md`** at the repo root — the glossary.
- **`docs/adrs/`** — read ADRs that touch the area you're about to work in.
  Start from the index table in
  [`docs/adrs/README.md`](../adrs/README.md) to find the relevant ones.

If any of these files don't exist, **proceed silently**. Don't flag their
absence; don't suggest creating them upfront. The `/domain-modeling` skill
(reached via `/grill-with-docs` and `/improve-codebase-architecture`) creates
them lazily when terms or decisions actually get resolved.

## ADRs live in `docs/adrs/`, not `docs/adr/`

This repo's ADRs predate these skills and use the plural directory. When a
skill would write a new ADR to `docs/adr/`, write it to `docs/adrs/` instead:
take the next free number, copy the shape of an existing file, and add a row to
the index in `docs/adrs/README.md`.

## File structure

Single-context repo:

```text
/
├── GLOSSARY.md
├── docs/adrs/
│   ├── README.md                       ← index
│   ├── 0001-target-waveshare-7in5-v2-on-a-pi-zero-2w.md
│   └── 0012-sleep-the-panel-between-refreshes.md
└── internal/
```

If the repo ever splits into multiple contexts, a root `GLOSSARY-MAP.md`
pointing at one `GLOSSARY.md` per context is the switch.

## Use the glossary's vocabulary

When your output names a domain concept (in an issue title, a refactor proposal,
a hypothesis, a test name), use the term as defined in `GLOSSARY.md`. Don't
drift to synonyms the glossary explicitly avoids.

If the concept you need isn't in the glossary yet, that's a signal — either
you're inventing language the project doesn't use (reconsider) or there's a real
gap (note it for `/domain-modeling`).

## Flag ADR conflicts

If your output contradicts an existing ADR, surface it explicitly rather than
silently overriding:

> _Contradicts ADR-0012 (sleep the panel between refreshes) — but worth
> reopening because…_

An ADR describes the decision as of its date. When the code and an ADR
disagree, the code wins — and the fix is a new ADR that supersedes the old one,
not an edit that quietly rewrites it.
