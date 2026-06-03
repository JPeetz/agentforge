# Fix Plan: Critical Issue #3 - Silent Error Drops in Pipe Operations

**Status:** Ready for Implementation  
**Severity:** CRITICAL (Data loss, unexpected behavior, potential panics)  
**File:** `internal/tool/registry.go`  
**Error Lines:** 256, 263-264 (silent error discards)  
**Estimated Time:** 15 minutes  

---

## Problem Summary

**The Issue:**
```go
stdout, _ := cmd.StdoutPipe()     // 🔴 Error discarded (line 256)
stderr, _ := cmd.StderrPipe()     // 🔴 Error discarded

// ...

outBytes, _ := io.ReadAll(stdout) // 🔴 Error discarded (line 263)
errBytes, _ := io.ReadAll(stderr) // 🔴 Error discarded (line 264)
```

When pipe setup fails, `stdout` or `stderr` become `nil`. Passing `nil` to `io.ReadAll()` causes a panic or silently returns empty data, masking the failure.

**Attack/Failure Scenarios:**

1. **Resource exhaustion:** System runs out of file descriptors
   - `StdoutPipe()` fails → stdout is nil
   - `io.ReadAll(nil)` → panic or zero output
   - Agent never knows command partially failed

2. **I/O error during streaming:** Network issue, disk full
   - `io.ReadAll()` returns error (e.g., "connection reset")
   - Error discarded with `_`
   - Tool output silently truncated
   - Agent assumes tool ran successfully

3. **Command fails with diagnostic info in stderr**
   - `io.ReadAll()` succeeds but stderr has error message
   - Error message lost if read fails

---

## Root Cause

The code uses the blank identifier `_` to ignore errors from:
1. `cmd.StdoutPipe()` - creates a reader for command's stdout
2. `cmd.StderrPipe()` - creates a reader for command's stderr  
3. `io.ReadAll(stdout)` - reads all data from reader
4. `io.ReadAll(stderr)` - reads all data from reader

If ANY of these fail, the failure is silent. The tool execution appears to succeed even though output may be missing or corrupted.

---

## The Fix

### Pattern: Check All Errors, Return Early

**Before (Unsafe):**
```go
stdout, _ := cmd.StdoutPipe()
stderr, _ := cmd.StderrPipe()

if err := cmd.Start(); err != nil {
    return nil, fmt.Errorf("shell: start: %w", err)
}

outBytes, _ := io.ReadAll(stdout)   // If stdout is nil, panics or returns empty
errBytes, _ := io.ReadAll(stderr)   // Same

return map[string]any{
    "stdout": string(outBytes),
    "stderr": string(errBytes),
}, nil
```

**After (Safe):**
```go
stdout, err := cmd.StdoutPipe()
if err != nil {
    return nil, fmt.Errorf("shell: stdout pipe: %w", err)
}

stderr, err := cmd.StderrPipe()
if err != nil {
    return nil, fmt.Errorf("shell: stderr pipe: %w", err)
}

if err := cmd.Start(); err != nil {
    return nil, fmt.Errorf("shell: start: %w", err)
}

outBytes, err := io.ReadAll(stdout)
if err != nil {
    return nil, fmt.Errorf("shell: read stdout: %w", err)
}

errBytes, err := io.ReadAll(stderr)
if err != nil {
    return nil, fmt.Errorf("shell: read stderr: %w", err)
}

return map[string]any{
    "stdout": string(outBytes),
    "stderr": string(errBytes),
}, nil
```

---

## What Gets Fixed

### Error Path 1: StdoutPipe() failure
- **Before:** `stdout` is nil, error ignored, `io.ReadAll(nil)` has undefined behavior
- **After:** Returns error early: `"shell: stdout pipe: too many open files"`
- **Agent sees:** Tool execution failed (correct)

### Error Path 2: StderrPipe() failure  
- **Before:** `stderr` is nil, error ignored, `io.ReadAll(nil)` has undefined behavior
- **After:** Returns error early: `"shell: stderr pipe: device not ready"`
- **Agent sees:** Tool execution failed (correct)

### Error Path 3: io.ReadAll() on stdout fails
- **Before:** Error ignored, agent gets partial/corrupted output
- **After:** Returns error: `"shell: read stdout: connection reset by peer"`
- **Agent sees:** Tool execution failed (correct)

### Error Path 4: io.ReadAll() on stderr fails
- **Before:** Error ignored, stderr output lost  
- **After:** Returns error: `"shell: read stderr: I/O error"`
- **Agent sees:** Tool execution failed, diagnostic data lost (but at least knows it failed)

---

## Test Cases to Add

### Test 1: Normal command execution (baseline)
```go
// Should work fine - all operations succeed
result, err := tool.Execute(ctx, map[string]any{"command": "echo hello"})
// Expect: err == nil, result["stdout"] == "hello\n"
```

### Test 2: Verify errors are NOT ignored
```go
// Setup: Mock exec.Command to fail at StdoutPipe()
result, err := tool.Execute(ctx, ...)
// Expect: err != nil, error message contains "stdout pipe"
// Expect: NOT panic, NOT empty result
```

### Test 3: stderr is properly captured
```go
// Command that writes to both stdout and stderr
// echo "good" && echo "bad" >&2
result, err := tool.Execute(ctx, {"command": "sh -c 'echo good && echo bad >&2'"})
// Expect: result["stdout"] contains "good"
// Expect: result["stderr"] contains "bad"
```

### Test 4: Command failure is communicated
```go
// Command that exits with error
result, err := tool.Execute(ctx, {"command": "false"})
// Expect: result may be populated but cmd.Wait() error should be checked/returned
```

---

## Integration with Other Fixes

- **Depends on:** Fix #2 (shell injection) - comes after parsing
- **Independent of:** Fixes #1, #4
- **Enables:** Reliable tool execution with proper error reporting

---

## Validation Checklist

- [ ] All `_` error assignments in ShellTool.Execute() are replaced with explicit error checks
- [ ] Each error check returns early with wrapped error message
- [ ] Error messages include context (which operation failed)
- [ ] Test 1 passes (normal execution unchanged)
- [ ] Test 2 fails if errors still being ignored (verifies we're checking)
- [ ] Test 3 captures both stdout and stderr correctly
- [ ] Test 4 shows command errors are propagated
- [ ] Existing tests still pass
- [ ] No panics on I/O errors

---

## Files to Modify

| File | Lines | Change | Type |
|------|-------|--------|------|
| `internal/tool/registry.go` | 256, 263-264 | Replace `_, _` with explicit error checks | Error handling |

---

## Commit Message Template

```
Fix: Explicitly check pipe and I/O errors in ShellTool.Execute

Replace silent error ignores (_) with explicit error checks in pipe setup
and output reading. This ensures tool execution failures are detected and
reported rather than silently producing corrupted/partial output.

Changes:
- Check StdoutPipe() error, return early if it fails
- Check StderrPipe() error, return early if it fails
- Check io.ReadAll(stdout) error, return early if it fails
- Check io.ReadAll(stderr) error, return early if it fails
- Wrap all errors with context (e.g., "shell: read stdout: %w")

Impact:
- Before: Resource exhaustion → nil → panic or silent failure
- After: Resource exhaustion → explicit error → agent knows tool failed

Fixes: Silent error drops in pipe operations
```

---

## Notes

- **Silent errors pattern is dangerous:** In Go, explicit error handling prevents subtle bugs
- **From Go idioms:** `if err != nil { return err }` is the standard pattern
- **Tool reliability:** Users depend on tools returning accurate output; silent failures are the worst case
- **Debugging:** Clear error messages help diagnose deployment issues

