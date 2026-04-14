package cmd

import (
	"fmt"
	"os"
)

func Execute() error {
	if len(os.Args) < 2 {
		printUsage()
		return nil
	}

	switch os.Args[1] {
	case "build":
		return runBuild(os.Args[2:])
	case "images":
		return runImages()
	case "rmi":
		return runRmi(os.Args[2:])
	case "run":
		return runRun(os.Args[2:])
	case "import":
		return runImport(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		return fmt.Errorf("unknown command: %s", os.Args[1])
	}
}

func printUsage() {
	fmt.Println(`Usage: docksmith <command> [options]

Commands:
  build   -t <name:tag> [--no-cache] <context>   Build an image from a Docksmithfile
  images                                          List all images
  rmi     <name:tag>                              Remove an image
  run     [-e KEY=VALUE] <name:tag> [cmd...]      Run a container
  import  <tarball.tar> <name:tag>                Import a docker-saved image tarball`)
}
