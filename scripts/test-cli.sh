#!/usr/bin/env bash
# ============================================================================
#  ServerDash — CLI Functional Test
#
#  Tests the serverdash CLI management script in a Docker container with mocks.
#
#  Usage:
#    bash scripts/test-cli.sh
# ============================================================================

set -euo pipefail

# ==================== Configuration ====================
IMAGE_NAME="serverdash-cli-test"
CONTAINER=""
PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0

# ==================== Colors ====================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# ==================== Helpers ====================
pass() { ((PASS_COUNT++)) || true; echo -e "  ${GREEN}PASS${NC} $1"; }
fail() { ((FAIL_COUNT++)) || true; echo -e "  ${RED}FAIL${NC} $1"; }
skip() { ((SKIP_COUNT++)) || true; echo -e "  ${YELLOW}SKIP${NC} $1"; }

assert() {
    local name="$1"
    local exit_code="$2"
    if [[ "$exit_code" -eq 0 ]]; then
        pass "$name"
    else
        fail "$name"
    fi
}

# Assert output contains a string
assert_contains() {
    local name="$1"
    local output="$2"
    local expected="$3"
    if echo "$output" | grep -qF "$expected"; then
        pass "$name"
    else
        fail "$name (expected '$expected' in output)"
    fi
}

assert_not_contains() {
    local name="$1"
    local output="$2"
    local unexpected="$3"
    if echo "$output" | grep -qF "$unexpected"; then
        fail "$name (unexpected '$unexpected' in output)"
    else
        pass "$name"
    fi
}

cleanup() {
    echo ""
    echo -e "${CYAN}Cleaning up...${NC}"
    if [[ -n "$CONTAINER" ]]; then
        docker rm -f "$CONTAINER" &>/dev/null || true
    fi
    docker rmi -f "$IMAGE_NAME" &>/dev/null || true
}
trap cleanup EXIT

# ==================== Build Test Image ====================
echo -e "${CYAN}[0/7] Building test image...${NC}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

CONTAINER="serverdash-cli-test-$$"

# Create a temporary Dockerfile
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"; cleanup' EXIT

cat > "$TMPDIR/Dockerfile" <<'DOCKERFILE'
FROM almalinux:10

RUN dnf install -y curl procps-ng nmap-ncat bash && dnf clean all

# Create directories
RUN mkdir -p /usr/local/bin /var/lib/serverdash /var/log/serverdash /etc/serverdash

# Mock systemctl
RUN mkdir -p /tmp/mock-state
COPY mock-systemctl.sh /usr/local/bin/systemctl
RUN chmod +x /usr/local/bin/systemctl

# Mock journalctl
COPY mock-journalctl.sh /usr/local/bin/journalctl
RUN chmod +x /usr/local/bin/journalctl

# Mock serverdash-server binary
COPY mock-serverdash-server.sh /usr/local/bin/serverdash-server
RUN chmod +x /usr/local/bin/serverdash-server

# Mock caddy binary
COPY mock-caddy.sh /usr/local/bin/caddy
RUN chmod +x /usr/local/bin/caddy

# Create env file
RUN echo 'SERVERDASH_PORT=39921' > /etc/serverdash/serverdash.env && \
    echo 'SERVERDASH_DATA_DIR=/var/lib/serverdash' >> /etc/serverdash/serverdash.env

# Create dummy files
RUN echo '{}' > /var/lib/serverdash/Caddyfile && \
    echo '{"level":"info","msg":"caddy started"}' > /var/log/serverdash/caddy.log

# Create os-release
RUN echo 'PRETTY_NAME="AlmaLinux 10 (Test)"' > /etc/os-release

# Install the CLI script
COPY serverdash-cli.sh /usr/local/bin/serverdash
RUN chmod +x /usr/local/bin/serverdash

CMD ["sleep", "infinity"]
DOCKERFILE

# Mock systemctl
cat > "$TMPDIR/mock-systemctl.sh" <<'MOCK'
#!/usr/bin/env bash
echo "systemctl $*" >> /tmp/systemctl-calls.log

case "$1" in
    is-active)
        if [[ -f /tmp/mock-state/serverdash-active ]]; then
            exit 0
        else
            exit 3
        fi
        ;;
    start)
        touch /tmp/mock-state/serverdash-active
        ;;
    stop)
        rm -f /tmp/mock-state/serverdash-active
        ;;
    restart)
        touch /tmp/mock-state/serverdash-active
        ;;
    show)
        if [[ "$*" == *"MainPID"* ]]; then
            echo "12345"
        fi
        ;;
    *)
        ;;
esac
exit 0
MOCK

# Mock journalctl
cat > "$TMPDIR/mock-journalctl.sh" <<'MOCK'
#!/usr/bin/env bash
echo "journalctl $*" >> /tmp/journalctl-calls.log
echo "2024-01-01 00:00:00 serverdash[1234]: Panel started on port 39921"
echo "2024-01-01 00:00:01 serverdash[1234]: Ready to serve requests"
exit 0
MOCK

# Mock serverdash-server
cat > "$TMPDIR/mock-serverdash-server.sh" <<'MOCK'
#!/usr/bin/env bash
case "$1" in
    --version)
        echo "ServerDash v0.9.1"
        ;;
    --reset-password)
        echo "Admin password has been reset."
        touch /tmp/reset-password-called
        ;;
    *)
        echo "serverdash-server: unknown flag $1"
        exit 1
        ;;
esac
MOCK

# Mock caddy
cat > "$TMPDIR/mock-caddy.sh" <<'MOCK'
#!/usr/bin/env bash
case "$1" in
    version)
        echo "v2.7.0 h1:abc123"
        ;;
    start)
        # Start a tiny HTTP listener on port 2019 to simulate Caddy admin API
        if ! nc -z localhost 2019 2>/dev/null; then
            (while true; do echo -e "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\n{}" | nc -l -p 2019 2>/dev/null || break; done) &
            echo $! > /tmp/mock-state/caddy-pid
        fi
        ;;
    stop)
        if [[ -f /tmp/mock-state/caddy-pid ]]; then
            kill $(cat /tmp/mock-state/caddy-pid) 2>/dev/null || true
            rm -f /tmp/mock-state/caddy-pid
        fi
        # Kill any lingering nc on port 2019
        pkill -f "nc -l -p 2019" 2>/dev/null || true
        ;;
    *)
        ;;
esac
exit 0
MOCK

# Copy the CLI script
cp "$PROJECT_ROOT/scripts/serverdash-cli.sh" "$TMPDIR/serverdash-cli.sh"

docker build -t "$IMAGE_NAME" "$TMPDIR" > /dev/null 2>&1
echo -e "  ${GREEN}Image built${NC}"

# Start container
docker run -d --name "$CONTAINER" "$IMAGE_NAME" > /dev/null
echo -e "  ${GREEN}Container started${NC}"

# Helper to run commands in container
run() {
    docker exec "$CONTAINER" bash -c "$*" 2>&1
}

# ==================== Tests ====================

# ── Section 1: serverdash version ──
echo ""
echo -e "${CYAN}[1/7] Testing 'serverdash version'...${NC}"

OUT=$(run "serverdash version")
assert_contains "version: shows CLI version" "$OUT" "ServerDash CLI"
assert_contains "version: shows Server version" "$OUT" "ServerDash Server"
assert_contains "version: shows Caddy version" "$OUT" "Caddy"

OUT2=$(run "serverdash --version")
assert_contains "version: --version flag works" "$OUT2" "ServerDash CLI"

# ── Section 2: serverdash help ──
echo ""
echo -e "${CYAN}[2/7] Testing 'serverdash help'...${NC}"

OUT=$(run "serverdash help")
assert_contains "help: Panel Management section" "$OUT" "Panel Management:"
assert_contains "help: Caddy Management section" "$OUT" "Caddy Management:"
assert_contains "help: Other section" "$OUT" "Other:"
assert_contains "help: lists panel status" "$OUT" "panel status"
assert_contains "help: lists caddy upgrade" "$OUT" "caddy upgrade"
assert_contains "help: lists reset-password" "$OUT" "reset-password"

OUT2=$(run "serverdash --help")
assert_contains "help: --help flag works" "$OUT2" "Panel Management:"

# ── Section 3: serverdash panel ──
echo ""
echo -e "${CYAN}[3/7] Testing 'serverdash panel' commands...${NC}"

# Panel status (stopped)
run "rm -f /tmp/mock-state/serverdash-active" > /dev/null
OUT=$(run "serverdash panel status")
assert_contains "panel status (stopped): shows Stopped" "$OUT" "Stopped"

# Panel start
OUT=$(run "serverdash panel start")
assert_contains "panel start: shows started" "$OUT" "started successfully"

# Verify systemctl was called
CALLS=$(run "cat /tmp/systemctl-calls.log")
assert_contains "panel start: calls systemctl start" "$CALLS" "start serverdash"

# Panel status (running)
OUT=$(run "serverdash panel status")
assert_contains "panel status (running): shows Running" "$OUT" "Running"
assert_contains "panel status (running): shows PID" "$OUT" "PID"

# Panel start when already running
OUT=$(run "serverdash panel start")
assert_contains "panel start (already running): warns" "$OUT" "already running"

# Panel restart
run "> /tmp/systemctl-calls.log" > /dev/null
OUT=$(run "serverdash panel restart")
assert_contains "panel restart: shows restarted" "$OUT" "restarted successfully"
CALLS=$(run "cat /tmp/systemctl-calls.log")
assert_contains "panel restart: calls systemctl restart" "$CALLS" "restart serverdash"

# Panel stop
OUT=$(run "serverdash panel stop")
assert_contains "panel stop: shows stopped" "$OUT" "stopped"

# Panel stop when already stopped
OUT=$(run "serverdash panel stop")
assert_contains "panel stop (already stopped): warns" "$OUT" "not running"

# ── Section 4: serverdash panel logs ──
echo ""
echo -e "${CYAN}[4/7] Testing 'serverdash panel logs'...${NC}"

run "> /tmp/journalctl-calls.log" > /dev/null
run "serverdash panel logs" > /dev/null
CALLS=$(run "cat /tmp/journalctl-calls.log")
assert_contains "panel logs: default -n 100" "$CALLS" "-n 100"

run "> /tmp/journalctl-calls.log" > /dev/null
run "serverdash panel logs -n 50" > /dev/null
CALLS=$(run "cat /tmp/journalctl-calls.log")
assert_contains "panel logs -n 50: passes correct lines" "$CALLS" "-n 50"

# ── Section 5: serverdash caddy ──
echo ""
echo -e "${CYAN}[5/7] Testing 'serverdash caddy' commands...${NC}"

# Caddy status (stopped — no listener on 2019)
run "pkill -f 'nc -l -p 2019' 2>/dev/null; rm -f /tmp/mock-state/caddy-pid" > /dev/null || true
OUT=$(run "serverdash caddy status")
assert_contains "caddy status (stopped): shows Stopped" "$OUT" "Stopped"
assert_contains "caddy status: shows version" "$OUT" "2.7.0"

# Caddy start
OUT=$(run "serverdash caddy start")
assert_contains "caddy start: shows started" "$OUT" "started"

# Caddy status (running)
sleep 1
OUT=$(run "serverdash caddy status")
assert_contains "caddy status (running): shows Running" "$OUT" "Running"

# Caddy stop
OUT=$(run "serverdash caddy stop")
assert_contains "caddy stop: shows stopped" "$OUT" "stopped"

# Caddy logs
OUT=$(run "serverdash caddy logs")
assert_contains "caddy logs: shows log content" "$OUT" "caddy started"

# Caddy logs — missing file
run "rm -f /var/log/serverdash/caddy.log" > /dev/null
OUT=$(run "serverdash caddy logs 2>&1 || true")
assert_contains "caddy logs (no file): warns" "$OUT" "not found"
# Restore
run "echo '{\"msg\":\"caddy\"}' > /var/log/serverdash/caddy.log" > /dev/null

# Caddy not installed
run "mv /usr/local/bin/caddy /usr/local/bin/caddy.bak" > /dev/null
OUT=$(run "serverdash caddy status")
assert_contains "caddy status (not installed): shows Not installed" "$OUT" "Not installed"
run "mv /usr/local/bin/caddy.bak /usr/local/bin/caddy" > /dev/null

# ── Section 6: serverdash info & misc ──
echo ""
echo -e "${CYAN}[6/7] Testing 'serverdash info' and other commands...${NC}"

OUT=$(run "serverdash info")
assert_contains "info: Versions section" "$OUT" "Versions"
assert_contains "info: Services section" "$OUT" "Services"
assert_contains "info: Network section" "$OUT" "Network"
assert_contains "info: Paths section" "$OUT" "Paths"
assert_contains "info: System section" "$OUT" "System"

# reset-password
run "rm -f /tmp/reset-password-called" > /dev/null
run "touch /tmp/mock-state/serverdash-active" > /dev/null
OUT=$(run "serverdash reset-password")
assert_contains "reset-password: resets" "$OUT" "reset"
CALLED=$(run "test -f /tmp/reset-password-called && echo yes || echo no")
assert_contains "reset-password: calls server binary" "$CALLED" "yes"

# ── Section 7: Error handling & interactive ──
echo ""
echo -e "${CYAN}[7/7] Testing error handling and interactive mode...${NC}"

# Unknown command
OUT=$(run "serverdash foobar 2>&1 || echo EXIT_NONZERO")
assert_contains "unknown command: shows error" "$OUT" "Unknown command"
assert_contains "unknown command: exits non-zero" "$OUT" "EXIT_NONZERO"

# Unknown panel subcommand
OUT=$(run "serverdash panel foobar 2>&1 || echo EXIT_NONZERO")
assert_contains "unknown panel subcommand: exits non-zero" "$OUT" "EXIT_NONZERO"

# Unknown caddy subcommand
OUT=$(run "serverdash caddy foobar 2>&1 || echo EXIT_NONZERO")
assert_contains "unknown caddy subcommand: exits non-zero" "$OUT" "EXIT_NONZERO"

# Interactive mode — exit with 0
OUT=$(run "echo '0' | serverdash 2>&1 || true")
assert_contains "interactive: exit with 0" "$OUT" "Bye"

# ==================== Summary ====================
echo ""
echo "════════════════════════════════════════════"
echo -e "  ${GREEN}PASS: ${PASS_COUNT}${NC}  ${RED}FAIL: ${FAIL_COUNT}${NC}  ${YELLOW}SKIP: ${SKIP_COUNT}${NC}"
TOTAL=$((PASS_COUNT + FAIL_COUNT + SKIP_COUNT))
echo -e "  Total: ${TOTAL}"
echo "════════════════════════════════════════════"

if [[ $FAIL_COUNT -gt 0 ]]; then
    exit 1
fi
