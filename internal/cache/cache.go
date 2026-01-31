package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"flowrun/internal/workflow"
)

// Entry represents a cached execution result.
type Entry struct {
	Hash      string    `json:"hash"`
	Timestamp time.Time `json:"timestamp"`
	ExitCode  int       `json:"exit_code"`
}

// Store manages the cache file.
type Store struct {
	Path    string
	Entries map[string]Entry // Key: "workflow:step"
	mu      sync.RWMutex
}

// New creates or loads a cache store.
// If path is empty, defaults to .flowrun_cache.json in CWD.
func New(path string) (*Store, error) {
	if path == "" {
		cwd, _ := os.Getwd()
		path = filepath.Join(cwd, ".flowrun_cache.json")
	}

	s := &Store{
		Path:    path,
		Entries: make(map[string]Entry),
	}

	if err := s.Load(); err != nil {
		// If load fails (e.g. bad json), just start empty but warn? 
		// For now return error only if it's not "not exists".
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return s, nil
}

// Load reads the cache from disk.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.Path)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &s.Entries)
}

// Save writes the cache to disk.
func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s.Entries, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.Path, data, 0644)
}

// Get retrieves a cache entry.
func (s *Store) Get(wfName, stepName string) (Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.Entries[key(wfName, stepName)]
	return e, ok
}

// Set adds or updates a cache entry.
func (s *Store) Set(wfName, stepName, hash string, exitCode int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Entries[key(wfName, stepName)] = Entry{
		Hash:      hash,
		Timestamp: time.Now(),
		ExitCode:  exitCode,
	}
}

func key(wf, step string) string {
	return fmt.Sprintf("%s:%s", wf, step)
}

// ComputeHash calculates a deterministic hash for a step.
// It includes: Workflow Name, Step Name, Command, Effective Env, and Dependency Hashes.
func ComputeHash(wfName string, step workflow.Step, effectiveEnv []string, depHashes map[string]string) string {
	h := sha256.New()

	// 1. Identity & Config
	h.Write([]byte("v1")) // Version prefix to invalidate old hashes if logic changes
	h.Write([]byte(wfName))
	h.Write([]byte(step.Name))
	h.Write([]byte(step.Run))
	h.Write([]byte(step.Timeout))
	h.Write([]byte(fmt.Sprintf("%d", step.Retry)))

	// 2. Environment
	// effectiveEnv should be the final list of "KEY=VAL" strings
	// We sort them to ensure deterministic order
	// Copy to avoid mutating original
	sortedEnv := make([]string, len(effectiveEnv))
	copy(sortedEnv, effectiveEnv)
	sort.Strings(sortedEnv)
	
	for _, e := range sortedEnv {
		h.Write([]byte(e))
		h.Write([]byte(";"))
	}

	// 3. Dependencies
	// Merkle-tree like: if dependency hash changes, this step's hash changes
	if len(step.Needs) > 0 {
		sortedDeps := make([]string, len(step.Needs))
		copy(sortedDeps, step.Needs)
		sort.Strings(sortedDeps)

		for _, dep := range sortedDeps {
			h.Write([]byte(dep))
			h.Write([]byte(":"))
			if val, ok := depHashes[dep]; ok {
				h.Write([]byte(val))
			} else {
				h.Write([]byte("missing")) // Should not happen in valid flow
			}
			h.Write([]byte(";"))
		}
	}

	return hex.EncodeToString(h.Sum(nil))
}
