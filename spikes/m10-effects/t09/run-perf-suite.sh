#!/usr/bin/env bash
# M10 T09: AC-MAT-6/7 — kind=typical at std and high, plus the skill-page
# m8 / m9 regressions. Needs the e2e build (automation) in build/windows-amd64.
# Writes every log and both metrics files next to this script; the gates run
# at the end and their exit codes are what to report.
set -u
ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
HERE="$(cd "$(dirname "$0")" && pwd)"
OUT="$HERE/perf-suite"
mkdir -p "$OUT"
cd "$ROOT" || exit 1
if tasklist //FI "IMAGENAME eq Mile Knowledge Base.exe" 2>/dev/null | grep -q "Mile Knowledge"; then
  echo "ABORT: Mile Knowledge Base is already running"; exit 2
fi

echo "== perf.sh --effects std"
bash scripts/e2e/perf.sh --effects std > "$OUT/perf-std.txt" 2>&1; echo "exit $?"
echo "== perf.sh --effects high"
bash scripts/e2e/perf.sh --effects high > "$OUT/perf-high.txt" 2>&1; echo "exit $?"
echo "== perf-m1.sh (index; no material tier)"
bash scripts/e2e/perf-m1.sh > "$OUT/perf-m1.txt" 2>&1; echo "exit $?"
echo "== perf-ai.sh --effects std"
bash scripts/e2e/perf-ai.sh --effects std > "$OUT/perf-ai-std.txt" 2>&1; echo "exit $?"
echo "== perf-ai.sh --effects high"
bash scripts/e2e/perf-ai.sh --effects high > "$OUT/perf-ai-high.txt" 2>&1; echo "exit $?"

for LEVEL in std high; do
  python -I -X utf8 "$HERE/assemble-typical.py" \
    --perf "$OUT/perf-$LEVEL.txt" --m1 "$OUT/perf-m1.txt" --ai "$OUT/perf-ai-$LEVEL.txt" \
    --out "$(cygpath -w "$OUT/typical-$LEVEL.json")"
  echo "gate typical ($LEVEL):"
  python -I scripts/perf-gate.py --kind typical --input "$(cygpath -w "$OUT/typical-$LEVEL.json")"
  echo "gate exit $?"
done

echo "== perf-m8.sh (B-M10-03)"
bash scripts/e2e/perf-m8.sh > "$OUT/perf-m8.txt" 2>&1; echo "exit $?"
echo "== perf-m9.sh"
bash scripts/e2e/perf-m9.sh > "$OUT/perf-m9.txt" 2>&1; echo "exit $?"
echo "RESULT $(grep -c . "$OUT"/*.txt | tr '\n' ' ')"
