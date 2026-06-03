# Fix Plan: Critical Issue #2 - Shell Injection (tool/registry.go:251)

**Status:** Ready for Implementation  
**Severity:** CRITICAL (Command injection via LLM-generated input)  
**File:** `internal/tool/registry.go`  
**Error Lines:** 251 (shell command execution), 212 (AllowedCommands check)  
**Estimated Time:** 30 minutes  

---

## Vulnerability Summary

**The Problem:**
```go
cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)  // 🔴 UNSAFE at line 251
```

The ShellTool accepts a string from the LLM and passes it directly to `sh -c`, enabling arbitrary command injection.

**Attack Scenario:**
1. Agent has `shell` capability with no `AllowedCommands` filter
2. LLM is asked: "Execute to find if user home dir exists"
3. LLM generates: `"ls /home/$(cat /etc/passwd | head -1)"`
4. This gets passed to `sh -c "ls /home/$(cat /etc/passwd | head -1)"`
5. Shell interprets `$(...)` and reads `/etc/passwd`
6. Unintended file access occurs

---

## Root Cause

1. **Line 251:** Uses `sh -c` with untrusted LLM input
2. **Line 212:** `AllowedCommands` field exists but is never validated/enforced
3. **No parsing:** Command string is treated as opaque; no argv parsing

---

## The Fix

### Step 1: Add shlex dependency (already in go.mod or add it)
```bash
go get github.com/google/shlex
```

Check if it's already available:
```bash
grep shlex /Users/joergpeetz/.openclaw/workspace/agentforge/go.mod
```

### Step 2: Modify ShellTool.Execute() method

**Before (Unsafe):**
```go
func (s *ShellTool) Execute(ctx context.Context, args map[string]any) (map[string]any, error) {
    cmdStr, ok := args["command"].(string)
    if !ok {
        return nil, fmt.Errorf("shell: missing 'command' argument")
    }
    // ... timeout/workdir setup ...
    
    cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)  // 🔴 UNSAFE
    // ...
}
```

**After (Safe):**
```go
import "github.com/google/shlex"

func (s *ShellTool) Execute(ctx context.Context, args map[string]any) (map[string]any, error) {
    cmdStr, ok := args["command"].(string)
    if !ok {
        return nil, fmt.Errorf("shell: missing 'command' argument")
    }
    
    // Parse command string as shell tokens (not raw string)
    parts, err := shlex.Split(cmdStr)
    if err != nil {
        return nil, fmt.Errorf("shell: parse error: %w", err)
    }
    if len(parts) == 0 {
        return nil, fmt.Errorf("shell: empty command")
    }
    
    // Enforce whitelist if configured
    if len(s.AllowedCommands) > 0 {
        found := false
        for _, allowed := range s.AllowedCommands {
            if parts[0] == allowed {
                found = true
                break
            }
        }
        if !found {
            return nil, fmt.Errorf("shell: command %q not allowed (whitelist: %v)", parts[0], s.AllowedCommands)
        }
    }
    
    // Use argv form (first arg is command, rest are args) - NO shell interpretation
    cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
    
    // ... rest of implementation (timeout, workdir, pipes, etc.)
}
```

---

## Why This Works

1. **shlex.Split():** Parses shell-like syntax into proper argv array
   - Input: `"ls /home/$(cat /etc/passwd)"`
   - Output: `["ls", "/home/$(cat", "/etc/passwd)"]` (literal strings, NOT interpreted)
   
2. **argv form:** `exec.Command(parts[0], parts[1:]...)` uses direct program execution
   - No shell subprocess to interpret `$()` or other metacharacters
   - Equivalent to how the shell would parse and exec directly
   
3. **Whitelist enforcement:** If `AllowedCommands` is set (e.g., `["ls", "grep", "cat"]`), only those commands can run

---

## Test Cases to Add

### Test 1: Normal command execution still works
```go
args := map[string]any{"command": "ls -la /tmp"}
// Should work: splits to ["ls", "-la", "/tmp"]
// exec.Command("ls", "-la", "/tmp") is valid
```

### Test 2: Command injection is prevented
```go
args := map[string]any{"command": "ls /home/$(cat /etc/passwd)"}
// Should NOT read /etc/passwd
// Becomes exec.Command("ls", "/home/$(cat", "/etc/passwd)") 
// Tries to ls non-existent directory "/home/$(cat"
// Shell metacharacters are literal, not interpreted
```

### Test 3: Whitelist enforcement
```go
s.AllowedCommands = []string{"ls", "grep"}
args := map[string]any{"command": "cat /etc/passwd"}
// Should fail: "cat" not in whitelist
// Error: "shell: command \"cat\" not allowed"
```

### Test 4: Quoted arguments are preserved
```go
args := map[string]any{"command": `echo "hello world"`}
// Should output: hello world
// shlex handles quote parsing correctly
```

---

## Integration with Other Fixes

- **Independent:** No dependencies on Fixes #1, #3, #4
- **Blocked by:** Nothing
- **Enables:** Secure shell tool execution; prerequisite for agent deployment

---

## Validation Checklist

- [ ] shlex import compiles without errors
- [ ] ShellTool.Execute() accepts parsed argv form
- [ ] AllowedCommands whitelist is enforced (test case 3 fails correctly)
- [ ] Normal commands still execute (test case 1 works)
- [ ] Shell metacharacters are NOT interpreted (test case 2 proves this)
- [ ] Quoted arguments work correctly (test case 4)
- [ ] Error messages are clear and include command name + context
- [ ] Existing tests still pass

---

## Files to Modify

| File | Lines | Change | Type |
|------|-------|--------|------|
| `internal/tool/registry.go` | 210-280 (ShellTool.Execute) | Use shlex.Split + argv form, enforce AllowedCommands | Refactor |

## Files to Add/Update

| File | Purpose |
|------|---------|
| `internal/tool/registry_test.go` (or new file) | Add shell injection/whitelist tests |

---

## Commit Message Template

```
Fix: Prevent shell injection in ShellTool by using argv form

Replace unsafe sh -c string execution with shlex parsing and argv form
execution. This prevents LLM-generated shell metacharacters ($(), pipes,
redirects, etc.) from being interpreted by the shell.

Changes:
- Parse command string with shlex.Split() to get proper argv tokens
- Execute with exec.Command(parts[0], parts[1:]...) not sh -c
- Enforce AllowedCommands whitelist if configured
- Add clear error message when command not in whitelist

Security impact:
- Before: "ls /home/$(cat /etc/passwd)" → reads /etc/passwd
- After: Becomes exec.Command("ls", "/home/$(cat", "/etc/passwd)") 
        → fails with "ls: cannot access '/home/$(cat': No such file or directory"

Fixes: Shell injection vulnerability in ShellTool.Execute
```

