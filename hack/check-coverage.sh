#!/usr/bin/env bash
set -euo pipefail

# Optional coverage profile from make cover-gate (unused; per-package cover is simpler).
_profile="${1:-}"

check() {
  local pkg=$1 min=$2
  local out pct

  out=$(go test -cover "$pkg" 2>&1)
  echo "$out"
  pct=$(echo "$out" | sed -n 's/.*coverage: \([0-9.]*\)% of statements.*/\1/p' | tail -1)
  if [[ -z "$pct" ]]; then
    echo "FAIL $pkg: could not parse coverage"
    return 1
  fi

  echo "  $pkg: ${pct}% (floor ${min}%)"
  if awk -v p="$pct" -v m="$min" 'BEGIN { exit !(p + 0 >= m + 0) }'; then
    echo "PASS $pkg"
    return 0
  fi

  echo "FAIL $pkg: ${pct}% < ${min}%"
  return 1
}

failed=0
check ./pkg/mapping 95 || failed=1
check ./cmd/argocd-config/commands 70 || failed=1
check ./pkg/validate 85 || failed=1
check ./pkg/convert 85 || failed=1

exit "$failed"
