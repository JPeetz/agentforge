#!/usr/bin/env python3
"""Generate a hero image via Fal.ai FLUX.1-dev and save to the pipeline images directory.

Usage: python3 generate-image.py "<prompt>" <output_path>

The FAL_KEY env var must be set (source ~/.hermes/.env first, or it's already in the environment).

Example:
  source ~/.hermes/.env
  python3 generate-image.py "A dark server room with glowing blue LEDs, cyberpunk style" \\
    ~/workspace/agentforge/pipeline/images/2026-05-10-hero.png
"""
import os
import sys
import time
import urllib.request

try:
    import fal_client
except ImportError:
    print("ERROR: fal-client not installed. Run: pip3 install fal-client")
    sys.exit(1)


def generate_image(prompt: str, output_path: str) -> str:
    """Generate an image via Fal.ai and save it. Returns the output path on success."""
    fal_key = os.environ.get("FAL_KEY", "")
    if not fal_key:
        print("ERROR: FAL_KEY env var not set. Run: source ~/.hermes/.env")
        sys.exit(1)

    os.environ["FAL_KEY"] = fal_key

    print(f"[Fal.ai] Submitting prompt: {prompt[:80]}...")

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

    if not result or "images" not in result or len(result["images"]) == 0:
        print(f"ERROR: No image in result. Response: {result}")
        sys.exit(1)

    image_url = result["images"][0]["url"]
    print(f"[Fal.ai] Image URL: {image_url}")

    os.makedirs(os.path.dirname(os.path.abspath(output_path)), exist_ok=True)
    urllib.request.urlretrieve(image_url, output_path)

    size = os.path.getsize(output_path)
    print(f"[Fal.ai] Saved: {output_path} ({size} bytes)")
    return output_path


if __name__ == "__main__":
    if len(sys.argv) < 3:
        print(f"Usage: {sys.argv[0]} '<prompt>' <output_path>")
        sys.exit(1)

    prompt = sys.argv[1]
    output = sys.argv[2]

    # Wait a moment to avoid rate limiting if called in a loop
    time.sleep(3)

    generate_image(prompt, output)
