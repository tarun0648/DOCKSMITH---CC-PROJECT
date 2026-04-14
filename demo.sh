#!/usr/bin/env bash
# =============================================================================
# demo.sh  –  Runs all 8 demo steps from the Docksmith project spec.
# Must be run on Linux after setup.sh has been run.
# =============================================================================
set -euo pipefail

# Ensure docksmith is on PATH
export PATH="$HOME/.local/bin:$PATH"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="$SCRIPT_DIR/sample-app"
DS="docksmith"

RED='\033[0;31m'; GREEN='\033[0;32m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'
step() { echo ""; echo -e "${CYAN}${BOLD}━━━ Demo Step $1 ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"; echo -e "  $2"; echo ""; }
ok()   { echo -e "${GREEN}✓ PASS${NC}  $*"; }
fail() { echo -e "${RED}✗ FAIL${NC}  $*"; }

cd "$APP_DIR"

# Step 1: Cold build
step 1 "Cold build (all layer-producing steps should show [CACHE MISS])"
echo "$ docksmith build -t myapp:latest ."
$DS build -t myapp:latest .

# Step 2: Warm build
step 2 "Warm build (all layer-producing steps should show [CACHE HIT])"
echo "$ docksmith build -t myapp:latest ."
$DS build -t myapp:latest .

# Step 3: Edit source, partial invalidation
step 3 "Edit a source file -> COPY and RUN below it become [CACHE MISS]"
ORIG_CONTENT=$(cat app/main.py)
echo "  Appending a comment to app/main.py ..."
echo "# demo edit $(date +%s)" >> app/main.py
echo "$ docksmith build -t myapp:latest ."
$DS build -t myapp:latest .
printf '%s' "$ORIG_CONTENT" > app/main.py
echo "  (source file restored)"

# Step 4: List images
step 4 "List images"
echo "$ docksmith images"
$DS images

# Step 5: Run
step 5 "Run the container (expect visible output)"
echo "$ docksmith run myapp:latest"
$DS run myapp:latest

# Step 6: Override env
step 6 "Run with env override (-e GREETING=Bonjour)"
echo "$ docksmith run -e GREETING=Bonjour myapp:latest"
$DS run -e GREETING=Bonjour myapp:latest

# Step 7: Isolation check
step 7 "Isolation check: file written inside container must NOT appear on host"
echo "$ docksmith run myapp:latest"
$DS run myapp:latest
PROOF="/tmp/container-proof.txt"
if [[ -f "$PROOF" ]]; then
  fail "ISOLATION BROKEN: $PROOF found on host!"
else
  ok "Isolation verified: $PROOF does NOT exist on host."
fi

# Step 8: Remove image
step 8 "Remove the image"
echo "$ docksmith rmi myapp:latest"
$DS rmi myapp:latest
echo ""
echo "$ docksmith images"
$DS images

echo ""
echo -e "${GREEN}${BOLD}All demo steps complete.${NC}"
