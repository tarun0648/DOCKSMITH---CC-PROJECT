package builder

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractLayer extracts a tar file into destDir.
func ExtractLayer(tarPath, destDir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("open layer %s: %w", tarPath, err)
	}
	defer f.Close()
	return extractTar(f, destDir)
}

// ExtractLayerBytes extracts a tar from an in-memory byte slice.
func ExtractLayerBytes(data []byte, destDir string) error {
	r := bytes.NewReader(data)
	return extractTar(r, destDir)
}

func extractTar(r io.Reader, destDir string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Security: strip leading / or ../
		cleanName := filepath.Clean(strings.TrimPrefix(hdr.Name, "/"))
		if strings.HasPrefix(cleanName, "..") {
			continue
		}

		target := filepath.Join(destDir, cleanName)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)|0755); err != nil {
				return err
			}
		case tar.TypeSymlink:
			// Remove existing and create symlink
			os.Remove(target)
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)|0644)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
	return nil
}
