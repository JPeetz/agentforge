#!/usr/bin/env python3
"""Run GEO evaluation-prompt endpoint against the draft."""
import json, urllib.request, sys

SEO_API = 'https://seo-api-nu.vercel.app'

def call(endpoint, payload):
    data = json.dumps(payload).encode('utf-8')
    req = urllib.request.Request(
        f'{SEO_API}/api/{endpoint}',
        data=data,
        headers={'Content-Type': 'application/json'},
        method='POST'
    )
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return json.loads(resp.read().decode('utf-8'))
    except Exception as e:
        return {'error': str(e)}

with open(sys.argv[1]) as f:
    content = f.read()

parts = content.split('---', 2)
body = parts[2].strip() if len(parts) >= 3 else content
title = ''
for line in parts[1].split('\n') if len(parts) >= 3 else []:
    if line.strip().startswith('title:'):
        title = line.split(':', 1)[1].strip().strip('"').strip("'")
        break

result = call('geo/evaluation-prompt', {
    'content': body,
    'targetQuery': f'{title} self-hosted AI agents replace SaaS',
    'includeSignals': True,
})

print('=== evaluation-prompt ===')
if 'error' in result:
    print(f'ERROR: {result["error"]}')
elif 'data' in result:
    d = result['data']
    algo = d.get('algorithmicSignals', {})
    print('Algorithmic signals:')
    for k, v in algo.items():
        if isinstance(v, dict):
            score = v.get('score', v.get('compositeScore', 'N/A'))
            rating = v.get('rating', 'N/A')
            print(f'  {k}: {score} ({rating})')
    print(f'\nTokens estimate: {d.get("tokensEstimate", "N/A")}')
    print(f'Expected output schema: citationLikelihood (0-100), verdict (cite-ready|needs-work|not-citable)')
    print(f'\nSystem prompt preview: {d.get("systemPrompt", "")[:200]}...')
    print(f'\nUser prompt preview: {d.get("userPrompt", "")[:200]}...')
