#!/usr/bin/env python3
"""Run citation-likelihood evaluation via OpenRouter (full content)."""
import json
import os
import sys
import re

# Read env file directly
env_path = os.path.expanduser('~/.hermes/.env')
with open(env_path) as f:
    for line in f:
        line = line.strip()
        if line and not line.startswith('#') and '=' in line:
            k, v = line.split('=', 1)
            os.environ[k.strip()] = v.strip().strip('"').strip("'")

api_key = os.environ.get('OPENROUTER_API_KEY')
if not api_key:
    print('ERROR: OPENROUTER_API_KEY not set')
    sys.exit(1)

# Read the draft
draft_path = '/Users/joergpeetz/workspace/agentforge/pipeline/drafts/2026-05-20-self-hosted-ai-agent-stack-from-scratch.md'
with open(draft_path) as f:
    content = f.read()

# Remove frontmatter
content = re.sub(r'^---.*?---', '', content, flags=re.DOTALL).strip()
# Remove code blocks for token efficiency
content_no_code = re.sub(r'```.*?```', '[CODE BLOCK]', content, flags=re.DOTALL)

# Build the evaluation prompt
system_prompt = """You are a GEO (Generative Engine Optimization) evaluator. Assess how likely content is to be cited by AI systems. Be critical and specific. Return ONLY valid JSON."""

user_prompt = f"""## Target Query
"self-hosted AI agents replace SaaS"

## Content to Evaluate (full article, code blocks replaced with [CODE BLOCK])

{content_no_code[:12000]}

## Algorithmic Signals
- Entity density: 73/100 (good)
- Answer structure: 48/100 (moderate) 
- Quotability: 75/100 (strong)
- E-E-A-T: 40/100 (moderate)

## Score this content 0-100 on each dimension, then composite citation likelihood.

Return ONLY this JSON:
{{
  "citationLikelihood": <0-100>,
  "dimensions": {{
    "factualDensity": <0-100>,
    "answerDirectness": <0-100>,
    "authoritySignals": <0-100>,
    "uniqueInsight": <0-100>,
    "structuralClarity": <0-100>,
    "queryAlignment": <0-100>
  }},
  "topStrengths": ["s1", "s2", "s3"],
  "topIssues": ["i1", "i2", "i3"],
  "rewriteInstructions": ["r1"],
  "verdict": "cite-ready" | "needs-work" | "not-citable"
}}

Verdict thresholds: cite-ready >= 75, needs-work 40-74, not-citable < 40"""

import urllib.request

data = json.dumps({
    "model": "openrouter/owl-alpha",
    "messages": [
        {"role": "system", "content": system_prompt},
        {"role": "user", "content": user_prompt}
    ],
    "max_tokens": 1500,
    "temperature": 0.1
}).encode()

req = urllib.request.Request(
    'https://openrouter.ai/api/v1/chat/completions',
    data=data,
    headers={
        'Authorization': f'Bearer {api_key}',
        'Content-Type': 'application/json'
    }
)

try:
    with urllib.request.urlopen(req, timeout=120) as resp:
        result = json.loads(resp.read())
        content_text = result['choices'][0]['message']['content']
        try:
            eval_result = json.loads(content_text)
            print(json.dumps(eval_result, indent=2))
        except json.JSONDecodeError:
            match = re.search(r'```(?:json)?\s*(\{.*?\})\s*```', content_text, re.DOTALL)
            if match:
                print(match.group(1))
            else:
                print(content_text)
except Exception as e:
    print(f'Error: {e}')
