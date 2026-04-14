package cmd

import (
	"fmt"
	"strings"

	"github.com/docksmith/internal/runtime"
	"github.com/docksmith/internal/store"
)

func runRun(args []string) error {
	var envOverrides []string
	var nameTag string
	var extraCmd []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-e":
			if i+1 >= len(args) {
				return fmt.Errorf("-e requires KEY=VALUE")
			}
			i++
			envOverrides = append(envOverrides, args[i])
		default:
			if strings.HasPrefix(args[i], "-e=") {
				envOverrides = append(envOverrides, strings.TrimPrefix(args[i], "-e="))
			} else if nameTag == "" {
				nameTag = args[i]
			} else {
				extraCmd = append(extraCmd, args[i])
			}
		}
	}

	if nameTag == "" {
		return fmt.Errorf("run requires <name:tag>")
	}

	s, err := store.New()
	if err != nil {
		return err
	}

	name, tag := splitNameTag(nameTag)
	img, err := s.GetImage(name, tag)
	if err != nil {
		return fmt.Errorf("image not found: %s", nameTag)
	}

	if len(extraCmd) == 0 && len(img.Config.Cmd) == 0 {
		return fmt.Errorf("no CMD defined in image and no command provided")
	}

	r, err := runtime.New(s)
	if err != nil {
		return err
	}

	return r.Run(img, envOverrides, extraCmd)
}
