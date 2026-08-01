// Package main — dashboard asset embedding.
//
// The //go:embed directive requires the target directory to be a
// sibling of the source file's directory. The embed lives in
// cmd/promptsheond/ so the dashboard assets under
// cmd/promptsheond/frontend/dist/ are picked up by the Go
// compiler.
package main

import "embed"

//go:embed all:frontend/dist
var frontendDist embed.FS
