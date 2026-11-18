// Command genvault writes reproducible sample knowledge bases for tests,
// benchmarks and manual runs. It never touches a real vault.
//
//	go run ./cmd/genvault -kind typical -out %TEMP%/mkb-typical
//	go run ./cmd/genvault -kind big -out %TEMP%/mkb-big
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/testkit"
)

func main() {
	kind := flag.String("kind", "typical", "typical or big")
	out := flag.String("out", "", "destination directory (must not exist or be empty)")
	flag.Parse()
	if *out == "" {
		fmt.Fprintln(os.Stderr, "-out is required")
		os.Exit(2)
	}
	var err error
	switch *kind {
	case "typical":
		err = testkit.BuildTypical(*out)
	case "big":
		err = testkit.BuildBig(*out)
	default:
		err = fmt.Errorf("unknown kind %q", *kind)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "genvault:", err)
		os.Exit(1)
	}
	fmt.Println("genvault: wrote", *kind, "sample to", *out)
}
