#!/usr/bin/env python3
"""Quick GEO check against production SEO-API."""
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
        with urllib.request.urlopen(req, timeout=15) as resp:
            return json.loads(resp.read().decode('utf-8'))
    except Exception as e:
        return {'error': str(e)}

with open(sys.argv[1]) as f:
    content = f.read()

parts = content.split('---', 2)
body = parts[2].strip() if len(parts) >= 3 else content

for ep in ['entity-density', 'answer-structure', 'quotability', 'eeat-signals']:
    result = call(f'geo/{ep}', {'content': body})
    print(f'\n=== {ep} ===')
    if 'error' in result:
        print(f'ERROR: {result["error"]}')
    elif 'data' in result:
        d = result['data']
        score = d.get('score', d.get('compositeScore', 'N/A'))
        rating = d.get('rating', 'N/A')
        print(f'Score: {score} ({rating})')
        if 'gaps' in d and d['gaps']:
            print(f'Gaps: {d["gaps"]}')
        if 'signals' in d:
            for k, v in d['signals'].items():
                if v:
                    print(f'  {k}: {v[:3] if isinstance(v, list) else v}')
        if 'topStrengths' in d and d['topStrengths']:
            print(f'Strengths: {d["topStrengths"][:3]}')
