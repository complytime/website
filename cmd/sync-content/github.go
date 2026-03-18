// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	githubAPI        = "https://api.github.com"
	maxRetries       = 3
	maxResponseBytes = 10 << 20 // 10 MB safety ceiling for API response bodies
	maxDirDepth      = 10
)

// GitHub API response types

type Repo struct {
	Name            string   `json:"name"`
	FullName        string   `json:"full_name"`
	Description     string   `json:"description"`
	Language        string   `json:"language"`
	StargazersCount int      `json:"stargazers_count"`
	HTMLURL         string   `json:"html_url"`
	DefaultBranch   string   `json:"default_branch"`
	PushedAt        string   `json:"pushed_at"`
	Archived        bool     `json:"archived"`
	Fork            bool     `json:"fork"`
	Topics          []string `json:"topics"`
}

type FileResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	SHA      string `json:"sha"`
}

type DirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
}

type BranchResponse struct {
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

// apiClient wraps net/http for authenticated GitHub REST API calls.
type apiClient struct {
	token string
	http  *http.Client
}

func (c *apiClient) do(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return c.http.Do(req)
}

// getJSON fetches a URL and decodes JSON, retrying on rate limit (403/429)
// with exponential backoff and respect for Retry-After / X-RateLimit-Reset.
func (c *apiClient) getJSON(ctx context.Context, url string, dst any) error {
	var lastErr error
	for attempt := range maxRetries + 1 {
		resp, err := c.do(ctx, url)
		if err != nil {
			return err
		}

		if resp.StatusCode == http.StatusOK {
			limited := io.LimitReader(resp.Body, maxResponseBytes)
			err = json.NewDecoder(limited).Decode(dst)
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			return err
		}

		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		lastErr = fmt.Errorf("GET %s: %d %s", url, resp.StatusCode, body)

		if !isRateLimited(resp) || attempt == maxRetries {
			return lastErr
		}

		wait := retryWait(resp, attempt)
		slog.Warn("rate limited, retrying", "url", url, "attempt", attempt+1, "wait", wait.Round(time.Second))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	return lastErr
}

func isRateLimited(resp *http.Response) bool {
	if resp.StatusCode == http.StatusTooManyRequests {
		return true
	}
	if resp.StatusCode == http.StatusForbidden {
		return resp.Header.Get("X-RateLimit-Remaining") == "0"
	}
	return false
}

func retryWait(resp *http.Response, attempt int) time.Duration {
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if seconds, err := strconv.Atoi(ra); err == nil {
			return time.Duration(seconds) * time.Second
		}
	}
	if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
		if ts, err := strconv.ParseInt(reset, 10, 64); err == nil {
			wait := time.Until(time.Unix(ts, 0)) + time.Second
			if wait > 0 && wait < 5*time.Minute {
				return wait
			}
			if wait >= 5*time.Minute {
				slog.Warn("rate limit reset too far in future, using backoff", "reset_in", wait.Round(time.Second))
			}
		}
	}
	return time.Duration(1<<uint(attempt)) * time.Second
}

// appendRef appends a ?ref= query parameter to a URL when ref is non-empty,
// allowing content to be fetched at a specific commit SHA.
func appendRef(apiURL, ref string) string {
	if ref == "" {
		return apiURL
	}
	sep := "?"
	if strings.Contains(apiURL, "?") {
		sep = "&"
	}
	return apiURL + sep + "ref=" + url.QueryEscape(ref)
}

// escapePathSegments escapes each segment of a slash-delimited path for use in URLs.
func escapePathSegments(path string) string {
	segs := strings.Split(path, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/")
}

// fetchPeribolosRepos fetches peribolos.yaml from the org's .github repo and
// returns the list of repo names declared under orgs.<org>.repos.
func (c *apiClient) fetchPeribolosRepos(ctx context.Context, org string) ([]string, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/.github/contents/peribolos.yaml",
		githubAPI, url.PathEscape(org))
	var f FileResponse
	if err := c.getJSON(ctx, apiURL, &f); err != nil {
		return nil, fmt.Errorf("fetching peribolos.yaml from %s/.github: %w", org, err)
	}
	content, err := decodeContent(f)
	if err != nil {
		return nil, fmt.Errorf("decoding peribolos.yaml: %w", err)
	}

	var pc PeribolosConfig
	if err := yaml.Unmarshal([]byte(content), &pc); err != nil {
		return nil, fmt.Errorf("parsing peribolos.yaml: %w", err)
	}

	orgData, ok := pc.Orgs[org]
	if !ok {
		return nil, fmt.Errorf("peribolos.yaml has no entry for org %q", org)
	}

	names := make([]string, 0, len(orgData.Repos))
	for name := range orgData.Repos {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// getRepoMetadata fetches full metadata for a single repo from the GitHub API.
func (c *apiClient) getRepoMetadata(ctx context.Context, owner, name string) (*Repo, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s",
		githubAPI, url.PathEscape(owner), url.PathEscape(name))
	var repo Repo
	if err := c.getJSON(ctx, apiURL, &repo); err != nil {
		return nil, err
	}
	return &repo, nil
}

func (c *apiClient) getREADME(ctx context.Context, owner, repo, ref string) (string, string, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/readme",
		githubAPI, url.PathEscape(owner), url.PathEscape(repo))
	apiURL = appendRef(apiURL, ref)
	var f FileResponse
	if err := c.getJSON(ctx, apiURL, &f); err != nil {
		return "", "", err
	}
	content, err := decodeContent(f)
	return content, f.SHA, err
}

func (c *apiClient) getFileContent(ctx context.Context, owner, repo, path, ref string) (string, string, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s",
		githubAPI, url.PathEscape(owner), url.PathEscape(repo), escapePathSegments(path))
	apiURL = appendRef(apiURL, ref)
	var f FileResponse
	if err := c.getJSON(ctx, apiURL, &f); err != nil {
		return "", "", err
	}
	content, err := decodeContent(f)
	return content, f.SHA, err
}

func (c *apiClient) listDir(ctx context.Context, owner, repo, path, ref string) ([]DirEntry, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s",
		githubAPI, url.PathEscape(owner), url.PathEscape(repo), escapePathSegments(path))
	apiURL = appendRef(apiURL, ref)
	var entries []DirEntry
	if err := c.getJSON(ctx, apiURL, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (c *apiClient) getBranchSHA(ctx context.Context, owner, repo, branch string) (string, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/branches/%s",
		githubAPI, url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(branch))
	var b BranchResponse
	if err := c.getJSON(ctx, apiURL, &b); err != nil {
		return "", err
	}
	return b.Commit.SHA, nil
}

// listDirMD recursively lists .md files under a directory, reusing listDir.
// Returns paths relative to the repo root (e.g. "docs/guide.md").
// Recursion is bounded to maxDirDepth levels to limit API calls on deeply
// nested repositories.
func (c *apiClient) listDirMD(ctx context.Context, owner, repo, dir, ref string) ([]string, error) {
	return c.listDirMDDepth(ctx, owner, repo, dir, ref, 0)
}

func (c *apiClient) listDirMDDepth(ctx context.Context, owner, repo, dir, ref string, depth int) ([]string, error) {
	if depth >= maxDirDepth {
		slog.Warn("max directory depth reached, skipping deeper levels", "repo", owner+"/"+repo, "dir", dir, "depth", depth)
		return nil, nil
	}
	entries, err := c.listDir(ctx, owner, repo, dir, ref)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		switch {
		case e.Type == "file" && strings.HasSuffix(e.Name, ".md"):
			if e.Path != "" {
				files = append(files, e.Path)
			} else {
				files = append(files, dir+"/"+e.Name)
			}
		case e.Type == "dir":
			subDir := dir + "/" + e.Name
			if e.Path != "" {
				subDir = e.Path
			}
			sub, err := c.listDirMDDepth(ctx, owner, repo, subDir, ref, depth+1)
			if err != nil {
				slog.Warn("could not list subdir", "repo", owner+"/"+repo, "dir", subDir, "error", err)
				continue
			}
			files = append(files, sub...)
		}
	}
	return files, nil
}

func decodeContent(f FileResponse) (string, error) {
	if f.Encoding != "base64" {
		return f.Content, nil
	}
	raw := strings.NewReplacer("\n", "", "\r", "").Replace(f.Content)
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	return string(decoded), nil
}
