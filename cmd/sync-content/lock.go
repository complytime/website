// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// ContentLock tracks approved branch commit SHAs per repository.
// The lockfile is committed to version control and governs which upstream
// content versions the deploy workflow is allowed to sync.
type ContentLock struct {
	Repos map[string]string `json:"repos"`
}

// readLock loads a content lockfile from disk. If the file does not exist
// (bootstrap case), an empty lock is returned with no error.
func readLock(path string) (*ContentLock, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &ContentLock{Repos: make(map[string]string)}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading lock %s: %w", path, err)
	}

	var lock ContentLock
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("parsing lock %s: %w", path, err)
	}
	if lock.Repos == nil {
		lock.Repos = make(map[string]string)
	}
	return &lock, nil
}

// writeLock persists the lockfile to disk with deterministic key ordering.
func writeLock(path string, lock *ContentLock) error {
	ordered := make([]string, 0, len(lock.Repos))
	for k := range lock.Repos {
		ordered = append(ordered, k)
	}
	sort.Strings(ordered)

	m := make(map[string]string, len(ordered))
	for _, k := range ordered {
		m[k] = lock.Repos[k]
	}

	wrapper := struct {
		Repos map[string]string `json:"repos"`
	}{Repos: m}

	data, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling lock: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// sha returns the approved branch SHA for a repo, or "" if not locked.
func (l *ContentLock) sha(repo string) string {
	if l == nil || l.Repos == nil {
		return ""
	}
	return l.Repos[repo]
}
