# AgentForge HIGH-Priority Fixes - Comprehensive Plan

**Date:** June 3, 2026  
**Scope:** 8 HIGH-priority issues from security audit  
**Status:** Planning & Implementation Ready  
**Approach:** Systematic fix-plan-test-document cycle

---

## Issue Breakdown & Fix Strategy

### Issue #1: Error Handling Antipattern (312+ Ignored Errors)
**Severity:** HIGH  
**Files:** Multiple across codebase  
**Current:** `_, _ := someFunc()` pattern throughout  
**Fix Strategy:** 
- Audit and categorize each ignored error
- Replace with explicit error checks
- Return errors early or handle appropriately
- Add logging for unexpected error cases
**Estimated Time:** 2-3 hours  
**Files to Change:** 10+ files

**Examples to Fix:**
```go
// Before
d, _ := time.ParseDuration(ts)

// After
d, err := time.ParseDuration(ts)
if err != nil {
    return nil, fmt.Errorf("parse timeout: %w", err)
}
```

---

### Issue #2: Race Condition in SSE Hub
**Severity:** HIGH (Concurrency Bug)  
**File:** `internal/sse/sse.go:139-197`  
**Current:** `topicSubs` map accessed without lock  
**Fix Strategy:**
- Analyze current locking pattern
- Add sync.RWMutex protection to topicSubs
- Use proper lock/unlock in Subscribe/Unsubscribe/Publish
- Add race condition tests
**Estimated Time:** 1-2 hours  
**Test:** `go test -race ./...`

---

### Issue #3: Incomplete Memory Store (6 TODOs)
**Severity:** HIGH (Feature Gap)  
**File:** `internal/memory/store.go`  
**Current:** FTS5 search and git integration stubbed  
**Fix Strategy:**
- Implement SQLite FTS5 insert/search
- Complete git integration for versioning
- Implement LLM-based compression
- Add comprehensive memory store tests
**Estimated Time:** 3-4 hours  
**Blockers:** SQLite expertise, git library knowledge

---

### Issue #4: Capability Derivation Validation
**Severity:** HIGH (Auth Security)  
**File:** `internal/security/capability.go:93-116`  
**Current:** Docstring says "must be subset" but no enforcement  
**Fix Strategy:**
- Add validation loop: each derived permission must exist in parent
- Return error if constraint violated
- Add comprehensive tests for valid/invalid derivations
**Estimated Time:** 1 hour  
**Impact:** Prevents privilege escalation in capability chains

---

### Issue #5: No Environment Variable Sanitization
**Severity:** HIGH (Secret Leakage)  
**Files:** `internal/mcpclient/client.go:236`, `internal/api/mcp/manager.go:248`  
**Current:** Shell commands inherit ALL env vars  
**Fix Strategy:**
- Create utility function for env var filtering
- Whitelist safe vars (PATH, HOME, LANG, etc.)
- Block secret vars (API keys, tokens)
- Add tests verifying secret blocking
**Estimated Time:** 1-2 hours  
**Critical:** Prevents OPENAI_KEY leakage to subprocesses

---

### Issue #6: Config Validation Missing
**Severity:** HIGH (Deployment Safety)  
**File:** `internal/config/config.go`  
**Current:** No validation of Port, API keys, schema  
**Fix Strategy:**
- Add validation function to Config struct
- Validate port range (1-65535)
- Validate API keys non-empty
- Validate provider configuration
- Return clear errors for invalid configs
**Estimated Time:** 1-2 hours  
**Benefit:** Catch config errors at startup, not runtime

---

### Issue #7: TODOs in Critical Paths (6 more)
**Severity:** HIGH (Incomplete Implementation)  
**Files:** `internal/engine/agent.go`, `internal/api/mcp/server.go`  
**Current:** Health checks, observability, agent delegation stubbed  
**Fix Strategy:**
- Implement health checks for LLM, memory, MCP
- Wire observability to event bus
- Complete agent delegation logic
- Add health check tests
**Estimated Time:** 2-3 hours  
**Impact:** Enables production monitoring and reliability

---

### Issue #8: No Test Coverage for 6 Modules
**Severity:** HIGH (Coverage Gap)  
**Modules:** `security/*`, `llm/*`, `memory/*`, `session/*`, `cron/*`, `skill/*`  
**Current:** ~10% overall coverage, 0% for these modules  
**Fix Strategy:**
- Write unit tests for each module
- Aim for >80% coverage per module
- Focus on critical paths and error cases
- Add integration tests where appropriate
**Estimated Time:** 4-6 hours  
**Impact:** Catch regressions early, improve reliability

---

## Implementation Sequence

```
Phase 1 (Security & Safety - 4 hours)
├─ Issue #5: Env var sanitization (1-2 hours) ← START HERE
├─ Issue #4: Capability validation (1 hour)
└─ Issue #6: Config validation (1-2 hours)

Phase 2 (Reliability - 5 hours)
├─ Issue #1: Error handling (2-3 hours)
├─ Issue #2: Race conditions (1-2 hours)
└─ Issue #7: TODOs in critical paths (2-3 hours)

Phase 3 (Feature Completeness - 4 hours)
└─ Issue #3: Memory store FTS5 (3-4 hours)

Phase 4 (Test Coverage - 6 hours)
└─ Issue #8: Module test coverage (4-6 hours)

Total Estimated Time: 15-18 hours (2-3 days)
```

---

## Success Criteria for Each Fix

### Fix #1 (Error Handling)
- [ ] No `_, _` patterns in error-returning functions
- [ ] All errors explicitly checked or handled
- [ ] 100+ error checks added
- [ ] Existing tests still pass

### Fix #2 (Race Conditions)
- [ ] `go test -race ./...` passes
- [ ] SSE Hub has sync.RWMutex on all shared state
- [ ] New race condition tests added
- [ ] No data races detected

### Fix #3 (Memory Store)
- [ ] FTS5 search returns results
- [ ] Git integration tracks changes
- [ ] LLM compression works
- [ ] Memory store tests pass (>80% coverage)

### Fix #4 (Capability Validation)
- [ ] Invalid derivations rejected with error
- [ ] Valid derivations succeed
- [ ] Privilege escalation prevented
- [ ] 5+ validation tests pass

### Fix #5 (Env Var Sanitization)
- [ ] Secret env vars blocked from subprocesses
- [ ] Safe vars (PATH, HOME) preserved
- [ ] Tests verify blocking
- [ ] No OPENAI_KEY in subprocess env

### Fix #6 (Config Validation)
- [ ] Invalid port rejected
- [ ] Empty API key rejected
- [ ] Invalid provider rejected
- [ ] Errors caught at startup

### Fix #7 (Critical Path TODOs)
- [ ] Health checks functional
- [ ] Observability wired to bus
- [ ] Agent delegation complete
- [ ] 10+ new tests added

### Fix #8 (Test Coverage)
- [ ] >80% coverage for each module
- [ ] Critical paths tested
- [ ] Error cases covered
- [ ] Overall coverage >50%

---

## Documentation Plan

For each fix, create:
1. **Detailed fix plan** (like FIXPLAN_CRITICAL_*.md)
2. **Implementation notes** (what changed, why)
3. **Test documentation** (test cases added)
4. **Integration notes** (how it affects other systems)

Final deliverable: `HIGH_PRIORITY_FIXES_COMPLETE.md`

---

## Risk Assessment

| Issue | Risk | Mitigation |
|-------|------|-----------|
| Error handling (widespread) | Regression | Comprehensive testing |
| Race conditions | Data corruption | Race detector, tests |
| Memory store | Data loss | Backup before changes |
| Capability validation | Auth bypass | Security review |
| Env sanitization | Secret leak | Whitelist approach |
| Config validation | Misconfiguration | Early detection |
| Critical TODOs | Incomplete features | Full implementation |
| Test coverage | Hidden bugs | Targeted testing |

---

## Next Steps

1. **Start with Fix #5 (Env Sanitization)** - Security-critical, smallest scope
2. **Move to Fix #4 (Capability Validation)** - Auth security, small scope
3. **Fix #6 (Config Validation)** - Deployment safety
4. **Continue through remaining issues in priority order**

All changes will be:
- ✅ Comprehensively tested
- ✅ Documented in detail
- ✅ Committed with clear messages
- ✅ Tracked in this document

---

**Ready to begin HIGH-priority fixes** 🚀
