// Package main — frontend embed.
//
// The //go:embed directive requires the target directory to be a
// sibling of the source file's directory. The embed lives in
// cmd/promptsheond/ so the dashboard assets in cmd/promptsheond/
// frontend/dist/ are picked up by the Go compiler.
package main

import "embed"

// frontendDist holds the embedded dashboard assets. The variable
// is package-private and consumed via fs.Sub in buildServer below.
var frontendDist embed.FS