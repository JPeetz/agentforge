#!/usr/bin/env python3
"""Quick GEO audit using the SEO-API at seo-api-nu.vercel.app"""
import json, sys, os
sys.path.insert(0, os.path.expanduser('~/.hermes/skills/devops/content-pipeline/scripts'))
from seo_api_client import SEOApiClient

client = SEOApiClient(base_url='https://seo-api-nu.vercel.app')

draft_path = sys.argv[1] if len(sys.argv) > 1 else os.path.expanduser(
    '~/workspace/agentforge/pipeline/drafts/2026-05-10-self-hosted-ai-agent-stack-replaces-saas.md'
)

with open(draft_path) as f:
    content = f.read()

# Strip frontmatter
if content.startswith('---'):
    parts = content.split('---', 2)
    body = parts[2] if len(parts) > 2 else content
else:
    body = content

print("=== GEO AUDIT ===\n")

print("--- Entity Density ---")
print(json.dumps(client.entity_density(body), indent=2))

print("\n--- Answer Structure ---")
print(json.dumps(client.answer_structure(body), indent=2))

print("\n--- Quotability ---")
print(json.dumps(client.quotability(body), indent=2))

print("\n--- E-E-A-T Signals ---")
print(json.dumps(client.eeat_signals(body), indent=2))

print("\n--- Evaluation Prompt (for OpenRouter) ---")
eval_result = client.evaluation_prompt(body, "self-hosted AI agents replace SaaS")
print(json.dumps(eval_result, indent=2))
