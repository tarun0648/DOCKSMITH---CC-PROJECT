package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/docksmith/internal/cache"
	"github.com/docksmith/internal/image"
	"github.com/docksmith/internal/store"
)

// Builder executes a Docksmithfile and produces an image.
type Builder struct {
	contextDir string
	name       string
	tag        string
	noCache    bool
	store      *store.Store
	cacheIdx   *cache.Index
}

// New creates a Builder.
func New(contextDir, nameTag string, noCache bool) (*Builder, error) {
	s, err := store.New()
	if err != nil {
		return nil, err
	}

	idx, err := cache.Load(s.CacheDir())
	if err != nil {
		return nil, err
	}

	name, tag := splitNameTag(nameTag)
	return &Builder{
		contextDir: contextDir,
		name:       name,
		tag:        tag,
		noCache:    noCache,
		store:      s,
		cacheIdx:   idx,
	}, nil
}

func splitNameTag(nt string) (string, string) {
	parts := strings.SplitN(nt, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return nt, "latest"
}

// Build executes all instructions and writes the image manifest.
func (b *Builder) Build() error {
	docksmithFile := filepath.Join(b.contextDir, "Docksmithfile")
	instructions, err := ParseFile(docksmithFile)
	if err != nil {
		return err
	}

	// Validate first instruction is FROM
	if len(instructions) == 0 || instructions[0].Type != InstrFROM {
		return fmt.Errorf("Docksmithfile must start with FROM")
	}

	total := len(instructions)
	var baseImage *image.Manifest
	var layers []image.Layer
	var configEnv []string
	var configCmd []string
	var configWorkDir string
	envState := make(map[string]string)
	var prevDigest string // digest of last layer-producing step (or base manifest digest)
	cacheMissed := false  // once true, all subsequent steps are misses
	var buildCreated string

	startTotal := time.Now()

	for i, instr := range instructions {
		stepNum := i + 1
		switch instr.Type {

		case InstrFROM:
			fmt.Printf("Step %d/%d : FROM %s\n", stepNum, total, instr.Args)
			imgName, imgTag := splitNameTag(instr.Args)
			base, err := b.store.GetImage(imgName, imgTag)
			if err != nil {
				return fmt.Errorf("FROM: %w", err)
			}
			baseImage = base
			// Inherit layers from base
			layers = make([]image.Layer, len(base.Layers))
			copy(layers, base.Layers)
			// Inherit config
			configEnv = append([]string{}, base.Config.Env...)
			configCmd = append([]string{}, base.Config.Cmd...)
			configWorkDir = base.Config.WorkingDir
			// Rebuild envState from inherited env
			for _, kv := range configEnv {
				k, v, _ := parseKV(kv)
				envState[k] = v
			}
			// prevDigest starts as base manifest digest for cache key chaining
			prevDigest = base.Digest

		case InstrWORKDIR:
			fmt.Printf("Step %d/%d : WORKDIR %s\n", stepNum, total, instr.Args)
			configWorkDir = instr.Args

		case InstrENV:
			fmt.Printf("Step %d/%d : ENV %s\n", stepNum, total, instr.Args)
			k, v, err := ParseENV(instr.Args)
			if err != nil {
				return fmt.Errorf("line %d: %w", instr.LineNum, err)
			}
			envState[k] = v
			// Rebuild configEnv from envState (sorted)
			configEnv = envMapToSlice(envState)

		case InstrCMD:
			fmt.Printf("Step %d/%d : CMD %s\n", stepNum, total, instr.Args)
			cmd, err := ParseCMD(instr.Args)
			if err != nil {
				return fmt.Errorf("line %d: %w", instr.LineNum, err)
			}
			configCmd = cmd

		case InstrCOPY:
			srcArg, dest, err := ParseCOPY(instr.Args)
			if err != nil {
				return fmt.Errorf("line %d: %w", instr.LineNum, err)
			}

			// Expand globs
			srcFiles, err := ExpandGlob(b.contextDir, srcArg)
			if err != nil {
				return fmt.Errorf("line %d: COPY glob error: %w", instr.LineNum, err)
			}

			// Compute source file hashes for cache key
			srcHashes := make(map[string]string)
			for _, sf := range srcFiles {
				rel, _ := filepath.Rel(b.contextDir, sf)
				h, err := HashFile(sf)
				if err != nil {
					return fmt.Errorf("line %d: hash error: %w", instr.LineNum, err)
				}
				srcHashes[rel] = h
			}

			instrText := fmt.Sprintf("COPY %s", instr.Args)
			cacheKey := cache.ComputeKey(prevDigest, instrText, configWorkDir, envState, srcHashes)

			stepStart := time.Now()
			cacheHit, layerDigest := b.checkCache(cacheKey, &cacheMissed)

			if cacheHit {
				fmt.Printf("Step %d/%d : COPY %s [CACHE HIT]\n", stepNum, total, instr.Args)
				// Get existing layer info
				layerPath := b.store.LayerPath(layerDigest)
				fi, _ := os.Stat(layerPath)
				var sz int64
				if fi != nil {
					sz = fi.Size()
				}
				layers = append(layers, image.Layer{
					Digest:    layerDigest,
					Size:      sz,
					CreatedBy: instrText,
				})
				prevDigest = layerDigest
			} else {
				fmt.Printf("Step %d/%d : COPY %s", stepNum, total, instr.Args)

				// Assemble current filesystem from layers
				tmpFS, err := b.assembleFSTemp(baseImage, layers)
				if err != nil {
					fmt.Println()
					return fmt.Errorf("assemble fs: %w", err)
				}

				// Apply WORKDIR
				if configWorkDir != "" {
					wdPath := filepath.Join(tmpFS, configWorkDir)
					os.MkdirAll(wdPath, 0755)
				}

				// Copy files into tmpFS at dest
				destAbs := filepath.Join(tmpFS, dest)
				if err := copySrcFiles(srcFiles, b.contextDir, destAbs); err != nil {
					os.RemoveAll(tmpFS)
					fmt.Println()
					return fmt.Errorf("COPY error: %w", err)
				}

				// Create layer tar from just the copied files
				layerDigest, tarBytes, err := createCopyLayer(srcFiles, b.contextDir, dest)
				os.RemoveAll(tmpFS)
				elapsed := time.Since(stepStart)
				if err != nil {
					fmt.Println()
					return fmt.Errorf("create layer: %w", err)
				}

				fmt.Printf(" [CACHE MISS] %.2fs\n", elapsed.Seconds())

				// Save layer
				if err := b.saveLayer(layerDigest, tarBytes); err != nil {
					return fmt.Errorf("save layer: %w", err)
				}

				fi, _ := os.Stat(b.store.LayerPath(layerDigest))
				var sz int64
				if fi != nil {
					sz = fi.Size()
				}
				layers = append(layers, image.Layer{
					Digest:    layerDigest,
					Size:      sz,
					CreatedBy: instrText,
				})
				prevDigest = layerDigest

				if !b.noCache {
					b.cacheIdx.Set(cacheKey, layerDigest)
				}
			}

		case InstrRUN:
			instrText := fmt.Sprintf("RUN %s", instr.Args)
			cacheKey := cache.ComputeKey(prevDigest, instrText, configWorkDir, envState, nil)

			stepStart := time.Now()
			cacheHit, layerDigest := b.checkCache(cacheKey, &cacheMissed)

			if cacheHit {
				fmt.Printf("Step %d/%d : RUN %s [CACHE HIT]\n", stepNum, total, instr.Args)
				layerPath := b.store.LayerPath(layerDigest)
				fi, _ := os.Stat(layerPath)
				var sz int64
				if fi != nil {
					sz = fi.Size()
				}
				layers = append(layers, image.Layer{
					Digest:    layerDigest,
					Size:      sz,
					CreatedBy: instrText,
				})
				prevDigest = layerDigest
			} else {
				// Assemble full filesystem snapshot before
				tmpFS, err := b.assembleFSTemp(baseImage, layers)
				if err != nil {
					return fmt.Errorf("assemble fs: %w", err)
				}

				// Apply WORKDIR inside tmpFS
				if configWorkDir != "" {
					wdPath := filepath.Join(tmpFS, configWorkDir)
					os.MkdirAll(wdPath, 0755)
				}

				// Snapshot before
				beforeSnap, err := snapshotPaths(tmpFS)
				if err != nil {
					os.RemoveAll(tmpFS)
					return fmt.Errorf("snapshot before: %w", err)
				}

				// Run command in isolation
				envVars := append(envMapToSlice(envState), "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
				if err := RunIsolated(tmpFS, instr.Args, envVars, configWorkDir); err != nil {
					os.RemoveAll(tmpFS)
					return fmt.Errorf("RUN failed: %w", err)
				}

				elapsed := time.Since(stepStart)
				fmt.Printf("Step %d/%d : RUN %s [CACHE MISS] %.2fs\n", stepNum, total, instr.Args, elapsed.Seconds())

				// Snapshot after and compute delta
				afterSnap, err := snapshotPaths(tmpFS)
				if err != nil {
					os.RemoveAll(tmpFS)
					return fmt.Errorf("snapshot after: %w", err)
				}

				deltaFiles := computeDelta(tmpFS, beforeSnap, afterSnap)

				// Create delta layer
				layerDigest, tarBytes, err := CreateLayerFromPaths(deltaFiles)
				os.RemoveAll(tmpFS)
				if err != nil {
					return fmt.Errorf("create RUN layer: %w", err)
				}

				if err := b.saveLayer(layerDigest, tarBytes); err != nil {
					return fmt.Errorf("save layer: %w", err)
				}

				fi, _ := os.Stat(b.store.LayerPath(layerDigest))
				var sz int64
				if fi != nil {
					sz = fi.Size()
				}
				layers = append(layers, image.Layer{
					Digest:    layerDigest,
					Size:      sz,
					CreatedBy: instrText,
				})
				prevDigest = layerDigest

				if !b.noCache {
					b.cacheIdx.Set(cacheKey, layerDigest)
				}
			}
		}
	}

	// Save cache index
	if !b.noCache {
		if err := b.cacheIdx.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save cache index: %v\n", err)
		}
	}

	// Build manifest
	// Spec: "When all steps are cache hits, the manifest is rewritten with the
	// original created value so the manifest digest remains identical across rebuilds."
	if buildCreated == "" {
		// Check if image already exists and preserve its created timestamp
		if existing, err := b.store.GetImage(b.name, b.tag); err == nil && existing != nil {
			buildCreated = existing.Created
		} else {
			buildCreated = time.Now().UTC().Format(time.RFC3339)
		}
	}

	manifest := &image.Manifest{
		Name:    b.name,
		Tag:     b.tag,
		Created: buildCreated,
		Config: image.Config{
			Env:        configEnv,
			Cmd:        configCmd,
			WorkingDir: configWorkDir,
		},
		Layers: layers,
	}

	if err := b.store.SaveImage(manifest); err != nil {
		return fmt.Errorf("save manifest: %w", err)
	}

	totalElapsed := time.Since(startTotal)
	// Re-read to get digest
	saved, _ := b.store.GetImage(b.name, b.tag)
	shortDigest := ""
	if saved != nil && len(saved.Digest) > 13 {
		shortDigest = saved.Digest[7:15]
	}
	fmt.Printf("\nSuccessfully built %s %s:%s (%.2fs)\n", shortDigest, b.name, b.tag, totalElapsed.Seconds())
	return nil
}

// checkCache returns (hit, digest). Updates cacheMissed if needed.
func (b *Builder) checkCache(cacheKey string, cacheMissed *bool) (bool, string) {
	if b.noCache || *cacheMissed {
		*cacheMissed = true
		return false, ""
	}
	digest := b.cacheIdx.Get(cacheKey)
	if digest == "" {
		*cacheMissed = true
		return false, ""
	}
	// Verify layer file exists
	if !b.store.LayerExists(digest) {
		*cacheMissed = true
		return false, ""
	}
	return true, digest
}

// assembleFSTemp extracts all layer tars into a new temp directory.
func (b *Builder) assembleFSTemp(_ *image.Manifest, layers []image.Layer) (string, error) {
	tmpDir, err := os.MkdirTemp("", "docksmith-fs-*")
	if err != nil {
		return "", err
	}
	for _, layer := range layers {
		tarPath := b.store.LayerPath(layer.Digest)
		if err := ExtractLayer(tarPath, tmpDir); err != nil {
			os.RemoveAll(tmpDir)
			return "", fmt.Errorf("extract layer %s: %w", layer.Digest, err)
		}
	}
	return tmpDir, nil
}

// saveLayer writes tar bytes to the layer store.
func (b *Builder) saveLayer(digest string, tarBytes []byte) error {
	path := b.store.LayerPath(digest)
	if _, err := os.Stat(path); err == nil {
		return nil // already exists (content-addressed dedup)
	}
	return os.WriteFile(path, tarBytes, 0644)
}

// copySrcFiles copies source files into destDir inside the container FS.
func copySrcFiles(srcFiles []string, contextDir, destAbs string) error {
	if err := os.MkdirAll(destAbs, 0755); err != nil {
		return err
	}
	for _, src := range srcFiles {
		rel, _ := filepath.Rel(contextDir, src)
		dst := filepath.Join(destAbs, filepath.Base(rel))
		// Preserve directory structure for ** globs
		if err := copyFile(src, dst); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return os.MkdirAll(dst, fi.Mode())
	}
	os.MkdirAll(filepath.Dir(dst), 0755)
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, fi.Mode())
}

// createCopyLayer creates a tar delta for a COPY instruction.
func createCopyLayer(srcFiles []string, contextDir, dest string) (string, []byte, error) {
	var tuples []fileTuple
	// dest is an absolute path inside the container (e.g. /app)
	destClean := strings.TrimPrefix(dest, "/")

	for _, src := range srcFiles {
		rel, _ := filepath.Rel(contextDir, src)
		// Place file at dest/basename
		archivePath := filepath.Join(destClean, filepath.Base(rel))
		tuples = append(tuples, fileTuple{srcPath: src, archivePath: filepath.ToSlash(archivePath)})
	}

	return CreateLayerFromPaths(tuples)
}

// snapshotPaths records all file paths and their mod times under root.
func snapshotPaths(root string) (map[string]time.Time, error) {
	snap := make(map[string]time.Time)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		snap[rel] = info.ModTime()
		return nil
	})
	return snap, err
}

// computeDelta returns fileTuples for files that are new or modified after RUN.
func computeDelta(root string, before, after map[string]time.Time) []fileTuple {
	var result []fileTuple
	for rel, afterMod := range after {
		if rel == "." {
			continue
		}
		beforeMod, existed := before[rel]
		if !existed || afterMod.After(beforeMod) {
			absPath := filepath.Join(root, rel)
			fi, err := os.Lstat(absPath)
			if err != nil {
				continue
			}
			archivePath := filepath.ToSlash(rel)
			if fi.IsDir() {
				archivePath += "/"
			}
			result = append(result, fileTuple{srcPath: absPath, archivePath: archivePath})
		}
	}
	// Sort for reproducibility
	sort.Slice(result, func(i, j int) bool {
		return result[i].archivePath < result[j].archivePath
	})
	return result
}

func envMapToSlice(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(m))
	for _, k := range keys {
		out = append(out, k+"="+m[k])
	}
	return out
}

func parseKV(kv string) (string, string, error) {
	idx := strings.IndexByte(kv, '=')
	if idx < 0 {
		return kv, "", nil
	}
	return kv[:idx], kv[idx+1:], nil
}
