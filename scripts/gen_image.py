import os
import fal_client

# Set the API key
os.environ["FAL_KEY"] = "7921a04c-01e5-4497-82b2-a930b0e9e203:a8241239071f13207ba683c353edf234"

prompt = (
    "A dark, futuristic server room with glowing blue and purple neural network connections "
    "running through rack-mounted hardware. A human silhouette stands at a terminal, surrounded "
    "by floating holographic AI agent icons — a robot assistant, a workflow diagram, a code editor, "
    "and a chat bubble. Cyberpunk aesthetic, dramatic lighting, 16:9 aspect ratio, no text, "
    "no words, no letters. Photorealistic style with subtle digital art flourishes."
)

print("Submitting to Fal.ai FLUX.1-dev...")

result = fal_client.subscribe(
    "fal-ai/flux/dev",
    arguments={
        "prompt": prompt,
        "image_size": "landscape_16_9",
        "num_inference_steps": 28,
        "guidance_scale": 3.5,
        "seed": 42,
    },
    with_logs=True,
)

print(f"Result: {result}")

if result and "images" in result and len(result["images"]) > 0:
    image_url = result["images"][0]["url"]
    print(f"Image URL: {image_url}")
    
    # Download the image
    import urllib.request
    output_path = os.path.expanduser(
        "~/workspace/agentforge/pipeline/images/2026-05-10-self-hosted-ai-agent-stack-hero.png"
    )
    urllib.request.urlretrieve(image_url, output_path)
    print(f"Saved to: {output_path}")
else:
    print("No image in result")
