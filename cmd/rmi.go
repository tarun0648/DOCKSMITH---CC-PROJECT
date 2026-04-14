package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docksmith/internal/store"
)

func runRmi(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("rmi requires <name:tag>")
	}
	nameTag := args[0]

	s, err := store.New()
	if err != nil {
		return err
	}

	name, tag := splitNameTag(nameTag)
	img, err := s.GetImage(name, tag)
	if err != nil {
		return fmt.Errorf("image not found: %s", nameTag)
	}

	// Remove layer files belonging to this image
	for _, layer := range img.Layers {
		layerPath := filepath.Join(s.LayersDir(), layer.Digest[7:]+".tar")
		if err := os.Remove(layerPath); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: could not remove layer %s: %v\n", layer.Digest, err)
		}
	}

	// Remove manifest
	if err := s.RemoveImage(name, tag); err != nil {
		return err
	}

	fmt.Printf("Deleted: %s:%s\n", name, tag)
	return nil
}

func splitNameTag(nameTag string) (string, string) {
	parts := strings.SplitN(nameTag, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return nameTag, "latest"
}
