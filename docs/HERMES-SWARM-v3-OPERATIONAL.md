# Hermes Swarm v3 — Operational Handbook

## Daily Pipeline

### Schedule
| Time (UTC) | Agent | Output |
|-------------|-------|--------|
| 06:00 | Keyword Scout | today-keyword.json |
| 06:30 | CMO review | keyword approved |
| 07:30 | Research Agent | research-brief.md |
| 08:00 | Content Writer | draft.md (>=1200 words) |
| 09:00 | QA | approved.md (14/14 checks) |
| 09:45 | Image Creator | article-with-images.md + manifest.json |
| 10:00 | Reviewer | reviewer-approved.md (8-point check) |
| 09:00 (scheduled) | Publisher | article live on WordPress |
| 11:00 | Indexer | Google Indexing API submitted |
| 11:30 | CKO | vault archived |

### Escalation Rules
Escalate to CEO (agentforge-ceo) on:
- 2+ consecutive pipeline failures
- 3+ consecutive substandard articles
- New hire needed
- WP/API credential failures

Escalate to operator (Joerg) on:
- Existential risks
- Budget overruns
- Infrastructure failures

## Agent Profiles

### Content Writer
- Source from research-brief.md ONLY. Never write from memory.
- 1200-1800 words
- Mandatory: GEO definition block, H2 sections, FAQ, quotable summary
- No AI cliches. Direct builder voice.
- [IMAGE:] markers for hero + inline images

### QA
- 14 binary SEO checks
- On pass: write approved.md
- On fail: write draft-rejected.md, create revision task
- Max 2 revision cycles. On 3rd failure: escalate to CMO.

### Image Creator
- FAL_KEY env var. FLUX-schnell model.
- PIL compression before manifest.
- manifest.json with hero/inline roles + alt text.
- Fallback: ~/workspace/fallback/hero-default.png on Fal.ai failure.

### Publisher
- Requires reviewer-approved.md to exist.
- Strip spaces from app password before base64 encode.
- Duplicate slug check.
- Set Yoast OG meta.
- Update cluster-map slot to published.

### Indexer
- Authenticate via service account (GOOGLE_INDEXING_SA_KEY_PATH).
- POST URL_UPDATED.
- Handle 403 (escalate) and 429 (retry 60 min).

## File Conventions
- Drafts: ~/workspace/agentforge/pipeline/drafts/
- Queue: ~/workspace/agentforge/pipeline/queue/
- Published: ~/workspace/agentforge/pipeline/published/
- Images: ~/workspace/agentforge/pipeline/images/
- PDFs: ~/Documents/AgentForge/
- Logs: ~/workspace/agentforge/logs/

## Memory Protocol
All agents contribute to memory after every significant operation:
1. Obsidian vault: ~/obsidian-vault/AgentForge/
2. MeMex wiki: ~/MeMex-Zero-RAG/
3. Pipeline logs: ~/workspace/agentforge/logs/
4. Agent memory: ~/.hermes/profiles/<agent>/MEMORY.md
