# Docksmith

## Team Members

| Name | USN |
|------|-----|
| TARUN S | PES1UG23AM919 |
| Aditya Venkatesh  | PES1UG23AM024 |
| Adarsh R Menon | PES1UG23AM016 |
| Abhay H Bhargav | PES1UG23AM008 |

A simplified Docker-like build and runtime system built from scratch in Go.

Implements:
- A **build engine** that reads a `Docksmithfile` (6 instructions: `FROM`, `COPY`, `RUN`, `WORKDIR`, `ENV`, `CMD`)
- A **content-addressed layer store** under `~/.docksmith/`
- A **deterministic build cache** with correct invalidation
- A **container runtime** using Linux namespaces + chroot for process isolation

---

## How to Run on Mac

> **Important:** The container isolation uses Linux kernel features (PID/mount namespaces + chroot).  
> These are **not available natively on macOS**. You must use a Linux VM.

### Option A — Lima (recommended, lightweight)

[Lima](https://github.com/lima-vm/lima) runs a lightweight Linux VM using QEMU.

```bash
# 1. Install Lima and Go (on your Mac)
brew install lima go

# 2. Start a Linux VM named "docksmith"
limactl start --name=docksmith template://ubuntu

# 3. Shell into the VM
limactl shell docksmith

# ── You are now inside Linux ──────────────────────────────
# 4. Install Go inside the VM (if not present)
sudo apt-get update && sudo apt-get install -y golang-go

# 5. Install Docker inside the VM (needed once to pull base image)
sudo apt-get install -y docker.io
sudo usermod -aG docker $USER
newgrp docker

# 6. Clone / copy the project into the VM
#    Lima mounts your Mac home directory at /Users/<you>/
#    So if you cloned to ~/docksmith on Mac it's available at /Users/<you>/docksmith
cd /Users/$(logname)/docksmith   # adjust path as needed

# 7. Run one-time setup
bash setup.sh

# 8. Run the demo
cd sample-app
bash ../demo.sh
```

### Option B — Multipass (Ubuntu, very simple)

```bash
# 1. On your Mac:
brew install multipass

# 2. Launch an Ubuntu VM
multipass launch --name docksmith --cpus 2 --memory 2G --disk 20G

# 3. Shell in
multipass shell docksmith

# ── You are now inside Linux ──────────────────────────────
# 4. Install deps
sudo apt-get update
sudo apt-get install -y golang-go docker.io git
sudo usermod -aG docker $USER && newgrp docker

# 5. Get the code (transfer from Mac or git clone)
# Transfer: on your Mac run:
#   multipass transfer -r /path/to/docksmith docksmith:/home/ubuntu/docksmith

# 6. Setup + demo
cd ~/docksmith
bash setup.sh
cd sample-app && bash ../demo.sh
```

### Option C — WSL2 on Mac (Apple Silicon via UTM)

See [UTM](https://mac.getutm.app/) to run an Ubuntu ARM64 VM. Same steps as Option B inside the VM.

---

## Manual Step-by-Step

### 1. Prerequisites (inside Linux VM)

```bash
# Go 1.21+
go version

# Docker (one-time pull only; can be removed afterwards)
docker --version
```

### 2. One-Time Setup

```bash
# From the project root:
bash setup.sh
```

This will:
1. Pull `python:3.11-slim` via Docker and save it as a tarball
2. Build the `docksmith` binary
3. Install it to `~/.local/bin/docksmith`
4. Import the base image into `~/.docksmith/`

After this, **no internet connection is needed**.

### 3. Add docksmith to PATH

```bash
export PATH="$HOME/.local/bin:$PATH"
# Add that line to ~/.bashrc or ~/.zshrc to persist it
```

### 4. Verify Setup

```bash
docksmith images
# Should show: python   3.11-slim   <id>   <date>
```

---

## Building the Binary Manually

```bash
cd /path/to/docksmith
go build -o docksmith .
./docksmith --help
```

---

## All 8 Demo Steps

### Step 1 – Cold Build (all CACHE MISS)

```bash
cd sample-app
docksmith build -t myapp:latest .
```

Expected output:
```
Step 1/6 : FROM python:3.11-slim
Step 2/6 : WORKDIR /app
Step 3/6 : ENV APP_NAME=DocksmithApp
Step 4/6 : COPY app /app [CACHE MISS] 0.04s
Step 5/6 : RUN python3 -c "..." [CACHE MISS] 0.31s
Step 6/6 : CMD ["python3", "main.py"]

Successfully built a3f9b2c1d4e5 myapp:latest (0.35s)
```

### Step 2 – Warm Build (all CACHE HIT)

```bash
docksmith build -t myapp:latest .
```

Expected output: same steps, all show `[CACHE HIT]`, completes near-instantly.

### Step 3 – Partial Cache Invalidation

```bash
# Edit a source file
echo "# edit" >> app/main.py

docksmith build -t myapp:latest .
# COPY step → [CACHE MISS], RUN step → [CACHE MISS]
# Steps above (WORKDIR, ENV) are unaffected

# Revert
git checkout app/main.py
```

### Step 4 – List Images

```bash
docksmith images
# NAME       TAG         ID             CREATED
# myapp      latest      a3f9b2c1d4e5   2024-01-15 10:23:45
# python     3.11-slim   ...            ...
```

### Step 5 – Run Container

```bash
docksmith run myapp:latest
```

### Step 6 – Override Env

```bash
docksmith run -e GREETING=Bonjour myapp:latest
# Output shows: Bonjour, from DocksmithApp!

docksmith run -e GREETING=Hola -e RUN_MODE=debug myapp:latest
```

### Step 7 – Isolation Verification (PASS/FAIL)

```bash
docksmith run myapp:latest
# The app writes /tmp/container-proof.txt inside the container

# Check the host — should NOT exist:
ls /tmp/container-proof.txt
# ls: cannot access '/tmp/container-proof.txt': No such file or directory  ✓ PASS
```

### Step 8 – Remove Image

```bash
docksmith rmi myapp:latest

docksmith images
# myapp is gone; layer files removed from ~/.docksmith/layers/
```

---

## CLI Reference

```
docksmith build -t <name:tag> [--no-cache] <context>
docksmith images
docksmith rmi <name:tag>
docksmith run [-e KEY=VALUE ...] <name:tag> [cmd...]
docksmith import <tarball.tar> <name:tag>
```

### Flags

| Flag | Command | Description |
|------|---------|-------------|
| `-t name:tag` | `build` | Image name and tag to produce |
| `--no-cache` | `build` | Skip all cache lookups and writes |
| `-e KEY=VALUE` | `run` | Override / add an environment variable (repeatable) |

---

## Docksmithfile Reference

```dockerfile
FROM <name>[:<tag>]          # Set base image (must be first; must exist locally)
WORKDIR <path>               # Set working directory (no layer produced)
ENV <KEY>=<VALUE>            # Set env var injected into image + RUN commands (no layer)
COPY <src> <dest>            # Copy files from build context → image layer
RUN <command>                # Run command in isolated rootfs → image layer
CMD ["exec", "arg", ...]     # Default command (JSON array; no layer)
```

All 6 instructions are required. Any unrecognised instruction fails immediately with the line number.

---

## State Layout

```
~/.docksmith/
  images/          # JSON manifests:  <name>_<tag>.json
  layers/          # Tar archives:    <sha256hex>.tar
  cache/           # Cache index:     index.json
```

---

## Architecture Notes

### Isolation Mechanism

`docksmith` uses a **self-re-exec** pattern to avoid needing a setuid binary:

1. When `RunIsolated()` is called, the binary re-executes itself with the internal flag `__chroot_child`.
2. The child process is started with Linux namespace flags: `CLONE_NEWUTS | CLONE_NEWPID | CLONE_NEWNS`.
3. Inside the child, `chroot(2)` changes the root to the assembled layer filesystem.
4. The actual command is then `exec(2)`-ed inside the new root.

The **same** `RunIsolated()` function is used for:
- `RUN` instructions during `docksmith build`
- `docksmith run`

### Build Cache

Cache keys are SHA-256 hashes of:
- Previous layer digest (or base manifest digest for first layer)
- Full instruction text
- Current `WORKDIR` value
- All `ENV` values (sorted by key)
- For `COPY`: SHA-256 of each source file (sorted by path)

A cache miss cascades: all subsequent steps also miss.

### Reproducibility

Tar archives are always created with:
- Entries in **lexicographically sorted path order**
- File **timestamps zeroed** (`time.Unix(0,0)`)

This ensures identical content → identical digest on every build.

---

## Importing Additional Base Images

```bash
# Pull any image on a machine with Docker and internet:
docker pull alpine:3.18
docker save alpine:3.18 -o alpine-3.18.tar

# Import into Docksmith:
docksmith import alpine-3.18.tar alpine:3.18

# Verify:
docksmith images
```

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| `container isolation requires Linux` | You're on macOS/Windows bare metal. Use a Linux VM (see above). |
| `chroot failed: operation not permitted` | Run as root, or ensure `CLONE_NEWNS` is allowed (check `/proc/sys/kernel/unprivileged_userns_clone`). Some distros need `sudo sysctl kernel.unprivileged_userns_clone=1`. |
| `image not found: python:3.11-slim` | Run `bash setup.sh` first to import the base image. |
| Cache never hits | Check that your build context files haven't changed. Use `--no-cache` to force a clean build. |
| `exec ... failed` | The base image may not have `/bin/sh`. Check the image layers. |
