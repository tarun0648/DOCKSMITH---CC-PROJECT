package runtime

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/docksmith/internal/builder"
	"github.com/docksmith/internal/image"
	"github.com/docksmith/internal/store"
)

// Runtime assembles image filesystems and runs containers.
type Runtime struct {
	store *store.Store
}

// New creates a Runtime.
func New(s *store.Store) (*Runtime, error) {
	return &Runtime{store: s}, nil
}

// Run assembles the image filesystem and starts the container.
func (r *Runtime) Run(img *image.Manifest, envOverrides []string, overrideCmd []string) error {
	// Assemble filesystem
	tmpDir, err := os.MkdirTemp("", "docksmith-run-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	for _, layer := range img.Layers {
		tarPath := r.store.LayerPath(layer.Digest)
		if err := builder.ExtractLayer(tarPath, tmpDir); err != nil {
			return fmt.Errorf("extract layer %s: %w", layer.Digest, err)
		}
	}

	// Build env: image ENV first, then overrides
	envMap := make(map[string]string)
	for _, kv := range img.Config.Env {
		k, v := splitKV(kv)
		envMap[k] = v
	}
	for _, kv := range envOverrides {
		k, v := splitKV(kv)
		envMap[k] = v
	}
	envVars := envMapToSlice(envMap)
	envVars = append(envVars, "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")

	// Determine command
	var cmd []string
	if len(overrideCmd) > 0 {
		cmd = overrideCmd
	} else {
		cmd = img.Config.Cmd
	}

	workdir := img.Config.WorkingDir
	if workdir == "" {
		workdir = "/"
	}

	// Join command into shell string for RunIsolated
	cmdStr := shellJoin(cmd)

	if err := builder.RunIsolated(tmpDir, cmdStr, envVars, workdir); err != nil {
		// Print exit code if it's an exit error
		fmt.Fprintf(os.Stderr, "container exited with error: %v\n", err)
		return err
	}
	return nil
}

func splitKV(kv string) (string, string) {
	idx := strings.IndexByte(kv, '=')
	if idx < 0 {
		return kv, ""
	}
	return kv[:idx], kv[idx+1:]
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

func shellJoin(args []string) string {
	if len(args) == 1 {
		return args[0]
	}
	quoted := make([]string, len(args))
	for i, a := range args {
		if strings.ContainsAny(a, " \t\"'\\") {
			quoted[i] = fmt.Sprintf("'%s'", strings.ReplaceAll(a, "'", "'\\''"))
		} else {
			quoted[i] = a
		}
	}
	return strings.Join(quoted, " ")
}
