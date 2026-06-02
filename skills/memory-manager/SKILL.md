---
name: memory-manager
description: Manages the MeMex Zero RAG memory store — search, summarize, compress, archive, and maintain memory files. Use when the user asks to search memory, find past decisions, summarize logs, compress old files, or organize the knowledge base.
when_to_use: |
  - User asks to search memory, find past decisions, or look up information
  - User says "search memory", "what did we decide about", "summarize logs"
  - User asks to compress old daily files, archive, or reorganize knowledge
  - Do NOT use for: writing new memory (that's automatic)
allowed-tools:
  - filesystem
  - shell
capability-required:
  - read
  - write
version: 1.0.0
author: AgentForge
---

# Memory Manager Skill

## Overview

This skill manages the MeMex Zero RAG memory system — the deterministic, file-backed knowledge store used by all AgentForge agents.

## Memory Architecture

```
memory/
├── MEMORY.md           # Long-term curated memory
├── YYYY-MM-DD.md       # Daily raw logs
├── decisions.md        # Decision register
├── agents/{id}/        # Per-agent state
│   ├── state.md
│   └── learnings.md
└── projects/{name}/    # Project context
    └── context.md
```

## Operations

### Search
Query memory across all files:
```
memory search "capability-based security decision"
```
Returns files, snippets, relevance scores. Uses SQLite FTS5 + fallback grep.

### Summarize
Compress a file for context injection:
```
memory summarize decisions.md --max-tokens 500
```
Returns a condensed version for LLM context windows.

### Compress
Deduplicate and archive:
```
memory compress 2026-05-*.md
```
Old daily logs get summarized, raw entries moved to archive.

### Archive
Move files older than N days to archive:
```
memory archive --older-than 90d
```

### Organize
Restructure memory by topic:
```
memory organize --by-project
```

## Git Integration

Every memory operation auto-commits. You can:
- `memory history {file}` — show version history
- `memory diff {commit1} {commit2}` — compare versions
- `memory rollback {commit}` — restore previous version
- `memory push` / `memory pull` — sync with remote

## Best Practices

1. Daily logs are raw — don't edit them
2. MEMORY.md is curated — prune outdated info
3. Decisions get their own file — don't bury them in daily logs
4. Agent learnings auto-append — no manual intervention
5. Git commits provide audit trail — never disable auto-commit in production