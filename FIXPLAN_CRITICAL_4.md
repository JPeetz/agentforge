# Fix Plan: Critical Issue #4 - Glob Pattern Matching Not Implemented

**Status:** Ready for Implementation  
**Severity:** CRITICAL (Security boundary broken)  
**File:** `internal/security/capability.go`  
**Error Line:** 220 (TODO comment)  
**Estimated Time:** 30 minutes  

---

## Vulnerability Summary

**The Problem:**
```go
func (e *Enforcer) resourceAllowed(cap *Capability, resource string, op Operation) bool {
    for _, r := range cap.Resources {
        // TODO: implement glob pattern match (path.Match / net.ParseCIDR)
        if r.Path != "" && r.Path == resource {  // 🔴 Only exact match works!
            // Only exact match works
            for _, p := range r.Operations {
                // ...
            }
        }
    }
    return false
}
```

**The Issue:**
- Documentation claims glob pattern support (e.g., `/home/user/*`)
- Implementation only does exact string matching
- Users who think they have broad access find their access denied

**User Impact Example:**
```
Capability grants: /home/user/*  (intended: all files under /home/user)
User tries:        /home/user/secret.txt
Result:            ACCESS DENIED (because "/home/user/*" != "/home/user/secret.txt")
Workaround:        User adds exact path for each file (defeats the point)
```

---

## Root Cause

The `resourceAllowed()` method has an explicit TODO that was never implemented. Line 220 says:
```go
// TODO: implement glob pattern match (path.Match / net.ParseCIDR)
```

This indicates the feature was planned but left unfinished. The code only checks for exact string equality (`r.Path == resource`), so glob patterns like `*.txt` or `/home/*/docs` never match.

---

## The Fix

### Step 1: Add filepath import (already in stdlib)

```go
import "path/filepath"
```

### Step 2: Modify resourceAllowed() method

**Before (Incomplete):**
```go
func (e *Enforcer) resourceAllowed(cap *Capability, resource string, op Operation) bool {
    for _, r := range cap.Resources {
        // TODO: implement glob pattern match (path.Match / net.ParseCIDR)
        if r.Path != "" && r.Path == resource {  // Only exact match
            for _, p := range r.Operations {
                for _, perm := range p.Permissions {
                    if perm == op.Permission {
                        return true
                    }
                }
            }
        }
    }
    return false
}
```

**After (Complete):**
```go
func (e *Enforcer) resourceAllowed(cap *Capability, resource string, op Operation) bool {
    for _, r := range cap.Resources {
        if r.Path == "" {
            continue  // Skip empty paths
        }
        
        // Try exact match first (fast path)
        if r.Path == resource {
            if e.operationAllowed(r, op) {
                return true
            }
            continue
        }
        
        // Try glob pattern match
        match, err := filepath.Match(r.Path, resource)
        if err != nil {
            // Invalid glob pattern in capability — skip it
            continue
        }
        if match {
            if e.operationAllowed(r, op) {
                return true
            }
        }
    }
    return false
}

// Helper to reduce duplication
func (e *Enforcer) operationAllowed(r ResourceRule, op Operation) bool {
    for _, p := range r.Operations {
        for _, perm := range p.Permissions {
            if perm == op.Permission {
                return true
            }
        }
    }
    return false
}
```

---

## How filepath.Match Works

### Examples

```go
filepath.Match("/home/user/*", "/home/user/secret.txt")
// → true (matches any file under /home/user)

filepath.Match("/home/user/*.txt", "/home/user/doc.pdf")
// → false (glob requires .txt extension)

filepath.Match("/home/user/*.txt", "/home/user/readme.txt")
// → true (matches .txt files)

filepath.Match("/etc/pass?", "/etc/passwd")
// → true (? matches single character)

filepath.Match("/etc/**/config", "/etc/app/config")
// → false (filepath.Match doesn't support ** recursively; use your own logic if needed)

filepath.Match("/var/log/*.log", "/var/log/2026-06-03.log")
// → true (matches any .log file)
```

### Pattern Syntax
- `*` - Match any sequence of non-separator characters (0 or more)
- `?` - Match any single non-separator character
- `[...]` - Character range `[a-z]` matches a-z
- `\` - Escape special characters

---

## Test Cases to Add

### Test 1: Exact match still works (backward compatibility)
```go
cap := Issue("agent1", []Permission{PermRead}, 
    WithResource("/home/user/secret.txt", PermRead))
    
// Should work: exact match
allowed := enforcer.Eval(cap, "/home/user/secret.txt", PermRead)
// Expect: allowed == true
```

### Test 2: Glob pattern with wildcard
```go
cap := Issue("agent2", []Permission{PermRead},
    WithResource("/home/user/*", PermRead))

// Should work: glob matches
allowed1 := enforcer.Eval(cap, "/home/user/secret.txt", PermRead)
allowed2 := enforcer.Eval(cap, "/home/user/readme.md", PermRead)
// Expect: both == true

// Should fail: different directory
allowed3 := enforcer.Eval(cap, "/home/other/secret.txt", PermRead)
// Expect: allowed3 == false
```

### Test 3: Glob pattern with extension filter
```go
cap := Issue("agent3", []Permission{PermRead},
    WithResource("/var/log/*.log", PermRead))

// Should work: matches .log files
allowed1 := enforcer.Eval(cap, "/var/log/app.log", PermRead)
allowed2 := enforcer.Eval(cap, "/var/log/2026-06-03.log", PermRead)
// Expect: both == true

// Should fail: not .log extension
allowed3 := enforcer.Eval(cap, "/var/log/app.txt", PermRead)
// Expect: allowed3 == false
```

### Test 4: Single character match
```go
cap := Issue("agent4", []Permission{PermRead},
    WithResource("/etc/pass?", PermRead))

// Should work: ? matches single char
allowed := enforcer.Eval(cap, "/etc/passwd", PermRead)
// Expect: allowed == true
```

### Test 5: Character range match
```go
cap := Issue("agent5", []Permission{PermRead},
    WithResource("/data/[a-c].txt", PermRead))

// Should work: in range
allowed1 := enforcer.Eval(cap, "/data/a.txt", PermRead)
allowed2 := enforcer.Eval(cap, "/data/b.txt", PermRead)
// Expect: both == true

// Should fail: outside range
allowed3 := enforcer.Eval(cap, "/data/d.txt", PermRead)
// Expect: allowed3 == false
```

### Test 6: Invalid glob pattern handling (graceful degradation)
```go
cap := Issue("agent6", []Permission{PermRead},
    WithResource("[invalid", PermRead))  // Missing closing ]

// Should not crash, should gracefully skip invalid pattern
allowed := enforcer.Eval(cap, "/[invalid", PermRead)
// Expect: allowed == false (pattern doesn't match, no panic)
```

---

## Integration with Other Fixes

- **Independent:** No dependencies on Fixes #1-3
- **Depends on:** Nothing
- **Enables:** Proper capability resource scoping; essential for principle of least privilege

---

## Validation Checklist

- [ ] filepath.Match import added
- [ ] Exact match path still works (backward compat)
- [ ] Glob patterns with `*` work correctly
- [ ] Glob patterns with `?` work correctly
- [ ] Character ranges `[a-z]` work correctly
- [ ] Invalid glob patterns don't cause panic
- [ ] Test 1-6 all pass
- [ ] Existing capability tests still pass
- [ ] No performance regression (exact match is still checked first as fast path)

---

## Files to Modify

| File | Lines | Change | Type |
|------|-------|--------|------|
| `internal/security/capability.go` | 220+ (resourceAllowed) | Add filepath.Match fallback, extract operationAllowed helper | Implementation |

## Files to Add/Update

| File | Purpose |
|------|---------|
| `internal/security/capability_test.go` | Add glob pattern matching tests (test 1-6 above) |

---

## Commit Message Template

```
Fix: Implement glob pattern matching in capability resource enforcement

Replace incomplete exact-match-only implementation with proper glob pattern
support using filepath.Match. This completes the security boundary for
resource allowlists.

Changes:
- Add filepath.Match() check after exact match (fast path)
- Support glob patterns: *, ?, [a-z] syntax
- Invalid patterns gracefully skip (continue) without panicking
- Extract operationAllowed helper to reduce duplication

Feature impact:
- Before: WithResource("/home/user/*", ...) only matched exact path
- After: WithResource("/home/user/*", ...) matches /home/user/any/file

Security impact:
- Principle of least privilege now works correctly
- Admins can use broad patterns instead of exact paths
- Fine-grained permission model is now fully functional

Fixes: TODO in capability.go:220 for glob pattern matching
```

---

## Notes on filepath.Match

### Limitations
- Does NOT support `**` (recursive glob) - only flat directories
- Uses Unix path separators even on Windows (use `/` not `\`)
- Returns error if pattern is malformed (we catch and skip)

### Alternatives Considered
- `regexp.MatchString()` - Too powerful, regex syntax confusing for policy
- Custom glob - Reinventing wheel, harder to maintain
- `doublestar` package - Would add dependency, filepath.Match is good enough for 80% of use cases

---

## Performance Note

Adding glob matching adds O(N) filepath.Match calls where N = number of resource rules. In practice:
- Typical capability has 3-10 resource rules (small)
- Exact match is checked first (fast path, avoids glob)
- filepath.Match is O(pattern length), pattern is typically < 100 chars
- Overall impact negligible

