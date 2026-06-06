#!/usr/bin/env python3
"""Run citation-likelihood evaluation via SEO-API + OpenRouter."""
import sys
import os
import json

# Load env from both files
for env_file in [os.path.expanduser('~/.env/autoranker.env'), os.path.expanduser('~/.hermes/.env')]:
    if os.path.exists(env_file):
        with open(env_file) as f:
            for line in f:
                line = line.strip()
                if line and not line.startswith('#') and '=' in line:
                    key, _, val = line.partition('=')
                    key = key.strip()
                    val = val.strip().strip('"').strip("'")
                    if key not in os.environ:
                        os.environ[key] = val

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from seo_api_client import SEOApiClient as SEOAPIClient

base_url = os.environ.get('SEO_API_BASE_URL', '')
if not base_url:
    print('ERROR: SEO_API_BASE_URL not set')
    sys.exit(1)

if len(sys.argv) < 2:
    print('Usage: citation-check.py <draft.md> [query]')
    sys.exit(1)

draft_path = sys.argv[1]
query = sys.argv[2] if len(sys.argv) > 2 else 'self-hosted AI agents security 2026'

with open(draft_path) as f:
    content = f.read()

# Extract body (after frontmatter)
parts = content.split('---', 2)
body = parts[2].strip() if len(parts) > 2 else content

client = SEOAPIClient(base_url)

# Get the evaluation prompt
prompt_data = client.evaluation_prompt(body, query)
if 'error' in prompt_data:
    print(f'Error getting evaluation prompt: {prompt_data["error"]}')
    sys.exit(1)

# Call OpenRouter to evaluate
import requests

openrouter_key = os.environ.get('OPENROUTER_API_KEY', '')
if not openrouter_key:
    print('ERROR: OPENROUTER_API_KEY not set')
    sys.exit(1)

system_prompt = prompt_data['systemPrompt']
user_prompt = prompt_data['userPrompt']

# Use a capable model for evaluation
or_response = requests.post(
    'https://openrouter.ai/api/v1/chat/completions',
    headers={
        'Authorization': f'Bearer {openrouter_key}',
        'Content-Type': 'application/json',
    },
    json={
        'model': 'anthropic/claude-sonnet-4.6',
        'messages': [
            {'role': 'system', 'content': system_prompt},
            {'role': 'user', 'content': user_prompt},
        ],
        'max_tokens': 2000,
        'temperature': 0.1,
    },
    timeout=120,
)

if or_response.status_code != 200:
    print(f'OpenRouter error: {or_response.status_code}')
    print(or_response.text[:500])
    sys.exit(1)

or_data = or_response.json()
response_text = or_data['choices'][0]['message']['content']

# Parse JSON from response
try:
    # Try to extract JSON from the response
    json_start = response_text.find('{')
    json_end = response_text.rfind('}') + 1
    if json_start >= 0 and json_end > json_start:
        result = json.loads(response_text[json_start:json_end])
    else:
        result = {'raw': response_text}
except json.JSONDecodeError:
    result = {'raw': response_text}

print(json.dumps(result, indent=2))
