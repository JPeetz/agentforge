---
name: code-review
description: Reviews code for bugs, security issues, and style violations. Use when the user asks to review code, check a PR, find issues, audit, or code quality.
when_to_use: |
  - User asks to review code, check for bugs, or audit a file
  - User opens a PR and asks for feedback
  - User says "review", "check", "audit", "find issues", "code quality"
  - Do NOT use for: formatting-only or linting-only requests
argument-hint: Provide the file path or code block to review.
allowed-tools:
  - filesystem
  - shell
capability-required:
  - read
version: 1.0.0
author: AgentForge
license: MIT
tags:
  - code
  - review
  - security
  - quality
---

# Code Review Skill

## Overview

This skill performs systematic code review focusing on bugs, security vulnerabilities, and style violations. It follows a structured checklist approach to ensure nothing is missed.

## Review Process

### 1. Security Check (highest priority)
- Injection vulnerabilities (SQL, command, path traversal)
- Authentication bypass patterns
- Data exposure (secrets, PII in logs)
- Unsafe deserialization
- Insecure default configurations

### 2. Logic Errors
- Off-by-one errors
- Null/nil dereference potential
- Race conditions
- Infinite loops
- Incorrect error handling

### 3. Performance
- N+1 query patterns
- Unnecessary allocations
- Blocking I/O in hot paths
- Missing connection pooling

### 4. Style & Documentation
- Naming conventions
- Missing error wrapping
- Undocumented exported symbols
- Function length (>80 lines flag)

### 5. Test Coverage
- Edge cases missed
- Mocking complexity
- Test flakiness potential

## Output Format

```
## Security
- [SEVERITY] Issue description → Fix

## Logic
- [SEVERITY] Issue description → Fix

## Performance
- [SEVERITY] Issue description → Fix

## Style
- Issue description → Fix

## Tests
- Recommendation

## Summary
✅ N passed | ⚠️ N warnings | 🔴 N critical
```