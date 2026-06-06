# MeMex Wiki Staleness Sweep — 2026-05-17

## Summary

**Status**: ✅ CLEAN
**Files scanned**: 29
**Stale entries** (marked `stale: true`): 0
**Aged entries** (not modified >30 days): 0
**Revalidation queue size**: 0

## Findings

### Explicit Stale Flags
No wiki entries have `stale: true` in their frontmatter.

### Aged Entries (>30 days old)
The wiki is young — all entries are from May 9-14, 2026. Newest entries are only 3 days old. No files exceed the 30-day threshold.

### File Age Breakdown
- **May 14** (3 days old): 3 files (openclaw prompts)
- **May 9** (8 days old): 26 files (core content)

## What Needs Attention

### Potential Candidates for Revalidation (by topic, not age)
The following entries contain references to "stale content" as a concept, but don't require immediate action:

1. **wiki/synthesis/memex-to-llm-wiki.md** — Lists "Flag contradictions and stale content" as a task (meta-documentation, not stale itself)
2. **wiki/concepts/associative-trails.md** — Mentions "Flag broken or stale trails" as part of the associative trail concept
3. **wiki/sources/karpathy-llm-wiki-gist.md** — Documents linting requirements including stale content detection

### Recommendations

1. **No immediate action needed** — All entries are current and within acceptable age windows.
2. **Monitor synthesis/ entries** — The snapshot files (2026-04-23) reference April-era events. These are intentional time-capsule documents, not stale content.
3. **Next sweep**: Run again in 30 days (2026-06-16) to flag entries that cross the 30-day threshold.

## Notes

- The wiki structure is sound with clear categorization (concepts, entities, sources, synthesis, prompts).
- Newest additions are openclaw prompt templates (May 14), showing active development.
- No contradictions or gaps detected in this sweep.

---

**Swept by**: Agent (staleness-sweep)
**Next scheduled**: 2026-06-16
