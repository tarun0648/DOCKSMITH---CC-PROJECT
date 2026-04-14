package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/docksmith/internal/builder"
)

func runBuild(args []string) error {
	var tag string
	var noCache bool
	var context string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-t":
			if i+1 >= len(args) {
				return fmt.Errorf("-t requires an argument")
			}
			i++
			tag = args[i]
		case "--no-cache":
			noCache = true
		default:
			if strings.HasPrefix(args[i], "-") {
				return fmt.Errorf("unknown flag: %s", args[i])
			}
			context = args[i]
		}
	}

	if tag == "" {
		return fmt.Errorf("build requires -t <name:tag>")
	}
	if context == "" {
		return fmt.Errorf("build requires a context directory")
	}

	abs, err := absPath(context)
	if err != nil {
		return err
	}

	b, err := builder.New(abs, tag, noCache)
	if err != nil {
		return err
	}
	return b.Build()
}

func absPath(p string) (string, error) {
	if p == "." {
		return os.Getwd()
	}
	return p, nil
}
