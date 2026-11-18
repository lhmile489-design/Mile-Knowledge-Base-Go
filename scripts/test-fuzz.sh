#!/usr/bin/env bash
# G-FUZZ: `go test -fuzz` takes exactly one target, so run every target in
# every package that has one, sequentially. FUZZ_TIME overrides the duration.
set -u
cd "$(dirname "$0")/.." || exit 1
time="${FUZZ_TIME:-60s}"
targets=$(grep -rhoE '^func (Fuzz[A-Za-z0-9_]+)' --include='*_test.go' internal | sed 's/^func //' | sort -u)
if [ -z "$targets" ]; then
  echo "test-fuzz: no fuzz targets found" >&2
  exit 1
fi
status=0
for name in $targets; do
  pkg=$(grep -rl "^func ${name}(" --include='*_test.go' internal | head -1 | xargs -r dirname)
  echo "== fuzz ${name} (${pkg}, ${time})"
  go test -run='^$' -fuzz="^${name}\$" -fuzztime="$time" "./${pkg#./}" || status=1
done
exit "$status"
