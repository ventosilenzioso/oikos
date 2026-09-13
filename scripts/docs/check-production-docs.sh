#!/usr/bin/env bash
set -euo pipefail

docs=(docs/architecture.md docs/installation.md docs/operations.md docs/disaster-recovery.md docs/security.md docs/egg-spec.md docs/release-checklist.md)
for doc in "${docs[@]}"; do [[ -f "$doc" ]] || { echo "missing documentation: $doc" >&2; exit 1; }; done
for needle in '/metrics' '/healthz' 'oikos doctor' 'oikos update' 'pending manual verification'; do
  if ! grep -R -F -- "$needle" docs >/dev/null; then echo "missing documentation marker: $needle" >&2; exit 1; fi
done
echo 'production docs check: PASS'
