package builder

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ExpandGlob expands a glob pattern relative to contextDir.
// Returns sorted list of matched absolute paths.
func ExpandGlob(contextDir, pattern string) ([]string, error) {
	// Handle ** (double star) by converting to filepath.Walk
	if strings.Contains(pattern, "**") {
		return expandDoubleGlob(contextDir, pattern)
	}

	absPattern := filepath.Join(contextDir, pattern)
	matches, err := filepath.Glob(absPattern)
	if err != nil {
		return nil, err
	}

	// Expand directories
	var result []string
	for _, m := range matches {
		fi, err := os.Stat(m)
		if err != nil {
			continue
		}
		if fi.IsDir() {
			sub, err := walkDir(m)
			if err != nil {
				return nil, err
			}
			result = append(result, sub...)
		} else {
			result = append(result, m)
		}
	}
	sort.Strings(result)
	return result, nil
}

func expandDoubleGlob(contextDir, pattern string) ([]string, error) {
	// Split on **
	parts := strings.SplitN(pattern, "**", 2)
	prefix := strings.TrimSuffix(parts[0], "/")
	suffix := strings.TrimPrefix(parts[1], "/")

	baseDir := filepath.Join(contextDir, prefix)

	var result []string
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() {
			return nil
		}
		if suffix == "" {
			result = append(result, path)
			return nil
		}
		// Check if the file matches the suffix pattern
		rel, _ := filepath.Rel(baseDir, path)
		matched, _ := filepath.Match(suffix, filepath.Base(rel))
		if matched {
			result = append(result, path)
		}
		return nil
	})
	sort.Strings(result)
	return result, err
}

func walkDir(dir string) ([]string, error) {
	var result []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			result = append(result, path)
		}
		return nil
	})
	return result, err
}
