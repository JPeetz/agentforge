// Package skill — marketplace integration for SkillsMP and GitHub.
package skill

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ── Marketplace Client ──────────────────────────────────────────────────────

// MarketplaceClient searches and installs skills from SkillsMP.
type MarketplaceClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewMarketplaceClient creates a SkillsMP API client.
func NewMarketplaceClient(apiKey string) *MarketplaceClient {
	return &MarketplaceClient{
		BaseURL:    "https://skillsmp.com/api/v1",
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// SearchResult is a search result from SkillsMP.
type SearchResult struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	Stars       int      `json:"stars"`
	Tags        []string `json:"tags"`
	GitHubURL   string   `json:"github_url"`
	InstallCmd  string   `json:"install_cmd"`
}

// Search queries SkillsMP for skills matching a keyword.
func (c *MarketplaceClient) Search(keyword string) ([]SearchResult, error) {
	url := fmt.Sprintf("%s/skills/search?q=%s", c.BaseURL, keyword)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("skillsmp search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("skillsmp search: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var results []SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("skillsmp decode: %w", err)
	}
	return results, nil
}

// AISearch queries SkillsMP semantically.
func (c *MarketplaceClient) AISearch(query string) ([]SearchResult, error) {
	url := fmt.Sprintf("%s/skills/ai-search", c.BaseURL)

	body := fmt.Sprintf(`{"query":"%s"}`, query)
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("skillsmp ai-search: %w", err)
	}
	defer resp.Body.Close()

	var results []SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("skillsmp ai-search decode: %w", err)
	}
	return results, nil
}

// ── GitHub Skill Discovery ─────────────────────────────────────────────────

// GitHubSkillSource points to a repo with skills.
type GitHubSkillSource struct {
	Owner string
	Repo  string
	Ref   string
}

// DiscoverGitHubSkills scans a GitHub repo for SKILL.md files.
func (c *MarketplaceClient) DiscoverGitHubSkills(source GitHubSkillSource) ([]*Skill, error) {
	if source.Ref == "" {
		source.Ref = "main"
	}

	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1",
		source.Owner, source.Repo, source.Ref,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: %w", err)
	}
	defer resp.Body.Close()

	var tree struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		return nil, fmt.Errorf("github: decode: %w", err)
	}

	var skills []*Skill
	for _, entry := range tree.Tree {
		if strings.HasSuffix(entry.Path, "SKILL.md") {
			rawURL := fmt.Sprintf(
				"https://raw.githubusercontent.com/%s/%s/%s/%s",
				source.Owner, source.Repo, source.Ref, entry.Path,
			)
			skill, err := c.fetchSkillFromURL(rawURL)
			if err != nil {
				continue
			}
			skill.Source = "git"
			skill.SourceURL = rawURL
			skills = append(skills, skill)
		}
	}
	return skills, nil
}

func (c *MarketplaceClient) fetchSkillFromURL(rawURL string) (*Skill, error) {
	resp, err := c.HTTPClient.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	manifest, body, err := ParseSKILL(data)
	if err != nil {
		return nil, err
	}

	return &Skill{
		Manifest:  manifest,
		Body:      body,
		SourceURL: rawURL,
	}, nil
}

// ── Skill Bundle ───────────────────────────────────────────────────────────

// Bundle is a collection of related skills.
type Bundle struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Version      string   `json:"version"`
	Skills       []*Skill `json:"skills"`
	SharedAssets []string `json:"shared_assets"`
}

// LoadBundle loads all SKILL.md files from a bundle directory.
func LoadBundle(dir string) (*Bundle, error) {
	bundle := &Bundle{Name: filepath.Base(dir)}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Name() == "SKILL.md" {
			skill, err := Load(path)
			if err != nil {
				return fmt.Errorf("bundle: %s: %w", path, err)
			}
			bundle.Skills = append(bundle.Skills, skill)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return bundle, nil
}

// ── Auto-Activation ────────────────────────────────────────────────────────

// MatchScore rates how well a skill matches a user request.
type MatchScore struct {
	Skill  *Skill
	Score  float64
	Reason string
}

// AutoActivate scans all installed skills and returns sorted matches.
func (r *Repository) AutoActivate(userRequest string) []MatchScore {
	r.mu.RLock()
	defer r.mu.RUnlock()

	lower := strings.ToLower(userRequest)
	var matches []MatchScore

	for _, s := range r.skills {
		if s.Manifest.DisableModelInvocation {
			continue
		}

		score := 0.0
		reasons := []string{}

		desc := strings.ToLower(s.Manifest.Description)
		words := strings.Fields(lower)
		for _, w := range words {
			if len(w) > 3 && strings.Contains(desc, w) {
				score += 0.3
				reasons = append(reasons, "desc:"+w)
			}
		}

		when := strings.ToLower(s.Manifest.WhenToUse)
		for _, w := range words {
			if len(w) > 3 && strings.Contains(when, w) {
				score += 0.5
				reasons = append(reasons, "when:"+w)
			}
		}

		if strings.Contains(desc, lower) {
			score += 1.0
			reasons = append(reasons, "full match")
		}

		if score > 0.3 {
			matches = append(matches, MatchScore{
				Skill:  s,
				Score:  score,
				Reason: strings.Join(reasons, "; "),
			})
		}
	}

	// Sort descending
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].Score > matches[i].Score {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	return matches
}