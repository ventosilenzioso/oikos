#!/usr/bin/env bash
set -euo pipefail

fake="$(mktemp)"
trap 'rm -f "$fake"; rm -rf artifacts/security-policy-test' EXIT
mkdir -p artifacts/security-policy-test

cat >"$fake" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' '{"finding":{"osv":"GO-2026-4887","trace":[{"module":"github.com/docker/docker","package":"github.com/docker/docker/client"},{"function":"RoundTrip"}]}}'
printf '%s\n' '{"finding":{"osv":"GO-2026-4883","trace":[{"module":"github.com/docker/docker","package":"github.com/docker/docker/client"},{"function":"RoundTrip"}]}}'
exit 3
FAKE
chmod 0755 "$fake"
if ! GOVULNCHECK_BIN="$fake" bash scripts/security/run-govulncheck.sh >/dev/null 2>&1; then
  echo 'allowlisted govulncheck IDs harus diterima' >&2
  exit 1
fi

cat >"$fake" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' '{"finding":{"osv":"GO-NEW-0001","trace":[{"module":"example.com/new","package":"example.com/new"},{"function":"Vulnerable"}]}}'
exit 3
FAKE
chmod 0755 "$fake"
if GOVULNCHECK_BIN="$fake" bash scripts/security/run-govulncheck.sh >/dev/null 2>&1; then
  echo 'govulncheck ID baru harus menggagalkan policy' >&2
  exit 1
fi
echo 'govulncheck policy tests: PASS'
