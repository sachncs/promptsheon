// Package main is the single entry point for all Promptsheon binaries.
// The binary name (os.Args[0]) determines which mode runs:
//
//	promptsheond           → API server daemon (runDaemon)
//	promptsheon            → CLI (runCLI)
//	promptsheon-healthcheck → health probe (runHealthcheck)
//
// Build:
//
//	go build -o promptsheond .
//	go build -o promptsheon .
//	go build -o promptsheon-healthcheck .
package main

import (
	"os"
	"path/filepath"
	"strings"
)

func main() {
	bin := filepath.Base(os.Args[0])
	switch {
	case strings.HasPrefix(bin, "promptsheond"):
		runDaemon()
	case bin == "promptsheon-healthcheck" || strings.HasSuffix(bin, "-healthcheck"):
		runHealthcheck()
	default:
		runCLI()
	}
}
