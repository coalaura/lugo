package lsp

import "embed"

//go:embed stdlib/*.lua
var stdlibFS embed.FS
