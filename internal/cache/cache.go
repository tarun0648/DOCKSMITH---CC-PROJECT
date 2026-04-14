package cache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Index maps cache keys to layer digests.
type Index struct {
	path    string
	entries map[string]string // cacheKey -> layerDigest
}

// Load reads or creates the cache index at the given directory.
func Load(dir string) (*Index, error) {
	path := filepath.Join(dir, "index.json")
	idx := &Index{path: path, entries: make(map[string]string)}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return idx, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, &idx.entries); err != nil {
		return nil, err
	}
	return idx, nil
}

// Save writes the index to disk.
func (idx *Index) Save() error {
	data, err := json.MarshalIndent(idx.entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(idx.path, data, 0644)
}

// Get returns the layer digest for a cache key, or "" if not found.
func (idx *Index) Get(key string) string {
	return idx.entries[key]
}

// Set stores a cache key -> layer digest mapping.
func (idx *Index) Set(key, digest string) {
	idx.entries[key] = digest
}

// ComputeKey computes a deterministic cache key for a layer-producing instruction.
//
// Parameters:
//   prevDigest   - digest of the previous layer (or base manifest digest)
//   instruction  - full instruction text
//   workdir      - current WORKDIR value (empty string if not set)
//   envState     - map of all ENV values accumulated so far
//   srcHashes    - map of src file path -> sha256 hex (for COPY only; nil/empty for RUN)
func ComputeKey(prevDigest, instruction, workdir string, envState map[string]string, srcHashes map[string]string) string {
	h := sha256.New()

	h.Write([]byte(prevDigest))
	h.Write([]byte("\x00"))
	h.Write([]byte(instruction))
	h.Write([]byte("\x00"))
	h.Write([]byte(workdir))
	h.Write([]byte("\x00"))

	// ENV state: sorted by key
	envKeys := make([]string, 0, len(envState))
	for k := range envState {
		envKeys = append(envKeys, k)
	}
	sort.Strings(envKeys)
	for _, k := range envKeys {
		h.Write([]byte(k))
		h.Write([]byte("="))
		h.Write([]byte(envState[k]))
		h.Write([]byte("\x00"))
	}
	h.Write([]byte("\x00"))

	// Source file hashes (COPY only): sorted by path
	srcPaths := make([]string, 0, len(srcHashes))
	for p := range srcHashes {
		srcPaths = append(srcPaths, p)
	}
	sort.Strings(srcPaths)
	for _, p := range srcPaths {
		h.Write([]byte(p))
		h.Write([]byte("="))
		h.Write([]byte(srcHashes[p]))
		h.Write([]byte("\x00"))
	}

	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}
