package lsp

import (
	"embed"
	"strings"
)

//go:embed stdlib/*.lua stdlib/fivem/*.lua
var stdlibFS embed.FS

var fiveMNativeBundleNames = map[string]struct{}{
	"natives_21e43a33.lua":  {},
	"natives_0193d0af.lua":  {},
	"natives_universal.lua": {},
	"rdr3_universal.lua":    {},
	"ny_universal.lua":      {},
	"natives_server.lua":    {},
}

func isFiveMNativeBundleName(name string) bool {
	_, ok := fiveMNativeBundleNames[name]
	return ok
}

func isFiveMNativeBundlePath(path string) bool {
	if !strings.HasPrefix(path, "fivem/") {
		return false
	}

	return isFiveMNativeBundleName(strings.TrimPrefix(path, "fivem/"))
}

func fiveMNativeBundleURI(name string) string {
	if name == "" {
		return ""
	}

	return "std:///fivem/" + name
}

func fiveMNativeBundleNameFromURI(uri string) string {
	if !strings.HasPrefix(uri, "std:///fivem/") {
		return ""
	}

	name := strings.TrimPrefix(uri, "std:///fivem/")
	if !isFiveMNativeBundleName(name) {
		return ""
	}

	return name
}
