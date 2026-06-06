# AgentForge Goals & Kanban

## Founding Goal
**90 articles. 50,000 organic clicks. Profitable in 90 days.**

### Milestone 1 — Month 1 (Days 1-30)
- [ ] 30 articles published
- [ ] Google indexes domain
- [ ] 1,000 organic clicks
- [ ] AdSense approved

### Milestone 2 — Month 2 (Days 31-60)
- [ ] 60 articles total
- [ ] Apply to Ezoic at ~10K sessions
- [ ] 15,000 organic clicks

### Milestone 3 — Month 3 (Days 61-90)
- [ ] 90 articles total
- [ ] Apply to Mediavine at 50K sessions
- [ ] 50,000 organic clicks

## Success Criteria
- 90 articles indexed
- 50K+ organic clicks
- Mediavine application submitted
- Zero missed days

## Kanban Board
Task board: ~/.hermes/kanban.db

### Active Tasks (as of 2026-05-14)
| ID | Title | Assignee | Status |
|----|-------|----------|--------|
| t_30b056ee | Launch AgentForge (founding goal) | agentforge-ceo | running |
| t_3a6b864d | Hiring Manager: deploy all agents | hiring-manager | todo |
| t_3d4a62c6 | CKO: knowledge capture + bundles | cko | todo |
| t_43773271 | Content Department: daily pipeline | agentforge-cmo | todo |
| t_6d11fa3a | WPMD: WordPress build | wpmd-cmo | todo |

### Content Pipeline Agents (children of t_43773271)
| ID | Title | Assignee | Status |
|----|-------|----------|--------|
| t_ebb7c785 | Keyword Scout | keyword-scout | todo |
| t_7e183f19 | Research Agent | research-agent | todo |
| t_aa9e075b | Content Writer | content-writer | todo |
| t_03ecc39a | QA | qa | todo |
| t_12bbedcf | Image Creator | image-creator | todo |
| t_7847eadd | Reviewer | reviewer | todo |
| t_3f79a340 | Publisher | publisher | todo |
| t_a936a293 | Indexer | indexer | todo |

## Pipeline Status (2026-05-14)
- Articles in queue: 4 (May 10, 11, 13, 14) — all QA passed
- Articles published to WordPress: 0
- **CRITICAL BLOCKER: agent-forge.co returns NXDOMAIN** — domain does not resolve
- WordPress credentials not configured in publisher profile
- All child tasks in todo, zero runs completed
- Agent profiles exist but not all kanban assignees have matching profiles

## Critical Issues (2026-05-14)
1. **Domain down**: agent-forge.co NXDOMAIN — needs operator intervention (IONOS VPS / DNS)
2. **No agent deployments**: Hiring manager hasn't run, no agents deployed
3. **Profile mismatch**: Kanban assignees (publisher, indexer, etc.) don't match existing profile names exactly
4. **Dispatcher idle**: No child tasks have been spawned despite being in todo
