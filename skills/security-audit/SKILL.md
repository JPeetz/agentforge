---
name: security-audit
description: Performs capability-based security audits of agent configurations, tool permissions, and memory access patterns. Use when the user asks to audit security, check permissions, review capabilities, or verify agent safety.
when_to_use: |
  - User asks to audit security, review permissions, or verify agent safety
  - User says "security audit", "capability check", "permission review", "is this safe"
  - Do NOT use for: general code review (use code-review skill)
allowed-tools:
  - filesystem
  - shell
  - http
capability-required:
  - read
version: 1.0.0
author: AgentForge
---

# Security Audit Skill

## Overview

This skill audits AgentForge agent configurations against security best practices. It checks capability tokens, tool permissions, resource allowlists, and memory access patterns.

## Audit Checklist

### 1. Capability Token Audit
- Verify all agents have capability tokens (no ambient authority)
- Check token budgets are not excessive
- Verify timeouts are set
- Check delegation chains for privilege escalation

### 2. Resource Allowlist Audit
- Filesystem paths are restrictive (not `/` or `$HOME`)
- Network domains are explicit (not `*`)
- Operations are minimal (read-only where possible)

### 3. Tool Permission Audit
- No agent has `exec` + `write` + `net` simultaneously
- Shell tool has command allowlisting
- HTTP tool has domain allowlisting

### 4. Memory Access Audit
- Agent memory paths are scoped to department
- No cross-department memory access without explicit delegation
- Git history is being maintained

### 5. Subagent Audit
- Max depth is reasonable (≤ 5)
- Child capabilities are strict subsets
- No recursive spawning

## Output Format

```
## Security Audit: {agent-name}

### Capability Token
- Budget: {tokens} | Timeout: {duration} | Signature: valid ✅/❌

### Resources
- Filesystem: {paths} | Scope: {adequate/too broad}
- Network: {domains} | Scope: {adequate/too broad}

### Tools
- {tool}: {permissions} | Risk: {low/medium/high}

### Memory
- Path: {path} | Cross-dept access: {yes/no}

### Subagents
- Max depth: {N} | Delegation chain: {valid/invalid}

## Risk Score: {0-100}

## Recommendations
1. {action}
2. {action}
```