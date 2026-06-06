import re
with open('/Users/joergpeetz/workspace/agentforge/pipeline/drafts/2026-05-20-self-hosted-ai-agent-stack-from-scratch.md') as f:
    content = f.read()
# Remove frontmatter
content = re.sub(r'^---.*?---', '', content, flags=re.DOTALL)
# Remove code blocks
content = re.sub(r'```.*?```', '', content, flags=re.DOTALL)
# Count words
words = len(content.split())
print(f'Word count: {words}')
