package main

import (
	"fmt"
	"os"

	"github.com/docksmith/cmd"
	"github.com/docksmith/internal/builder"
)

func main() {
	// Internal re-exec for chroot isolation
	if len(os.Args) >= 2 && os.Args[1] == "__chroot_child" {
		builder.ChrootChild()
		return
	}

	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
