package lsp

import (
	"strings"

	"github.com/coalaura/lugo/ast"
)

func (s *Server) ensureFiveMNativeBundleLoaded(doc *Document) string {
	if s == nil || doc == nil {
		return ""
	}

	selection := s.getFiveMNativeSelection(doc)
	if !selection.Active() || selection.Build == "" {
		return ""
	}

	uri := fiveMNativeBundleURI(selection.Build)
	if uri == "" {
		return ""
	}

	if _, ok := s.Documents[uri]; ok {
		return uri
	}

	b, err := stdlibFS.ReadFile("stdlib/fivem/" + selection.Build)
	if err != nil {
		return ""
	}

	s.updateDocument(uri, b)

	if _, ok := s.Documents[uri]; ok {
		return uri
	}

	return ""
}

func (s *Server) ensureFiveMNativeSymbol(doc *Document, name string) bool {
	if s == nil || doc == nil || name == "" {
		return false
	}

	bundleURI := s.ensureFiveMNativeBundleLoaded(doc)
	if bundleURI == "" {
		return false
	}

	key := GlobalKey{ReceiverHash: 0, PropHash: ast.HashBytes([]byte(name))}
	syms, ok := s.GlobalIndex[key]
	if !ok {
		return false
	}

	for _, sym := range syms {
		if sym.URI != bundleURI {
			continue
		}

		tgtDoc, ok := s.Documents[sym.URI]
		if !ok {
			continue
		}

		if s.canSeeSymbol(doc, tgtDoc) {
			return true
		}
	}

	return false
}

func countLoadedFiveMNativeBundles(s *Server) int {
	if s == nil {
		return 0
	}

	count := 0
	for uri := range s.Documents {
		if strings.HasPrefix(uri, "std:///fivem/") && isFiveMNativeBundleName(strings.TrimPrefix(uri, "std:///fivem/")) {
			count++
		}
	}

	return count
}
