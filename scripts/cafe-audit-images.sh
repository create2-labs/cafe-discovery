#!/usr/bin/env bash
# cafe-discovery: golangci-lint, govulncheck, build Dockerfile, Docker Scout, rapport.
# PERS-D1b: persistence image is built from cafe-persistence (not this repo).
# Compatible bash 3.2+ (macOS).
#
# Le backend nécessite l'image locale oleglod/cafe-crypto-backend:build-oqs (construire dans cafe-crypto-backend avant).
#
# Variables: IMAGE_PREFIX, IMAGE_TAG, REPORT_DIR, SKIP_SCOUT, OQS_BASE

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_NAME="cafe-discovery"
RUN_ID="${RUN_ID:-$(date +%Y%m%d-%H%M%S)}"
IMAGE_TAG="${IMAGE_TAG:-cafe-audit-$RUN_ID}"
IMAGE_PREFIX="${IMAGE_PREFIX:-oleglod}"
REPORT_DIR="${REPORT_DIR:-$REPO_ROOT/reports}"
REPORT_FILE="${REPORT_FILE:-$REPORT_DIR/cafe-discovery-security-audit-$RUN_ID.md}"
OQS_BASE="${OQS_BASE:-oleglod/cafe-crypto-backend}"
SKIP_SCOUT="${SKIP_SCOUT:-0}"

info()  { printf '%s\n' "→ $*"; }
warn()  { printf '%s\n' "⚠ $*" >&2; }
have() { command -v "$1" >/dev/null 2>&1; }

STATE_FILE=""
state_init() { STATE_FILE=$(mktemp); : >"$STATE_FILE"; }
state_set() { local k="$1" v="${2:-}"; grep -v "^${k}|" "$STATE_FILE" 2>/dev/null >"${STATE_FILE}.n" || true; echo "${k}|${v}" >>"${STATE_FILE}.n"; mv "${STATE_FILE}.n" "$STATE_FILE"; }
state_get() { grep "^${1}|" "$STATE_FILE" 2>/dev/null | head -1 | cut -d'|' -f2- || true; }

run_lint() {
  if ! have golangci-lint; then
    (cd "$REPO_ROOT" && go vet ./... 2>&1) && state_set lint "go vet OK" || state_set lint "échec go vet"
  else
    (cd "$REPO_ROOT" && golangci-lint run ./... 2>&1) && state_set lint "golangci-lint OK" || state_set lint "échec golangci-lint"
  fi
}
run_gov() {
  if ! have govulncheck; then state_set gov "govulncheck absent"; return; fi
  local out
  if out=$(cd "$REPO_ROOT" && govulncheck ./... 2>&1); then
    if echo "$out" | grep -q "Vulnerability #"; then state_set gov "alertes (voir log)"; printf '%s\n' "$out" >>"$REPORT_DIR/govulncheck-$REPO_NAME.log"; else state_set gov "OK"; fi
  else state_set gov "échec"; printf '%s\n' "$out" >>"$REPORT_DIR/govulncheck-$REPO_NAME.err" 2>/dev/null || true; fi
}

tag() { echo "${IMAGE_PREFIX}/$1:${IMAGE_TAG}"; }

scout_parse() { tr -d '\033' | grep -E 'Target[[:space:]]+│' | head -1 | \
  sed -E 's/.*[[:space:]]([0-9]+)C[[:space:]]+([0-9]+)H[[:space:]]+([0-9]+)M[[:space:]]+([0-9]+)L.*/C=\1 H=\2 M=\3 L=\4/' || echo "?"; }
scout_line() {
  [ "$SKIP_SCOUT" = 1 ] && { echo "SKIP"; return; }
  ! docker scout version >/dev/null 2>&1 && { echo "n/a"; return; }
  docker scout quickview "local://$1" 2>&1 | scout_parse
}

IM_BACK=""
SC_BACK=""
OK_BACK=0

main() {
  mkdir -p "$REPORT_DIR"
  state_init
  info "REPO_ROOT=$REPO_ROOT"

  run_lint || true
  run_gov || true

  if ! docker image inspect "${OQS_BASE}:build-oqs" >/dev/null 2>&1; then
    warn "Image ${OQS_BASE}:build-oqs absente — builder cafe-crypto-backend (./scripts/cafe-audit-images.sh) ou pull."
  fi

  IM_BACK="$(tag cafe-discovery-backend)"
  if ( cd "$REPO_ROOT" && docker build -f Dockerfile -t "$IM_BACK" . ); then
    OK_BACK=1
  else
    OK_BACK=0
    warn "build backend échoué"
  fi

  if [ "$OK_BACK" = 1 ]; then
    SC_BACK=$(scout_line "$IM_BACK" || true)
  else SC_BACK=KO; fi

  {
    echo "# cafe-discovery — audit"
    echo "- Généré: $(date -u '+%Y-%m-%d %H:%M UTC')"
    echo ""
    echo "## Analyse statique"
    echo "- linter: $(state_get lint | tr '|' /)"
    echo "- govulncheck: $(state_get gov | tr '|' /)"
    echo ""
    echo "## Images (Docker Scout quickview, Target)"
    echo "| id | image | build | C/H/M/L |"
    echo "|---|----|----|----|"
    bb="✗"; [ "$OK_BACK" = 1 ] && bb="✓"
    echo "| backend | \`$IM_BACK\` | $bb | $SC_BACK |"
    echo ""
    echo "Persistence: voir \`cafe-persistence/scripts/cafe-audit-images.sh\` (PERS-D1b)."
    echo ""
    if [ "$OK_BACK" = 1 ] && [ "$SKIP_SCOUT" != 1 ] && docker scout version >/dev/null 2>&1; then
      echo "## CVE — backend"
      docker scout cves "local://$IM_BACK" --format markdown 2>&1 || true
      echo ""
    fi
  } > "$REPORT_FILE"
  info "Rapport: $REPORT_FILE"
}

main
