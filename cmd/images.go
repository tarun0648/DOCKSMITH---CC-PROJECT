package cmd

import (
	"fmt"
	"sort"
	"time"

	"github.com/docksmith/internal/store"
)

func runImages() error {
	s, err := store.New()
	if err != nil {
		return err
	}

	images, err := s.ListImages()
	if err != nil {
		return err
	}

	sort.Slice(images, func(i, j int) bool {
		return images[i].Name < images[j].Name
	})

	fmt.Printf("%-20s %-12s %-14s %-20s\n", "NAME", "TAG", "ID", "CREATED")
	for _, img := range images {
		id := img.Digest
		if len(id) > 19 {
			// strip "sha256:" prefix then take 12 chars
			id = img.Digest[7:]
			if len(id) > 12 {
				id = id[:12]
			}
		}
		created, _ := time.Parse(time.RFC3339, img.Created)
		fmt.Printf("%-20s %-12s %-14s %-20s\n",
			img.Name,
			img.Tag,
			id,
			created.Format("2006-01-02 15:04:05"),
		)
	}
	return nil
}
