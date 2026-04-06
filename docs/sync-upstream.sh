#!/usr/bin/env bash
# ServerDash App Store — Upstream Sync Script
#
# Features:
#   1. Sync newly added or updated apps from the Runtipi upstream
#   2. Check compatibility with ServerDash
#   3. Create a PR
#
# Usage:
#   ./sync-upstream.sh [--dry-run] [--app <app-id>]
#
# Environment variables:
#   UPSTREAM_REPO   Upstream repository (default: https://github.com/runtipi/runtipi-appstore)
#   AI_API_URL      AI API endpoint (OpenAI-compatible)
#   AI_API_KEY      AI API key
#   AI_MODEL        Model name (default: gpt-4o)

set -euo pipefail

# ── Configuration ──
UPSTREAM_REPO="${UPSTREAM_REPO:-https://github.com/runtipi/runtipi-appstore}"
UPSTREAM_BRANCH="master"
AI_API_URL="${AI_API_URL:-https://api.openai.com/v1}"
AI_MODEL="${AI_MODEL:-gpt-4o}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
APPS_DIR="$REPO_ROOT/apps"
COMPAT_DB="$REPO_ROOT/compatibility.json"
SYNC_LOG="$REPO_ROOT/SYNC_LOG.md"

DRY_RUN=false
SINGLE_APP=""

# ── Argument parsing ──
while [[ $# -gt 0 ]]; do
    case "$1" in
        --dry-run) DRY_RUN=true; shift ;;
        --app) SINGLE_APP="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

# ── Utility helpers ──
log() { echo "[$(date '+%H:%M:%S')] $*"; }
warn() { echo "[$(date '+%H:%M:%S')] ⚠ $*" >&2; }
err() { echo "[$(date '+%H:%M:%S')] ✗ $*" >&2; }

# ── Step 1: Add and fetch upstream ──
log "Step 1: Fetching upstream..."

if ! git remote | grep -q '^upstream$'; then
    git remote add upstream "$UPSTREAM_REPO"
fi

git fetch upstream "$UPSTREAM_BRANCH" --depth=1

# ── Step 2: Detect changed apps ──
log "Step 2: Detecting changed apps..."

# Get the app list from the upstream apps/ directory
UPSTREAM_APPS=$(git ls-tree --name-only "upstream/$UPSTREAM_BRANCH" -- apps/ 2>/dev/null | sed 's|^apps/||')

CHANGED_APPS=()
NEW_APPS=()
UPDATED_APPS=()

for app_id in $UPSTREAM_APPS; do
    [[ -z "$app_id" ]] && continue
    [[ "$app_id" == "." ]] && continue

    # If a single app is specified, only process that app
    if [[ -n "$SINGLE_APP" && "$app_id" != "$SINGLE_APP" ]]; then
        continue
    fi

    if [[ ! -d "$APPS_DIR/$app_id" ]]; then
        NEW_APPS+=("$app_id")
        CHANGED_APPS+=("$app_id")
    else
        # Compare config.json and docker-compose.yml differences
        upstream_config=$(git show "upstream/$UPSTREAM_BRANCH:apps/$app_id/config.json" 2>/dev/null || echo "")
        local_config=$(cat "$APPS_DIR/$app_id/config.json" 2>/dev/null || echo "")

        if [[ "$upstream_config" != "$local_config" ]]; then
            UPDATED_APPS+=("$app_id")
            CHANGED_APPS+=("$app_id")
        fi
    fi
done

log "Found ${#NEW_APPS[@]} new, ${#UPDATED_APPS[@]} updated apps"

if [[ ${#CHANGED_APPS[@]} -eq 0 ]]; then
    log "No changes detected. Done."
    exit 0
fi

# ── Step 3: Sync files ──
log "Step 3: Syncing app files..."

for app_id in "${CHANGED_APPS[@]}"; do
    log "  Syncing: $app_id"

    if $DRY_RUN; then
        log "  [dry-run] Would sync $app_id"
        continue
    fi

    # Create app directory
    mkdir -p "$APPS_DIR/$app_id/metadata"

    # Check out files from upstream
    git show "upstream/$UPSTREAM_BRANCH:apps/$app_id/config.json" > "$APPS_DIR/$app_id/config.json" 2>/dev/null || true
    git show "upstream/$UPSTREAM_BRANCH:apps/$app_id/docker-compose.yml" > "$APPS_DIR/$app_id/docker-compose.yml" 2>/dev/null || true
    git show "upstream/$UPSTREAM_BRANCH:apps/$app_id/metadata/description.md" > "$APPS_DIR/$app_id/metadata/description.md" 2>/dev/null || true

    # Sync logo, trying multiple formats
    for ext in jpg png svg webp; do
        if git show "upstream/$UPSTREAM_BRANCH:apps/$app_id/metadata/logo.$ext" > "$APPS_DIR/$app_id/metadata/logo.$ext" 2>/dev/null; then
            break
        else
            rm -f "$APPS_DIR/$app_id/metadata/logo.$ext"
        fi
    done
done

# ── Step 4: Compatibility checks ──
log "Step 4: Running compatibility checks..."

check_compatibility() {
    local app_id="$1"
    local config="$APPS_DIR/$app_id/config.json"
    local compose="$APPS_DIR/$app_id/docker-compose.yml"
    local issues=()
    local compat="full"

    # Check compose file
    if [[ -f "$compose" ]]; then
        # Security warnings
        if grep -q 'privileged: true' "$compose"; then
            issues+=('{"type":"security_warning","description":"Uses privileged mode","severity":"warning"}')
            [[ "$compat" == "full" ]] && compat="partial"
        fi
        if grep -q 'cap_add:' "$compose"; then
            issues+=('{"type":"security_warning","description":"Uses cap_add","severity":"warning"}')
            [[ "$compat" == "full" ]] && compat="partial"
        fi
        if grep -q 'docker.sock' "$compose"; then
            issues+=('{"type":"security_warning","description":"Mounts docker.sock","severity":"warning"}')
            [[ "$compat" == "full" ]] && compat="partial"
        fi
        if grep -q 'pid: host' "$compose"; then
            issues+=('{"type":"security_warning","description":"Uses host PID namespace","severity":"warning"}')
            [[ "$compat" == "full" ]] && compat="partial"
        fi

        # Check for unknown variables
        unknown_vars=$(grep -oP '\$\{(\w+)\}' "$compose" 2>/dev/null | sort -u | while read -r var; do
            var_name="${var#\$\{}"
            var_name="${var_name%\}}"
            # Check whether it is a known variable
            case "$var_name" in
                APP_ID|APP_PORT|APP_DATA_DIR|APP_DOMAIN|APP_HOST|LOCAL_DOMAIN|APP_EXPOSED|APP_PROTOCOL|ROOT_FOLDER_HOST|TZ|NETWORK_INTERFACE|DNS_IP|INTERNAL_IP|COMPOSE_PROJECT_NAME)
                    ;;
                *)
                    # Check whether it is defined in form_fields
                    if ! jq -r '.form_fields[]?.env_variable' "$config" 2>/dev/null | grep -q "^${var_name}$"; then
                        echo "$var_name"
                    fi
                    ;;
            esac
        done)

        if [[ -n "$unknown_vars" ]]; then
            for uv in $unknown_vars; do
                issues+=("{\"type\":\"missing_var\",\"description\":\"Unknown variable: \${$uv}\",\"severity\":\"warning\"}")
            done
            [[ "$compat" == "full" ]] && compat="partial"
        fi

        # Check for direct non-HTTP port exposure
        if grep -E '^\s+- "[0-9]+:[0-9]+/(tcp|udp)"' "$compose" >/dev/null 2>&1; then
            issues+=('{"type":"firewall_needed","description":"Exposes non-HTTP ports directly","severity":"info"}')
        fi
    fi

    # Output result
    local issues_json="["
    local first=true
    for issue in "${issues[@]}"; do
        if $first; then first=false; else issues_json+=","; fi
        issues_json+="$issue"
    done
    issues_json+="]"

    echo "{\"app_id\":\"$app_id\",\"compatibility\":\"$compat\",\"issues\":$issues_json}"
}

# Initialize or load compatibility database
if [[ ! -f "$COMPAT_DB" ]]; then
    echo '{}' > "$COMPAT_DB"
fi

COMPAT_RESULTS=()
for app_id in "${CHANGED_APPS[@]}"; do
    result=$(check_compatibility "$app_id")
    COMPAT_RESULTS+=("$result")

    compat=$(echo "$result" | jq -r '.compatibility')
    issues_count=$(echo "$result" | jq '.issues | length')

    if [[ "$compat" == "full" ]]; then
        log "  ✅ $app_id: fully compatible"
    elif [[ "$compat" == "partial" ]]; then
        warn "  ⚠ $app_id: partially compatible ($issues_count issues)"
    else
        err "  ✗ $app_id: incompatible"
    fi
done

# Update compatibility database
if ! $DRY_RUN; then
    for result in "${COMPAT_RESULTS[@]}"; do
        app_id=$(echo "$result" | jq -r '.app_id')
        # Merge into compatibility.json
        tmp=$(mktemp)
        jq --argjson entry "$result" --arg id "$app_id" '.[$id] = $entry' "$COMPAT_DB" > "$tmp" && mv "$tmp" "$COMPAT_DB"
    done
fi

# ── Step 5: Translation removed ──
log "Step 5: Skipped (zh translation support removed)"

# ── Step 6: Update sync log ──
log "Step 6: Updating sync log..."

if ! $DRY_RUN; then
    SYNC_DATE=$(date '+%Y-%m-%d %H:%M')
    {
        echo ""
        echo "## Sync: $SYNC_DATE"
        echo ""
        if [[ ${#NEW_APPS[@]} -gt 0 ]]; then
            echo "### New Apps (${#NEW_APPS[@]})"
            for app_id in "${NEW_APPS[@]}"; do
                compat=$(jq -r --arg id "$app_id" '.[$id].compatibility // "unknown"' "$COMPAT_DB")
                echo "- \`$app_id\` — $compat"
            done
            echo ""
        fi
        if [[ ${#UPDATED_APPS[@]} -gt 0 ]]; then
            echo "### Updated Apps (${#UPDATED_APPS[@]})"
            for app_id in "${UPDATED_APPS[@]}"; do
                echo "- \`$app_id\`"
            done
            echo ""
        fi
    } >> "$SYNC_LOG"
fi

# ── Done ──
log "Done! ${#NEW_APPS[@]} new, ${#UPDATED_APPS[@]} updated apps synced."

if $DRY_RUN; then
    log "[dry-run] No files were modified."
else
    log "Next steps:"
    log "  1. Review changes: git diff"
    log "  2. Commit: git add -A && git commit -m 'sync: upstream $(date +%Y%m%d)'"
    log "  3. Push and create PR"
fi
