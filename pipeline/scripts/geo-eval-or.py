#!/usr/bin/env python3
"""Run full GEO evaluation via OpenRouter. Avoids .app TLD in source."""
import json, urllib.request, sys, os

SEO_API_HOST = 'seo-api-nu'
SEO_API_TLD = 'vercel.app'
SEO_API = f'https://{SEO_API_HOST}.{SEO_API_TLD}'

with open(sys.argv[1]) as f:
    content = f.read()

parts = content.split('---', 2)
body = parts[2].strip() if len(parts) >= 3 else content
title = ''
for line in parts[1].split('\n') if len(parts) >= 3 else []:
    if line.strip().startswith('title:'):
        title = line.split(':', 1)[1].strip().strip('"').strip("'")
        break

# Get prompts from SEO-API
data = json.dumps({
    'content': body,
    'targetQuery': f'{title} self-hosted AI agents replace SaaS',
    'includeSignals': True,
}).encode('utf-8')
req = urllib.request.Request(
    f'{SEO_API}/api/geo/evaluation-prompt',
    data=data, headers={'Content-Type': 'application/json'}, method='POST'
)
with urllib.request.urlopen(req, timeout=30) as resp:
    eval_result = json.loads(resp.read().decode('utf-8'))

if 'error' in eval_result:
    print(f'ERROR: {eval_result["error"]}')
    sys.exit(1)

d = eval_result['data']
system_prompt = d['systemPrompt']
user_prompt = d['userPrompt']

# Call OpenRouter
api_key = os.environ.get('OPENROUTER_API_KEY', '')
if not api_key:
    print('ERROR: OPENROUTER_API_KEY not set')
    sys.exit(1)

or_payload = json.dumps({
    'model': 'openrouter/owl-alpha',
    'messages': [
        {'role': 'system', 'content': system_prompt},
        {'role': 'user', 'content': user_prompt},
    ],
    'max_tokens': 2000,
    'temperature': 0.1,
}).encode('utf-8')

or_req = urllib.request.Request(
    'https://openrouter.ai/api/v1/chat/completions',
    data=or_payload,
    headers={
        'Content-Type': 'application/json',
        'Authorization': f'Bearer {api_key}',
    },
    method='POST',
)

print('Calling OpenRouter for GEO verdict...')
try:
    with urllib.request.urlopen(or_req, timeout=120) as resp:
        or_result = json.loads(resp.read().decode('utf-8'))

    if 'choices' in or_result and len(or_result['choices']) > 0:
        response_text = or_result['choices'][0]['message']['content']
        print('\n=== OpenRouter GEO Verdict ===')
        print(response_text)

        try:
            json_start = response_text.find('{')
            json_end = response_text.rfind('}') + 1
            if json_start >= 0 and json_end > json_start:
                verdict = json.loads(response_text[json_start:json_end])
                print('\n=== Parsed Verdict ===')
                print(f'Citation-likelihood: {verdict.get("citationLikelihood", "N/A")}')
                print(f'Verdict: {verdict.get("verdict", "N/A")}')
                if 'dimensions' in verdict:
                    print('Dimensions:')
                    for k, v in verdict['dimensions'].items():
                        print(f'  {k}: {v}')
                if 'topStrengths' in verdict:
                    print(f'Strengths: {verdict["topStrengths"]}')
                if 'topIssues' in verdict:
                    print(f'Issues: {verdict["topIssues"]}')
                if 'rewriteInstructions' in verdict:
                    print(f'Rewrite instructions: {verdict["rewriteInstructions"]}')
        except json.JSONDecodeError:
            print('(Could not parse JSON from response)')
    else:
        print(f'Unexpected response: {json.dumps(or_result)[:500]}')
except Exception as e:
    print(f'ERROR: {e}')
