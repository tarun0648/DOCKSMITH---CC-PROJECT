package cmd

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docksmith/internal/image"
	"github.com/docksmith/internal/store"
)

// runImport imports a Docker-exported tarball (docker save output) into the local store.
// Usage: docksmith import <tarball> <name:tag>
func runImport(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("import requires <tarball> <name:tag>")
	}
	tarball := args[0]
	nameTag := args[1]
	name, tag := splitNameTag(nameTag)

	s, err := store.New()
	if err != nil {
		return err
	}

	f, err := os.Open(tarball)
	if err != nil {
		return fmt.Errorf("open tarball: %w", err)
	}
	defer f.Close()

	// Parse Docker save format (OCI image tar)
	// Structure:
	//   manifest.json  - [{Config, RepoTags, Layers}]
	//   <hash>.json    - image config
	//   <layerhash>/layer.tar - layer tarballs
	files := make(map[string][]byte)
	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tarball: %w", err)
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return fmt.Errorf("reading entry %s: %w", hdr.Name, err)
		}
		files[hdr.Name] = data
	}

	// Parse manifest.json
	manifestData, ok := files["manifest.json"]
	if !ok {
		return fmt.Errorf("no manifest.json in tarball - is this a docker save output?")
	}

	type dockerManifest struct {
		Config   string   `json:"Config"`
		RepoTags []string `json:"RepoTags"`
		Layers   []string `json:"Layers"`
	}
	var dockerManifests []dockerManifest
	if err := json.Unmarshal(manifestData, &dockerManifests); err != nil {
		return fmt.Errorf("parse manifest.json: %w", err)
	}
	if len(dockerManifests) == 0 {
		return fmt.Errorf("empty manifest.json")
	}
	dm := dockerManifests[0]

	// Process each layer
	var layers []image.Layer
	for _, layerPath := range dm.Layers {
		layerData, ok := files[layerPath]
		if !ok {
			// Docker save sometimes uses different path separators or prefixes.
			// Try a suffix match on the path components.
			normalised := filepath.ToSlash(layerPath)
			for k, v := range files {
				if filepath.ToSlash(k) == normalised || strings.HasSuffix(filepath.ToSlash(k), "/layer.tar") {
					layerData = v
					ok = true
					break
				}
			}
		}
		if !ok {
			return fmt.Errorf("layer %q not found in tarball (available keys: %v)", layerPath, fileKeys(files))
		}

		// Compute digest of raw tar bytes
		h := sha256.Sum256(layerData)
		digest := fmt.Sprintf("sha256:%x", h)

		// Save to layers dir
		layerFilePath := s.LayerPath(digest)
		if _, err := os.Stat(layerFilePath); os.IsNotExist(err) {
			if err := os.WriteFile(layerFilePath, layerData, 0644); err != nil {
				return fmt.Errorf("write layer: %w", err)
			}
		}

		layers = append(layers, image.Layer{
			Digest:    digest,
			Size:      int64(len(layerData)),
			CreatedBy: fmt.Sprintf("<imported from %s>", filepath.Base(tarball)),
		})
	}

	// Parse image config for Env/Cmd/WorkingDir
	var configEnv []string
	var configCmd []string
	var configWorkDir string

	if dm.Config != "" {
		configData, ok := files[dm.Config]
		if ok {
			type dockerConfig struct {
				Config struct {
					Env        []string `json:"Env"`
					Cmd        []string `json:"Cmd"`
					WorkingDir string   `json:"WorkingDir"`
				} `json:"config"`
			}
			var dc dockerConfig
			if err := json.Unmarshal(configData, &dc); err == nil {
				configEnv = dc.Config.Env
				configCmd = dc.Config.Cmd
				configWorkDir = dc.Config.WorkingDir
			}
		}
	}

	manifest := &image.Manifest{
		Name:    name,
		Tag:     tag,
		Created: time.Now().UTC().Format(time.RFC3339),
		Config: image.Config{
			Env:        configEnv,
			Cmd:        configCmd,
			WorkingDir: configWorkDir,
		},
		Layers: layers,
	}

	if err := s.SaveImage(manifest); err != nil {
		return fmt.Errorf("save image: %w", err)
	}

	fmt.Printf("Imported %s:%s (%d layers)\n", name, tag, len(layers))
	return nil
}

// fileKeys returns a sorted list of keys for error messages.
func fileKeys(files map[string][]byte) []string {
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	return keys
}
