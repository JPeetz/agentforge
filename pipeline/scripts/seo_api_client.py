#!/usr/bin/env python3
"""
SEO + GEO API client for AgentForge agents.
Wraps all 11 endpoints of https://seo-api-nu.vercel.app

Usage:
    from seo_api_client import SEOApiClient
    
    client = SEOApiClient()
    
    # SEO checks
    density = client.keyword_density(article_body, target_keyword)
    readability = client.readability(article_body)
    meta = client.meta_check(title, description, keyword)
    serp = client.serp_preview(url, title, description)
    
    # GEO checks
    entities = client.entity_density(article_body)
    structure = client.answer_structure(article_body)
    quotes = client.quotability(article_body)
    eeat = client.eeat_signals(article_body)
    
    # GEO evaluation prompt (returns prompts for your OpenRouter call)
    eval_prompt = client.evaluation_prompt(article_body, target_query)
    # Then feed eval_prompt['systemPrompt'] + eval_prompt['userPrompt'] to OpenRouter
"""

import requests
import json
import os

DEFAULT_BASE_URL = 'https://seo-api-nu.vercel.app'

class SEOApiClient:
    def __init__(self, base_url=None):
        self.base_url = base_url or os.environ.get('SEO_API_BASE_URL', DEFAULT_BASE_URL)
    
    def _post(self, endpoint, payload):
        url = f'{self.base_url}{endpoint}'
        try:
            r = requests.post(url, json=payload, timeout=15)
            data = r.json()
            if not data.get('success'):
                return {'error': data.get('error', 'Unknown error')}
            return data['data']
        except requests.exceptions.Timeout:
            return {'error': 'API timeout'}
        except requests.exceptions.ConnectionError:
            return {'error': 'API unreachable'}
        except Exception as e:
            return {'error': str(e)}
    
    # ── SEO ──────────────────────────────────────────────────
    
    def keyword_density(self, content, keyword=None):
        """Analyze keyword frequency and density.
        Returns: { keyword, count, totalWords, density, rating, topKeywords }
        Rating: 'low' (<0.5%) | 'good' (0.5-3%) | 'high' (>3%, stuffing)
        """
        payload = {'content': content}
        if keyword:
            payload['keyword'] = keyword
        return self._post('/api/seo/keyword-density', payload)
    
    def readability(self, content):
        """Flesch Reading Ease + Kincaid Grade Level.
        Returns: { fleschScore, fleschLabel, gradeLevel, gradeLevelLabel, ... }
        Pass threshold: fleschScore >= 50
        """
        return self._post('/api/seo/readability', {'content': content})
    
    def meta_check(self, title, description, keyword=None):
        """Validate title and meta description.
        Returns: { titleLength, titleStatus, descLength, descStatus, score, issues, suggestions }
        Pass threshold: score >= 80
        """
        payload = {'title': title, 'description': description}
        if keyword:
            payload['keyword'] = keyword
        return self._post('/api/seo/meta-check', payload)
    
    def serp_preview(self, url, title, description=None):
        """Generate Google SERP preview data.
        Returns: { domain, displayTitle, displayDescription, titleTruncated, descTruncated, serpPreview }
        """
        payload = {'url': url, 'title': title}
        if description:
            payload['description'] = description
        return self._post('/api/seo/serp-preview', payload)
    
    def roi_projection(self, search_volume, position, conversion_rate, revenue_per_conversion, monthly_investment, timeframe):
        """SEO ROI projection based on SERP position.
        Returns: { ctr, monthlyVisitors, monthlyConversions, monthlyRevenue, roi, rating }
        """
        return self._post('/api/seo/roi', {
            'searchVolume': search_volume,
            'position': position,
            'conversionRate': conversion_rate,
            'revenuePerConversion': revenue_per_conversion,
            'monthlyInvestment': monthly_investment,
            'timeframe': timeframe,
        })
    
    def page_speed(self, lcp, fid, cls, page_size=None, http_requests=None, ttfb=None):
        """Core Web Vitals scoring.
        Returns: { score, rating, lcpRating, fidRating, clsRating, issues, passedChecks }
        """
        payload = {'lcp': lcp, 'fid': fid, 'cls': cls}
        if page_size: payload['pageSize'] = page_size
        if http_requests: payload['httpRequests'] = http_requests
        if ttfb: payload['ttfb'] = ttfb
        return self._post('/api/seo/page-speed', payload)
    
    # ── GEO ──────────────────────────────────────────────────
    
    def entity_density(self, content):
        """Named entity, statistic, and fact marker density.
        Returns: { namedEntityCount, statisticCount, factMarkerCount, score, rating, topEntities }
        Rating: 'low' | 'moderate' | 'good' | 'strong'
        """
        return self._post('/api/geo/entity-density', {'content': content})
    
    def answer_structure(self, content):
        """Q&A, definition, step pattern detection.
        Returns: { questionCount, definitionCount, stepPatternCount, score, rating, strengths, gaps }
        Rating: 'poor' | 'moderate' | 'good' | 'excellent'
        """
        return self._post('/api/geo/answer-structure', {'content': content})
    
    def quotability(self, content):
        """Per-sentence citation-worthiness scoring.
        Returns: { compositeScore, rating, topSentences }
        Rating: 'low' | 'moderate' | 'good' | 'strong'
        Pass threshold: compositeScore >= 50
        """
        return self._post('/api/geo/quotability', {'content': content})
    
    def eeat_signals(self, content):
        """E-E-A-T marker detection.
        Returns: { experienceScore, expertiseScore, authorityScore, trustScore, compositeScore, rating, signals, gaps }
        Rating: 'weak' | 'moderate' | 'good' | 'strong'
        Pass threshold: compositeScore >= 50
        """
        return self._post('/api/geo/eeat-signals', {'content': content})
    
    def evaluation_prompt(self, content, target_query, include_signals=True):
        """
        Get pre-built GEO evaluation prompts + algorithmic signals.
        Returns: { systemPrompt, userPrompt, tokensEstimate, expectedOutputSchema, algorithmicSignals }
        
        Usage:
            data = client.evaluation_prompt(article_body, "how to self-host AI agents")
            # Feed data['systemPrompt'] + data['userPrompt'] to OpenRouter
            # Parse JSON response for verdict
        """
        return self._post('/api/geo/evaluation-prompt', {
            'content': content,
            'targetQuery': target_query,
            'includeSignals': include_signals,
        })
    
    # ── Combined audit ───────────────────────────────────────
    
    def full_seo_audit(self, content, title, description, keyword, url=None):
        """Run all SEO endpoints. Returns combined report."""
        result = {
            'keyword_density': self.keyword_density(content, keyword),
            'readability': self.readability(content),
            'meta_check': self.meta_check(title, description, keyword),
        }
        if url:
            result['serp_preview'] = self.serp_preview(url, title, description)
        
        # Calculate overall SEO score
        issues = []
        kd = result['keyword_density']
        rd = result['readability']
        mc = result['meta_check']
        
        if isinstance(kd, dict) and 'rating' in kd:
            if kd['rating'] == 'high':
                issues.append('Keyword stuffing detected')
            elif kd['rating'] == 'low':
                issues.append('Keyword density too low')
        
        if isinstance(rd, dict) and 'fleschScore' in rd:
            if rd['fleschScore'] < 50:
                issues.append(f"Flesch score {rd['fleschScore']} below threshold (50)")
        
        if isinstance(mc, dict):
            issues.extend(mc.get('issues', []))
        
        result['summary'] = {
            'overall_score': mc.get('score', 0) if isinstance(mc, dict) else 0,
            'issues': issues,
            'pass': len(issues) == 0 and (isinstance(mc, dict) and mc.get('score', 0) >= 80),
        }
        return result
    
    def full_geo_audit(self, content):
        """Run all GEO endpoints. Returns combined report."""
        result = {
            'entity_density': self.entity_density(content),
            'answer_structure': self.answer_structure(content),
            'quotability': self.quotability(content),
            'eeat_signals': self.eeat_signals(content),
        }
        
        # Calculate overall GEO score
        scores = []
        for key in ['entity_density', 'answer_structure', 'quotability', 'eeat_signals']:
            d = result[key]
            if isinstance(d, dict):
                if 'score' in d:
                    scores.append(d['score'])
                elif 'compositeScore' in d:
                    scores.append(d['compositeScore'])
        
        avg_score = sum(scores) / len(scores) if scores else 0
        
        gaps = []
        for key in ['answer_structure', 'eeat_signals']:
            d = result[key]
            if isinstance(d, dict) and 'gaps' in d:
                gaps.extend(d['gaps'])
        
        result['summary'] = {
            'average_score': round(avg_score, 1),
            'gaps': gaps,
            'pass': avg_score >= 50,
        }
        return result


# ── CLI interface ─────────────────────────────────────────────

if __name__ == '__main__':
    import sys
    
    if len(sys.argv) < 2:
        print("Usage: python seo_api_client.py <command> [args]")
        print("Commands: keyword-density, readability, meta-check, serp-preview,")
        print("          entity-density, answer-structure, quotability, eeat-signals,")
        print("          evaluation-prompt, full-seo-audit, full-geo-audit")
        sys.exit(1)
    
    client = SEOApiClient()
    command = sys.argv[1]
    
    if command == 'keyword-density':
        content = sys.argv[2] if len(sys.argv) > 2 else sys.stdin.read()
        keyword = sys.argv[3] if len(sys.argv) > 3 else None
        print(json.dumps(client.keyword_density(content, keyword), indent=2))
    
    elif command == 'readability':
        content = sys.argv[2] if len(sys.argv) > 2 else sys.stdin.read()
        print(json.dumps(client.readability(content), indent=2))
    
    elif command == 'meta-check':
        title = sys.argv[2]
        desc = sys.argv[3]
        kw = sys.argv[4] if len(sys.argv) > 4 else None
        print(json.dumps(client.meta_check(title, desc, kw), indent=2))
    
    elif command == 'full-seo-audit':
        # Args: content_file title description keyword [url]
        with open(sys.argv[2]) as f:
            content = f.read()
        title = sys.argv[3]
        desc = sys.argv[4]
        kw = sys.argv[5]
        url = sys.argv[6] if len(sys.argv) > 6 else None
        print(json.dumps(client.full_seo_audit(content, title, desc, kw, url), indent=2))
    
    elif command == 'full-geo-audit':
        content = sys.argv[2] if len(sys.argv) > 2 else sys.stdin.read()
        print(json.dumps(client.full_geo_audit(content), indent=2))
    
    else:
        print(f"Unknown command: {command}")
        sys.exit(1)
