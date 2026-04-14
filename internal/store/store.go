package store

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docksmith/internal/image"
)

// Store manages the ~/.docksmith directory.
type Store struct {
	root string
}

// New creates a Store rooted at ~/.docksmith, creating directories if needed.
func New() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not find home dir: %w", err)
	}
	root := filepath.Join(home, ".docksmith")
	for _, sub := range []string{"images", "layers", "cache"} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0755); err != nil {
			return nil, err
		}
	}
	return &Store{root: root}, nil
}

func (s *Store) Root() string     { return s.root }
func (s *Store) LayersDir() string { return filepath.Join(s.root, "layers") }
func (s *Store) CacheDir() string  { return filepath.Join(s.root, "cache") }
func (s *Store) ImagesDir() string { return filepath.Join(s.root, "images") }

// ListImages returns all stored image manifests.
func (s *Store) ListImages() ([]*image.Manifest, error) {
	entries, err := os.ReadDir(s.ImagesDir())
	if err != nil {
		return nil, err
	}
	var out []*image.Manifest
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.ImagesDir(), e.Name()))
		if err != nil {
			continue
		}
		var m image.Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		out = append(out, &m)
	}
	return out, nil
}

// GetImage retrieves a manifest by name and tag.
func (s *Store) GetImage(name, tag string) (*image.Manifest, error) {
	path := filepath.Join(s.ImagesDir(), imageFileName(name, tag))
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("image %s:%s not found", name, tag)
		}
		return nil, err
	}
	var m image.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// SaveImage writes a manifest to disk with the correct digest.
func (s *Store) SaveImage(m *image.Manifest) error {
	// Compute digest: serialize with digest="" then hash
	orig := m.Digest
	m.Digest = ""
	canonical, err := json.Marshal(m)
	if err != nil {
		return err
	}
	h := sha256.Sum256(canonical)
	m.Digest = fmt.Sprintf("sha256:%x", h)
	_ = orig

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.ImagesDir(), imageFileName(m.Name, m.Tag))
	return os.WriteFile(path, data, 0644)
}

// RemoveImage deletes the manifest file.
func (s *Store) RemoveImage(name, tag string) error {
	path := filepath.Join(s.ImagesDir(), imageFileName(name, tag))
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("image %s:%s not found", name, tag)
		}
		return err
	}
	return nil
}

// LayerPath returns the full path for a layer by its digest.
func (s *Store) LayerPath(digest string) string {
	// digest is "sha256:<hex>"
	hex := strings.TrimPrefix(digest, "sha256:")
	return filepath.Join(s.LayersDir(), hex+".tar")
}

// LayerExists checks whether a layer file is present on disk.
func (s *Store) LayerExists(digest string) bool {
	_, err := os.Stat(s.LayerPath(digest))
	return err == nil
}

func imageFileName(name, tag string) string {
	safe := strings.ReplaceAll(name, "/", "_")
	return safe + "_" + tag + ".json"
}
