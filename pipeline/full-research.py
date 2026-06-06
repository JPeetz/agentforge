#!/usr/bin/env python3
"""Full research sweep for all 14 empty PD tracker categories using TinyFish."""
import requests, json, time, os, re, sys
from datetime import datetime

API_KEY = 'sk-tinyfish-5tF33Boni6Gu0P0iGXwogO5fhEZ_JzIQ'
SEARCH_BASE = 'https://api.search.tinyfish.ai'
FETCH_BASE = 'https://api.fetch.tinyfish.ai'
headers = {'X-API-Key': API_KEY, 'Content-Type': 'application/json'}

def search(query, count=10):
    try:
        resp = requests.get(SEARCH_BASE, params={'query': query, 'count': count}, headers=headers, timeout=60)
        if resp.status_code == 200:
            return resp.json().get('results', [])
    except Exception as e:
        print(f"  SEARCH ERROR: {e}")
    return []

def fetch_url(url):
    try:
        resp = requests.post(FETCH_BASE, json={'url': url, 'type': 'markdown'}, headers=headers, timeout=120)
        if resp.status_code == 200:
            data = resp.json()
            return data.get('content', data.get('markdown', data.get('data', '')))
    except Exception as e:
        print(f"  FETCH ERROR: {e}")
    return ''

def extract_prompts(text, source_url, source_title):
    """Extract prompt-like content from fetched text."""
    prompts = []
    if not text:
        return prompts
    
    # Find code blocks that look like prompts
    code_blocks = re.findall(r'```(?:markdown|text|txt|prompt)?\n(.*?)```', text, re.DOTALL)
    for block in code_blocks:
        block = block.strip()
        if len(block) > 50 and len(block) < 5000:
            # Check if it looks like a prompt (has instructional language)
            prompt_keywords = ['you are', 'your task', 'act as', 'role:', 'instructions',
                             'system:', 'prompt:', 'objective', 'goal:', 'respond',
                             'always', 'never', 'must', 'should', 'help', 'assist',
                             'generate', 'create', 'write', 'analyze', 'review']
            if any(kw in block.lower() for kw in prompt_keywords):
                prompts.append({
                    'text': block[:2000],
                    'source_url': source_url,
                    'source_title': source_title,
                    'type': 'code_block'
                })
    
    # Find markdown sections that look like prompts
    sections = re.split(r'\n#{1,3}\s+', text)
    for section in sections:
        lines = section.strip().split('\n')
        if len(lines) >= 3:
            content = '\n'.join(lines[1:])  # Skip heading
            if len(content) > 100 and len(content) < 3000:
                prompt_keywords = ['you are', 'your task', 'act as', 'instructions',
                                 'system prompt', 'role:', 'objective']
                if any(kw in content.lower() for kw in prompt_keywords):
                    prompts.append({
                        'text': content[:2000],
                        'source_url': source_url,
                        'source_title': source_title,
                        'type': 'section'
                    })
    
    return prompts

categories = {
    "coding": [
        "AI coding agent prompts site:github.com",
        "claude code agent prompts programming site:github.com",
        "agentic coding system prompts LLM site:github.com",
        "AI code generation agent prompts site:reddit.com"
    ],
    "research": [
        "AI research agent prompts site:github.com",
        "LLM research assistant agent prompts site:github.com",
        "deep research agent system prompts site:github.com",
        "AI research agent prompts site:reddit.com"
    ],
    "trading": [
        "AI trading agent prompts site:github.com",
        "LLM trading bot agent prompts site:github.com",
        "algorithmic trading AI agent prompts site:github.com",
        "AI trading prompts site:reddit.com"
    ],
    "image-generation": [
        "AI image generation agent prompts site:github.com",
        "stable diffusion agent prompts site:github.com",
        "image generation AI agent system prompts site:github.com",
        "AI image prompts agent site:reddit.com"
    ],
    "voice-tts": [
        "AI voice TTS agent prompts site:github.com",
        "text to speech AI agent prompts site:github.com",
        "voice assistant agent prompts site:github.com",
        "TTS voice AI agent site:reddit.com"
    ],
    "video-gen": [
        "AI video generation agent prompts site:github.com",
        "video generation AI agent system prompts site:github.com",
        "AI video agent prompts site:reddit.com"
    ],
    "customer-support": [
        "AI customer support agent prompts site:github.com",
        "chatbot customer service agent prompts site:github.com",
        "customer support AI agent system prompts site:github.com",
        "AI support agent prompts site:reddit.com"
    ],
    "email-calendar": [
        "AI email calendar agent prompts site:github.com",
        "LLM email assistant agent prompts site:github.com",
        "calendar management AI agent prompts site:github.com",
        "AI email agent prompts site:reddit.com"
    ],
    "project-mgmt": [
        "AI project management agent prompts site:github.com",
        "LLM project manager agent prompts site:github.com",
        "agentic project management system prompts site:github.com",
        "AI project manager prompts site:reddit.com"
    ],
    "claude-code": [
        "claude code agent prompts site:github.com",
        "claude code SOUL.md agent site:github.com",
        "claude code skills agent prompts site:github.com",
        "claude code prompts site:reddit.com"
    ],
    "codex-cli": [
        "openai codex agent prompts site:github.com",
        "codex CLI agent prompts site:github.com",
        "codex agentic coding prompts site:github.com"
    ],
    "youtube-automation": [
        "youtube automation AI agent site:github.com",
        "youtube content creation AI agent site:github.com",
        "youtube AI agent prompts site:reddit.com"
    ],
    "social-media-ai": [
        "social media AI agent prompts site:github.com",
        "twitter AI agent prompts site:github.com",
        "social media automation AI agent site:github.com",
        "AI social media prompts site:reddit.com"
    ]
}

all_results = {}
total_searches = 0
start_time = time.time()

for cat, queries in categories.items():
    print(f"\n{'='*60}")
    print(f"=== {cat} ===")
    cat_sources = []
    cat_prompts = []
    seen_urls = set()
    
    for q in queries:
        results = search(q, count=10)
        total_searches += 1
        print(f"  [{total_searches}] {q[:70]}... -> {len(results)} results")
        
        for r in results:
            url = r.get('url', '')
            if url in seen_urls:
                continue
            seen_urls.add(url)
            cat_sources.append({
                'title': r.get('title', ''),
                'url': url,
                'snippet': r.get('snippet', '')[:300]
            })
        
        time.sleep(13)  # Rate limit: 5 queries/min
    
    all_results[cat] = {
        'sources': cat_sources,
        'source_count': len(cat_sources),
        'prompts': cat_prompts
    }
    print(f"  Unique sources: {len(cat_sources)}")

elapsed = time.time() - start_time

# Save results
output_path = '/Users/joergpeetz/workspace/agentforge/pipeline/week2-research-raw.json'
with open(output_path, 'w') as f:
    json.dump({
        'timestamp': datetime.now().isoformat(),
        'elapsed_seconds': round(elapsed),
        'total_searches': total_searches,
        'categories': all_results
    }, f, indent=2)

total_sources = sum(v['source_count'] for v in all_results.values())
cats_with_data = sum(1 for v in all_results.values() if v['source_count'] > 0)

print(f"\n{'='*60}")
print(f"=== RESEARCH COMPLETE ===")
print(f"Time: {round(elapsed/60, 1)} minutes")
print(f"Searches: {total_searches}")
print(f"Categories with data: {cats_with_data}/14")
print(f"Total unique sources: {total_sources}")
print(f"Saved to: {output_path}")

# Print summary per category
print(f"\n--- Summary ---")
for cat, data in all_results.items():
    print(f"  {cat}: {data['source_count']} sources")
