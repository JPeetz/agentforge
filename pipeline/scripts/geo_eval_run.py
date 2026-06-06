#!/usr/bin/env python3
"""Run citation-likelihood evaluation via OpenRouter."""
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

# Build the evaluation prompt
system_prompt = """You are a GEO (Generative Engine Optimization) evaluator. Your job is to assess how likely a piece of content is to be cited by AI systems such as ChatGPT, Perplexity, Google AI Overviews, and Claude when answering user queries.

You evaluate content across six dimensions and return a structured JSON assessment. You must be critical and specific. Vague praise is useless — concrete, actionable findings only.

Return ONLY valid JSON matching the schema provided. No preamble, no markdown fences, no explanation outside the JSON object."""

user_prompt = f"""## Target Query
"self-hosted AI agents replace SaaS"

## Content to Evaluate

{content[:8000]}

## Pre-computed Algorithmic Signals
- Entity density score: 73/100 (good)
- Answer structure score: 48/100 (moderate)
- Quotability score: 75/100 (strong)
- E-E-A-T composite: 40/100 (moderate)

## Evaluation Task
Score this content on each dimension from 0-100. Then produce a composite citation likelihood score.

Return this exact JSON structure:
{{
  "citationLikelihood": <number 0-100>,
  "dimensions": {{
    "factualDensity": <number 0-100>,
    "answerDirectness": <number 0-100>,
    "authoritySignals": <number 0-100>,
    "uniqueInsight": <number 0-100>,
    "structuralClarity": <number 0-100>,
    "queryAlignment": <number 0-100>
  }},
  "topStrengths": [<string>, <string>, <string>],
  "topIssues": [<string>, <string>, <string>],
  "rewriteInstructions": [<string>],
  "verdict": "cite-ready" | "needs-work" | "not-citable"
}}

## Dimension Definitions
- factualDensity: Density of verifiable facts, statistics, named entities, and specific data points
- answerDirectness: How directly and completely the content answers the target query without burying the answer
- authoritySignals: E-E-A-T markers — experience, expertise, citations, credibility
- uniqueInsight: Original analysis, novel framing, or non-obvious information not found on every generic page
- structuralClarity: Use of headers, lists, definitions, step patterns that AI engines can extract and re-format
- queryAlignment: Semantic match between the content's core claims and how a user would phrase the target query

## Verdict Thresholds
- cite-ready: citationLikelihood >= 75
- needs-work: citationLikelihood 40-74
- not-citable: citationLikelihood < 40"""

import urllib.request
import urllib.error

data = json.dumps({
    "model": "openrouter/owl-alpha",
    "messages": [
        {"role": "system", "content": system_prompt},
        {"role": "user", "content": user_prompt}
    ],
    "max_tokens": 2000,
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
        # Try to parse JSON from the response
        try:
            eval_result = json.loads(content_text)
            print(json.dumps(eval_result, indent=2))
        except json.JSONDecodeError:
            # Try to extract JSON from markdown fences
            match = re.search(r'```(?:json)?\s*(\{.*?\})\s*```', content_text, re.DOTALL)
            if match:
                eval_result = json.loads(match.group(1))
                print(json.dumps(eval_result, indent=2))
            else:
                print(content_text)
except urllib.error.HTTPError as e:
    print(f'HTTP Error: {e.code} {e.reason}')
    print(e.read().decode())
except Exception as e:
    print(f'Error: {e}')
