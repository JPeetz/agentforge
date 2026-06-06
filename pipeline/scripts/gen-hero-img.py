#!/usr/bin/env python3
"""Generate hero image for self-hosted AI agent stack article."""
import os, sys, time, urllib.request

# Load FAL_KEY from env file
env_path = os.path.expanduser('~/.hermes/.env')
key = None
if os.path.exists(env_path):
    with open(env_path) as f:
        for line in f:
            line = line.strip()
            if line.startswith('FAL_KEY=') and not line.startswith('#'):
                key = line.split('=', 1)[1].strip().strip('"').strip("'")
                break

if not key:
    print('ERROR: FAL_KEY not found in ~/.hermes/.env')
    sys.exit(1)

os.environ['FAL_KEY'] = key

try:
    import fal_client
except ImportError:
    print('ERROR: fal-client not installed. Run: pip3 install fal-client')
    sys.exit(1)

output_path = os.path.expanduser(
    '~/workspace/agentforge/pipeline/images/2026-05-10-self-hosted-ai-agent-stack-hero.png'
)
os.makedirs(os.path.dirname(output_path), exist_ok=True)

prompt = (
    "Clean technical illustration of a self-hosted AI agent stack architecture. "
    "Dark background with subtle blue and purple gradient. "
    "Central hub-and-spoke diagram showing interconnected nodes: Ollama, OpenClaw, n8n, CrewAI, Dify, Langflow. "
    "Glowing connection lines between nodes. "
    "Minimalist flat design, no text, no people, no corporate imagery. "
    "Professional tech diagram style."
)

print(f'Generating hero image...')
print(f'Prompt: {prompt[:80]}...')

result = fal_client.subscribe(
    "fal-ai/flux/dev",
    arguments={
        "prompt": prompt,
        "image_size": "landscape_16_9",
        "num_inference_steps": 28,
        "guidance_scale": 3.5,
    },
    with_logs=True,
)

if result and "images" in result and len(result["images"]) > 0:
    image_url = result["images"][0]["url"]
    urllib.request.urlretrieve(image_url, output_path)
    print(f'Image saved to: {output_path}')
    print(f'Image URL: {image_url}')
else:
    print(f'ERROR: No image in result')
    print(f'Result: {result}')
    sys.exit(1)
