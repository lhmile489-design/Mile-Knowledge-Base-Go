#!/usr/bin/env bash
# T13 compatibility check: our transaction and provenance files are read by the
# legacy classes and vice versa. The legacy sources are copied into a temp dir
# (never modified) and run with bun.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
LEGACY="${LEGACY_DIR:-E:/WebApp/Mile-Project/Mile-Knowledge-Base-master}"
WORK=$(mktemp -d "$ROOT/.compat-XXXXXX")
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$WORK/src/shared" "$WORK/src/main/services" "$WORK/vault/raws"
cp "$LEGACY/src/shared/provenance.ts" "$WORK/src/shared/"
cp "$LEGACY/src/shared/errors.ts" "$WORK/src/shared/"
cp "$LEGACY/src/shared/vault.ts" "$WORK/src/shared/"
cp "$LEGACY/src/main/services/TransactionService.ts" "$WORK/src/main/services/"
cp "$ROOT/scripts/compat/tsconfig.json" "$WORK/"
cp "$ROOT/scripts/compat/make-legacy.ts" "$ROOT/scripts/compat/verify-ours.ts" "$WORK/"

pass=0; fail=0
ok() { pass=$((pass+1)); echo "PASS $1"; }
bad() { fail=$((fail+1)); echo "FAIL $1 :: $2"; }

cd "$WORK" || exit 1
echo "== legacy writes, Go reads"
TXID=$(bun run make-legacy.ts "$WORK/vault" | tail -1)
cp "$WORK/vault/.mile-kb/provenance.json" "$WORK/expected-provenance.json"
OUT=$(cd "$ROOT" && go run ./cmd/compatcheck restore "$(cygpath -w "$WORK/vault")" "$TXID" 2>&1)
echo "$OUT" | grep -q 'raws/a.md' && ok "legacy transaction restorable by Go" || bad "legacy transaction restorable by Go" "$OUT"
[ "$(cat "$WORK/vault/raws/a.md")" = "legacy original" ] && ok "Go restored the legacy pre-image" || bad "Go restored the legacy pre-image" "$(cat "$WORK/vault/raws/a.md")"
(cd "$ROOT" && go run ./cmd/compatcheck rewrite-provenance "$(cygpath -w "$WORK/vault")" >/dev/null)
cmp -s "$WORK/expected-provenance.json" "$WORK/vault/.mile-kb/provenance.json" && ok "provenance bytes unchanged by a Go read-write cycle" || bad "provenance bytes" "$(diff "$WORK/expected-provenance.json" "$WORK/vault/.mile-kb/provenance.json" | head -4)"

echo "== Go writes, legacy reads"
TXID2=$(cd "$ROOT" && go run ./cmd/compatcheck touch "$(cygpath -w "$WORK/vault")" | tail -1)
(cd "$ROOT" && go run ./cmd/compatcheck write-provenance "$(cygpath -w "$WORK/vault")" >/dev/null)
cmp -s "$WORK/expected-provenance.json" "$WORK/vault/.mile-kb/provenance.json" && ok "Go writes the same provenance bytes as legacy" || bad "provenance bytes from Go" "$(diff "$WORK/expected-provenance.json" "$WORK/vault/.mile-kb/provenance.json" | head -4)"
VERIFY=$(bun run verify-ours.ts "$WORK/vault" "$TXID2" | tail -1)
echo "$VERIFY" | grep -qF '"status":"committed"' && ok "legacy reads the Go manifest" || bad "legacy reads the Go manifest" "$VERIFY"
echo "$VERIFY" | grep -qF '"content":"legacy original\n"' && ok "legacy restored the Go pre-image" || bad "legacy restored the Go pre-image" "$VERIFY"
echo "$VERIFY" | grep -qF '"provenanceStable":true' && ok "legacy parse+serialize is a fixed point on our file" || bad "provenance stable under legacy" "$VERIFY"

echo "RESULT pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
