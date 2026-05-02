package lsp

import (
	"errors"

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

	uri := s.findFiveMNativeBundleURI(selection.Build)
	if uri != "" {
		return uri
	}

	uri = fiveMNativeBundleURI(selection.Build)
	if uri == "" {
		return ""
	}

	if _, ok := s.Documents[uri]; ok {
		return uri
	}

	b, err := s.readFiveMNativeBundle(selection.Build)
	if err != nil {
		return ""
	}

	s.updateDocument(uri, b)

	if _, ok := s.Documents[uri]; ok {
		return uri
	}

	return ""
}

func (s *Server) readFiveMNativeBundle(name string) ([]byte, error) {
	if name == "" {
		return nil, errors.New("empty FiveM native bundle name")
	}

	if s != nil && s.fiveMNativeBundleLoader != nil {
		return s.fiveMNativeBundleLoader(name)
	}

	b, err := loadRuntimeFiveMNativeBundle(name)
	if err == nil {
		return b, nil
	}

	return stdlibFS.ReadFile("stdlib/fivem/" + name)
}

func (s *Server) ensureFiveMNativeSymbol(doc *Document, name string) bool {
	if s == nil || doc == nil || name == "" {
		return false
	}

	bundleURI := s.ensureFiveMNativeBundleLoaded(doc)
	if bundleURI == "" {
		return false
	}

	selection := s.getFiveMNativeSelection(doc)

	key := GlobalKey{ReceiverHash: 0, PropHash: ast.HashBytes([]byte(name))}
	syms, ok := s.GlobalIndex[key]
	if !ok {
		return false
	}

	for _, sym := range syms {
		tgtDoc, ok := s.Documents[sym.URI]
		if !ok {
			continue
		}

		if fiveMNativeBundleNameFromDocument(tgtDoc) != selection.Build && sym.URI != bundleURI {
			continue
		}

		if s.canSeeSymbol(doc, tgtDoc) {
			return true
		}
	}

	return false
}

func (s *Server) findFiveMNativeBundleURI(name string) string {
	if s == nil || name == "" {
		return ""
	}

	for uri, doc := range s.Documents {
		if fiveMNativeBundleNameFromDocument(doc) == name {
			return uri
		}
	}

	return ""
}

func countLoadedFiveMNativeBundles(s *Server) int {
	if s == nil {
		return 0
	}

	count := 0
	for _, doc := range s.Documents {
		if fiveMNativeBundleNameFromDocument(doc) != "" {
			count++
		}
	}

	return count
}
