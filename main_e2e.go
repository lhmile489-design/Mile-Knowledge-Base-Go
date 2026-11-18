//go:build e2e

package main

// The automation plugin drives a running app from outside (PLAN-M0 G-E2E).
// It is behind the e2e build tag so release builds never contain it.
import _ "github.com/egoist/mygo/plugins/automation"
