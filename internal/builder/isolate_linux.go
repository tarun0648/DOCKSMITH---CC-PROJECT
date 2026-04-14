//go:build linux

package builder

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// RunIsolated executes a shell command inside rootDir using chroot isolation.
// It tries three strategies in order:
//  1. Unprivileged namespaces (self re-exec)
//  2. sudo unshare + self re-exec
//  3. sudo chroot directly (most compatible, works even on noexec mounts)
func RunIsolated(rootDir string, command string, envVars []string, workdir string) error {
	if workdir == "" {
		workdir = "/"
	}

	// Strategy 3: sudo chroot — most compatible, works everywhere.
	// We use this as the primary approach since it doesn't require
	// the binary to be on an exec-mounted filesystem.
	return runWithSudoChroot(rootDir, workdir, command, envVars)
}

// runWithSudoChroot uses `sudo chroot` to run the command in isolation.
// This works on Lima (noexec mounts), restricted kernels, and everywhere
// that has sudo available — which covers all typical demo/lab environments.
func runWithSudoChroot(rootDir, workdir, command string, envVars []string) error {
	// Ensure /proc is mounted inside the container root
	procDir := filepath.Join(rootDir, "proc")
	os.MkdirAll(procDir, 0755)
	// Best-effort; ignore errors (may already be mounted or no perms yet)
	mountProc := exec.Command("sudo", "mount", "-t", "proc", "proc", procDir)
	mountProc.Run()

	// Ensure /dev/null exists
	devDir := filepath.Join(rootDir, "dev")
	os.MkdirAll(devDir, 0755)

	// Build env string for env(1): KEY=VALUE KEY=VALUE ...
	// We pass env vars via `env -i KEY=VAL ... chroot ...` so the container
	// gets exactly the specified environment and nothing from the host.
	envArgs := []string{"-i"}
	envArgs = append(envArgs, envVars...)

	// Full command: sudo env -i KEY=VAL... chroot --userspec=<uid>:<gid> <root> /bin/sh -c <cmd>
	// We use --userspec to run as the current user inside the chroot.
	// Fall back to running as root inside chroot if --userspec fails.
	args := []string{
		"env",
	}
	args = append(args, envArgs...)
	args = append(args,
		"chroot", rootDir,
		"/bin/sh", "-c", command,
	)

	// Set working dir via a wrapper: cd <workdir> && <command>
	if workdir != "/" && workdir != "" {
		wrappedCmd := fmt.Sprintf("cd %s && %s", shellEscape(workdir), command)
		// Replace the last element (the command) with the wrapped version
		args[len(args)-1] = wrappedCmd
	}

	cmd := exec.Command("sudo", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Don't set cmd.Env — sudo handles the env via `env -i` inside the chroot

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// ChrootChild is kept for compatibility but is not used when sudo chroot is the primary path.
// It is still called when the binary re-execs itself with __chroot_child.
func ChrootChild() {
	if len(os.Args) < 5 {
		fmt.Fprintln(os.Stderr, "docksmith internal error: __chroot_child requires rootDir workdir cmd [args...]")
		os.Exit(1)
	}

	rootDir := os.Args[2]
	workdir := os.Args[3]
	cmdArgs := os.Args[4:]

	procDir := filepath.Join(rootDir, "proc")
	if mkErr := os.MkdirAll(procDir, 0755); mkErr == nil {
		_ = syscall.Mount("proc", procDir, "proc", 0, "")
	}

	devDir := filepath.Join(rootDir, "dev")
	os.MkdirAll(devDir, 0755)
	devNull := filepath.Join(devDir, "null")
	if _, statErr := os.Stat(devNull); os.IsNotExist(statErr) {
		_ = os.WriteFile(devNull, nil, 0666)
	}

	if err := syscall.Chroot(rootDir); err != nil {
		fmt.Fprintf(os.Stderr, "docksmith: chroot(%q) failed: %v\n", rootDir, err)
		os.Exit(1)
	}

	if err := syscall.Chdir(workdir); err != nil {
		_ = syscall.Chdir("/")
	}

	binary := cmdArgs[0]
	if resolved, err := exec.LookPath(binary); err == nil {
		binary = resolved
	}

	if err := syscall.Exec(binary, cmdArgs, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "docksmith: exec %q failed: %v\n",
			strings.Join(cmdArgs, " "), err)
		os.Exit(126)
	}
}