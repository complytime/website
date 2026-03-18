// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// PeribolosConfig is the top-level structure parsed from peribolos.yaml
// in the org's .github repo.
type PeribolosConfig struct {
	Orgs map[string]PeribolosOrg `yaml:"orgs"`
}

// PeribolosOrg represents an organization entry in peribolos.yaml.
type PeribolosOrg struct {
	Repos map[string]PeribolosRepo `yaml:"repos"`
}

// PeribolosRepo holds per-repo metadata from peribolos.yaml.
type PeribolosRepo struct {
	Description   string `yaml:"description"`
	DefaultBranch string `yaml:"default_branch"`
}

// SyncConfig is the top-level structure parsed from sync-config.yaml.
type SyncConfig struct {
	Defaults  Defaults  `yaml:"defaults"`
	Sources   []Source  `yaml:"sources"`
	Discovery Discovery `yaml:"discovery"`
}

// Discovery configures automatic detection of new repos and doc files
// that are not yet declared in sources.
type Discovery struct {
	IgnoreRepos []string `yaml:"ignore_repos"`
	IgnoreFiles []string `yaml:"ignore_files"`
	ScanPaths   []string `yaml:"scan_paths"`
}

// Defaults holds fallback values applied to every source unless overridden.
type Defaults struct {
	Branch string `yaml:"branch"`
}

// Source is a single upstream repository declared in the config file.
type Source struct {
	Repo        string     `yaml:"repo"`
	Branch      string     `yaml:"branch"`
	SkipOrgSync bool       `yaml:"skip_org_sync"`
	Files       []FileSpec `yaml:"files"`
}

// FileSpec describes one file to fetch from a source repo and where to place it.
type FileSpec struct {
	Src       string    `yaml:"src"`
	Dest      string    `yaml:"dest"`
	Transform Transform `yaml:"transform"`
}

// Transform describes optional mutations applied to fetched content.
type Transform struct {
	InjectFrontmatter map[string]any `yaml:"inject_frontmatter"`
	RewriteLinks      bool           `yaml:"rewrite_links"`
	StripBadges       bool           `yaml:"strip_badges"`
}

// loadConfig reads a sync-config.yaml file and returns the parsed configuration.
// It applies default values (e.g. branch) and validates that every source has
// the required fields.
func loadConfig(path string) (*SyncConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg SyncConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	if cfg.Defaults.Branch == "" {
		cfg.Defaults.Branch = "main"
	}

	for i := range cfg.Sources {
		src := &cfg.Sources[i]
		if src.Repo == "" {
			return nil, fmt.Errorf("config %s: source[%d] missing required field 'repo'", path, i)
		}
		if src.Branch == "" {
			src.Branch = cfg.Defaults.Branch
		}
		for j, f := range src.Files {
			if f.Src == "" {
				return nil, fmt.Errorf("config %s: source[%d] (%s) file[%d] missing 'src'", path, i, src.Repo, j)
			}
			if f.Dest == "" {
				return nil, fmt.Errorf("config %s: source[%d] (%s) file[%d] missing 'dest'", path, i, src.Repo, j)
			}
		}
	}

	return &cfg, nil
}
