package builder

import (
	"archive/tar"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var zeroTime = time.Unix(0, 0)

// CreateLayer creates a tar archive from files rooted at srcDir,
// writing only entries whose paths are in the provided list (relative to srcDir).
// If paths is nil, all files under srcDir are included.
// Entries are sorted for reproducibility; timestamps are zeroed.
// Returns sha256 digest and raw bytes.
func CreateLayerFromPaths(files []fileTuple) (digest string, tarBytes []byte, err error) {
	// Sort by path for reproducibility
	sort.Slice(files, func(i, j int) bool {
		return files[i].archivePath < files[j].archivePath
	})

	buf := &bytesBuffer{}
	h := sha256.New()
	mw := io.MultiWriter(buf, h)
	tw := tar.NewWriter(mw)

	for _, ft := range files {
		if err := addToTar(tw, ft.srcPath, ft.archivePath); err != nil {
			return "", nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return "", nil, err
	}

	digest = fmt.Sprintf("sha256:%x", h.Sum(nil))
	return digest, buf.bytes, nil
}

type fileTuple struct {
	srcPath     string // absolute path on host
	archivePath string // path inside the tar (relative, no leading /)
}

func addToTar(tw *tar.Writer, src, archivePath string) error {
	fi, err := os.Lstat(src)
	if err != nil {
		return err
	}

	// Mask to only permission bits — directory/symlink type bits must not
	// be included in the tar header Mode field or it overflows.
	perm := int64(fi.Mode().Perm())

	if fi.IsDir() {
		hdr := &tar.Header{
			Typeflag:   tar.TypeDir,
			Name:       strings.TrimSuffix(archivePath, "/") + "/",
			Mode:       perm | 0755,
			ModTime:    zeroTime,
			ChangeTime: zeroTime,
			AccessTime: zeroTime,
		}
		return tw.WriteHeader(hdr)
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		hdr := &tar.Header{
			Typeflag:   tar.TypeSymlink,
			Name:       archivePath,
			Linkname:   target,
			Mode:       perm,
			ModTime:    zeroTime,
			ChangeTime: zeroTime,
			AccessTime: zeroTime,
		}
		return tw.WriteHeader(hdr)
	}

	hdr := &tar.Header{
		Typeflag:   tar.TypeReg,
		Name:       archivePath,
		Mode:       perm,
		Size:       fi.Size(),
		ModTime:    zeroTime,
		ChangeTime: zeroTime,
		AccessTime: zeroTime,
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(tw, f)
	return err
}

// CollectFiles walks a directory and returns all files as fileTuples.
func CollectFiles(root string) ([]fileTuple, error) {
	var files []fileTuple
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		archivePath := filepath.ToSlash(rel)
		if info.IsDir() {
			archivePath += "/"
		}
		files = append(files, fileTuple{srcPath: path, archivePath: archivePath})
		return nil
	})
	return files, err
}

// HashFile returns the sha256 hex digest of a file's raw bytes.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// bytesBuffer is a simple io.Writer that accumulates bytes.
type bytesBuffer struct {
	bytes []byte
}

func (b *bytesBuffer) Write(p []byte) (int, error) {
	b.bytes = append(b.bytes, p...)
	return len(p), nil
}