package lsp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/coalaura/plain"
)

func (s *Server) RunCI(configPath string) int {
	s.IsCI = true

	s.Log = plain.New(
		plain.WithTarget(os.Stderr),
		plain.WithDate(plain.RFC3339Local),
	)

	b, err := os.ReadFile(configPath)
	if err != nil {
		s.Log.Errorf("Failed to read CI config: %v\n", err)

		return 1
	}

	var cfg CIConfig

	err = json.Unmarshal(b, &cfg)
	if err != nil {
		s.Log.Errorf("Failed to parse CI config: %v\n", err)

		return 1
	}

	for _, folder := range cfg.WorkspaceFolders {
		absPath, _ := filepath.Abs(folder)
		uri := s.normalizeURI(s.pathToURI(absPath))

		s.WorkspaceFolders = append(s.WorkspaceFolders, uri)
		s.lowerWorkspaceFolders = append(s.lowerWorkspaceFolders, strings.ToLower(s.uriToPath(uri)))
	}

	if len(s.WorkspaceFolders) > 0 {
		s.RootURI = s.WorkspaceFolders[0]
		s.lowerRootPath = s.lowerWorkspaceFolders[0]
	}

	s.applyInitializationOptions(cfg.Settings)

	s.refreshWorkspace()

	s.Log.Printf("CI completed. Found %d diagnostics (%d errors).\n", s.CIDiagnosticCount, s.CIErrorCount)

	if s.CIErrorCount > 0 {
		return 1
	}

	return 0
}

func (s *Server) printCIDiagnostics(uri string, diags []Diagnostic) {
	path := s.uriToPath(uri)

	for _, diag := range diags {
		s.CIDiagnosticCount++

		if diag.Severity == SeverityError {
			s.CIErrorCount++
		}

		level := "warning"

		switch diag.Severity {
		case SeverityError:
			level = "error"
		case SeverityHint, SeverityInformation:
			level = "notice"
		}

		line := diag.Range.Start.Line + 1
		col := diag.Range.Start.Character + 1

		msg := strings.ReplaceAll(diag.Message, "\n", " ")

		fmt.Fprintf(s.Writer, "::%s file=%s,line=%d,col=%d::%s\n", level, path, line, col, msg)
	}
}
