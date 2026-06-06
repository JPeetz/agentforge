#!/usr/bin/env python3
"""SEO analysis script for AutoRanker pipeline."""
import json
import sys
import re
import math
from collections import Counter

def strip_frontmatter(content):
    if content.startswith('---'):
        parts = content.split('---', 2)
        if len(parts) > 2:
            return parts[2]
    return content

def strip_markdown(content):
    # Remove code blocks
    content = re.sub(r'```[\s\S]*?```', '', content)
    # Remove inline code
    content = re.sub(r'`[^`]+`', '', content)
    # Remove images
    content = re.sub(r'!\[[^\]]*\]\([^)]+\)', '', content)
    # Remove links but keep text
    content = re.sub(r'\[([^\]]+)\)]', r'\1', content)
    # Remove headings markers
    content = re.sub(r'^#{1,6}\s+', '', content, flags=re.MULTILINE)
    # Remove bold/italic
    content = re.sub(r'[*_]{1,2}', '', content)
    # Remove blockquotes
    content = re.sub(r'^>\s+', '', content, flags=re.MULTILINE)
    # Remove horizontal rules
    content = re.sub(r'^-{3,}$', '', content, flags=re.MULTILINE)
    return content

def keyword_density(content, keywords):
    words = re.findall(r'\b[a-zA-Z]+\b', content.lower())
    total = len(words)
    if total == 0:
        return []
    results = []
    for kw in keywords:
        kw_lower = kw.lower()
        count = content.lower().count(kw_lower)
        density = (count / total) * 100
        results.append({
            'keyword': kw,
            'count': count,
            'density': round(density, 2)
        })
    return results

def readability_score(content):
    sentences = re.split(r'[.!?]+', content)
    sentences = [s.strip() for s in sentences if s.strip()]
    words = re.findall(r'\b[a-zA-Z]+\b', content)
    syllables = sum(max(1, len(re.findall(r'[aeiouy]+', w.lower()))) for w in words)
    
    num_sentences = max(len(sentences), 1)
    num_words = max(len(words), 1)
    
    avg_sentence_length = num_words / num_sentences
    avg_syllables_per_word = syllables / num_words
    
    # Flesch Reading Ease
    flesch = 206.835 - (1.015 * avg_sentence_length) - (84.6 * avg_syllables_per_word)
    flesch = max(0, min(100, flesch))
    
    # Flesch-Kincaid Grade Level
    fk_grade = (0.39 * avg_sentence_length) + (11.8 * avg_syllables_per_word) - 15.59
    fk_grade = max(0, fk_grade)
    
    if flesch >= 80:
        level = "Easy (6th grade)"
    elif flesch >= 60:
        level = "Standard (8th-9th grade)"
    elif flesch >= 40:
        level = "Moderately difficult (college)"
    else:
        level = "Difficult (college graduate)"
    
    return {
        'flesch_reading_ease': round(flesch, 1),
        'flesch_kincaid_grade': round(fk_grade, 1),
        'avg_sentence_length': round(avg_sentence_length, 1),
        'avg_syllables_per_word': round(avg_syllables_per_word, 2),
        'total_words': num_words,
        'total_sentences': num_sentences,
        'readability_level': level
    }

def meta_check(content, title, description, keywords):
    issues = []
    score = 100
    
    # Title check
    if len(title) < 30:
        issues.append("Title too short (min 30 chars)")
        score -= 10
    elif len(title) > 60:
        issues.append("Title too long (max 60 chars, may be truncated in SERPs)")
        score -= 5
    
    # Description check
    if len(description) < 120:
        issues.append("Meta description too short (min 120 chars)")
        score -= 10
    elif len(description) > 160:
        issues.append("Meta description too long (max 160 chars)")
        score -= 5
    
    # Keyword in title
    content_lower = content.lower()
    for kw in keywords[:3]:
        if kw.lower() not in title.lower():
            issues.append(f"Primary keyword '{kw}' not in title")
            score -= 5
        if kw.lower() not in description.lower():
            issues.append(f"Primary keyword '{kw}' not in meta description")
            score -= 3
    
    # Check for H1
    h1_match = re.search(r'^#\s+(.+)$', content, re.MULTILINE)
    if not h1_match:
        issues.append("No H1 heading found")
        score -= 10
    
    # Check for H2 headings
    h2_count = len(re.findall(r'^##\s+', content, re.MULTILINE))
    if h2_count < 2:
        issues.append("Fewer than 2 H2 headings (add more structure)")
        score -= 5
    
    # Word count
    words = re.findall(r'\b[a-zA-Z]+\b', content)
    if len(words) < 300:
        issues.append("Content too short for SEO (min 300 words)")
        score -= 15
    elif len(words) < 1000:
        issues.append("Content could be longer for better SEO ranking")
        score -= 5
    
    return {
        'score': max(0, score),
        'issues': issues,
        'title_length': len(title),
        'description_length': len(description),
        'h1_found': bool(h1_match),
        'h2_count': h2_count,
        'word_count': len(words)
    }

if __name__ == '__main__':
    with open(sys.argv[1], 'r') as f:
        raw = f.read()
    
    # Parse frontmatter
    title = ""
    description = ""
    keywords = []
    if raw.startswith('---'):
        parts = raw.split('---', 2)
        fm = parts[1] if len(parts) > 1 else ""
        body = parts[2] if len(parts) > 2 else raw
        for line in fm.split('\n'):
            if line.startswith('title:'):
                title = line.split(':', 1)[1].strip().strip('"').strip("'")
            elif line.startswith('description:'):
                description = line.split(':', 1)[1].strip().strip('"').strip("'")
            elif line.startswith('keywords:'):
                kw_str = line.split(':', 1)[1].strip()
                keywords = [k.strip().strip('[]"\'') for k in kw_str.split(',')]
    else:
        body = raw
    
    clean = strip_frontmatter(body)
    clean = strip_markdown(clean)
    
    # Run checks
    kd = keyword_density(clean, keywords if keywords else ['self-hosted', 'AI agents', 'open source', 'SaaS', 'automation'])
    rd = readability_score(clean)
    mc = meta_check(body, title, description, keywords if keywords else ['self-hosted AI agents', 'open source AI', 'SaaS replacement'])
    
    report = {
        'keyword_density': kd,
        'readability': rd,
        'meta_check': mc
    }
    
    print(json.dumps(report, indent=2))
