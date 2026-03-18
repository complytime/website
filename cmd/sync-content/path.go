// SPDX-License-Identifier: Apache-2.0

package main

import (
	"path/filepath"
	"strings"
)

func isValidRepoName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	if strings.ContainsAny(name, "/\\") {
		return false
	}
	for _, seg := range strings.Split(name, string(filepath.Separator)) {
		if seg == ".." {
			return false
		}
	}
	return true
}

// isUnderDir reports whether target is under base after resolving symlinks and
// cleaning both paths. Both base and target are resolved through symlinks as
// far as the filesystem allows (the target may not fully exist yet). Prevents
// path traversal attacks from config dest fields, manifest entries, or
// API-sourced file paths that contain "../" sequences.
func isUnderDir(base, target string) bool {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(absBase); err == nil {
		absBase = resolved
	}
	absTarget, err := evalDeepest(target)
	if err != nil {
		return false
	}
	absBase = filepath.Clean(absBase) + string(filepath.Separator)
	absTarget = filepath.Clean(absTarget)
	return strings.HasPrefix(absTarget, absBase) || absTarget == strings.TrimSuffix(absBase, string(filepath.Separator))
}

// evalDeepest resolves a path through symlinks as deeply as the filesystem
// allows. If the full path doesn't exist, it walks up to the deepest existing
// ancestor, resolves that, and appends the remaining unresolved tail.
func evalDeepest(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}
	dir, remaining := filepath.Dir(abs), filepath.Base(abs)
	for dir != filepath.Dir(dir) {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(resolved, remaining), nil
		}
		remaining = filepath.Join(filepath.Base(dir), remaining)
		dir = filepath.Dir(dir)
	}
	return abs, nil
}

func languageOrDefault(lang string) string {
	if lang == "" {
		return "Unknown"
	}
	return lang
}
