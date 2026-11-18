// compatcheck drives the Go side of the M1 compatibility test with the legacy
// Electron app's transaction files (scripts/compat/legacy-compat.sh).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/mutation"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: compatcheck restore <vault> <txid> | touch <vault> | rewrite-provenance <vault>")
		os.Exit(2)
	}
	command, vault := os.Args[1], os.Args[2]
	manager := mutation.TxManager{Root: vault}

	switch command {
	case "restore":
		if len(os.Args) < 4 {
			fail("restore needs a transaction id")
		}
		restored, conflicted, _, f := manager.Restore(os.Args[3])
		if f != nil {
			fail(f.Message)
		}
		out, _ := json.Marshal(map[string]any{"restored": restored, "conflicted": conflicted})
		fmt.Println(string(out))
	case "touch":
		tx, err := manager.Begin("compat", "plan-compat")
		if err != nil {
			fail(err.Error())
		}
		target := filepath.Join(vault, "raws", "a.md")
		if err := manager.Capture(tx, "raws/a.md", target); err != nil {
			fail(err.Error())
		}
		if err := os.WriteFile(target, []byte("written by go\n"), 0o644); err != nil {
			fail(err.Error())
		}
		if err := manager.CommitWithAfterImages(tx); err != nil {
			fail(err.Error())
		}
		fmt.Println(tx.ID)
	case "write-provenance":
		store, err := mutation.OpenProvenance(vault)
		if err != nil {
			fail(err.Error())
		}
		if err := store.Record("projects/a<b>.md", "raws/a.md", 1, "hash"); err != nil {
			fail(err.Error())
		}
		fmt.Println("ok")
	case "rewrite-provenance":
		store, err := mutation.OpenProvenance(vault)
		if err != nil {
			fail(err.Error())
		}
		if err := store.Save(); err != nil {
			fail(err.Error())
		}
		fmt.Println("ok")
	default:
		fail("unknown command " + command)
	}
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
