package lsp

import "strings"

type LuaDocParam struct {
	Name string
	Type string
	Desc string
}

type LuaDocReturn struct {
	Type string
	Desc string
}

type LuaDocField struct {
	Name string
	Type string
	Desc string
}

type LuaDocClass struct {
	Name   string
	Parent string
	Desc   string
}

type LuaDocType struct {
	Type string
	Desc string
}

type LuaDocAlias struct {
	Name string
	Type string
	Desc string
}

type LuaDocGeneric struct {
	Name   string
	Parent string
}

type LuaDoc struct {
	Description   string
	Class         *LuaDocClass
	Type          *LuaDocType
	Alias         *LuaDocAlias
	Generics      []LuaDocGeneric
	Params        []LuaDocParam
	Returns       []LuaDocReturn
	Fields        []LuaDocField
	Overloads     []string
	See           []string
	IsDeprecated  bool
	DeprecatedMsg string
}

// findTypeEnd safely scans past complex types with spaces like 'fun(a: string): number'
func findTypeEnd(s string) int {
	var depth int

	for i := 0; i < len(s); i++ {
		c := s[i]

		if c == '(' || c == '<' || c == '{' || c == '[' {
			depth++
		} else if c == ')' || c == '>' || c == '}' || c == ']' {
			depth--
		} else if (c == ' ' || c == '\t') && depth <= 0 {
			return i
		}
	}

	return -1
}

func extractTypeDesc(s string) (typ, desc string) {
	s = strings.TrimSpace(s)

	idx := findTypeEnd(s)
	if idx == -1 {
		return s, ""
	}

	return s[:idx], strings.TrimSpace(s[idx:])
}

func extractNameParent(s string) (name, parent, desc string) {
	s = strings.TrimSpace(s)

	var nameEnd int

	for nameEnd < len(s) && s[nameEnd] != ' ' && s[nameEnd] != '\t' && s[nameEnd] != ':' {
		nameEnd++
	}

	if nameEnd == 0 {
		return "", "", s
	}

	name = s[:nameEnd]
	s = strings.TrimSpace(s[nameEnd:])

	if strings.HasPrefix(s, ":") {
		s = strings.TrimSpace(s[1:])

		var parentEnd int

		for parentEnd < len(s) && s[parentEnd] != ' ' && s[parentEnd] != '\t' {
			parentEnd++
		}

		parent = s[:parentEnd]
		desc = strings.TrimSpace(s[parentEnd:])
	} else {
		desc = s
	}

	return name, parent, desc
}

func parseLuaDoc(comments string) LuaDoc {
	var (
		doc       LuaDoc
		descLines []string
	)

	for line := range strings.SplitSeq(comments, "\n") {
		line = strings.TrimSpace(line)

		if after, ok := strings.CutPrefix(line, "@param"); ok {
			rest := strings.TrimSpace(after)
			idx := strings.IndexByte(rest, ' ')

			if idx != -1 {
				name := rest[:idx]
				rest = strings.TrimSpace(rest[idx:])

				typ, desc := extractTypeDesc(rest)
				desc = strings.TrimPrefix(desc, "- ")

				if strings.HasSuffix(name, "?") {
					name = name[:len(name)-1]
					typ += "?"
				}

				doc.Params = append(doc.Params, LuaDocParam{Name: name, Type: typ, Desc: desc})
			} else {
				name := strings.TrimSuffix(rest, "?")

				doc.Params = append(doc.Params, LuaDocParam{Name: name})
			}
		} else if after, ok := strings.CutPrefix(line, "@return"); ok {
			typ, desc := extractTypeDesc(strings.TrimSpace(after))

			desc = strings.TrimPrefix(desc, "- ")
			desc = strings.TrimPrefix(desc, "# ")

			doc.Returns = append(doc.Returns, LuaDocReturn{Type: typ, Desc: desc})
		} else if after, ok := strings.CutPrefix(line, "@field"); ok {
			rest := strings.TrimSpace(after)

			if strings.HasPrefix(rest, "public ") {
				rest = strings.TrimSpace(rest[7:])
			} else if strings.HasPrefix(rest, "private ") {
				rest = strings.TrimSpace(rest[8:])
			} else if strings.HasPrefix(rest, "protected ") {
				rest = strings.TrimSpace(rest[10:])
			}

			idx := strings.IndexByte(rest, ' ')
			if idx != -1 {
				name := rest[:idx]
				rest = strings.TrimSpace(rest[idx:])

				typ, desc := extractTypeDesc(rest)
				desc = strings.TrimPrefix(desc, "- ")

				if strings.HasSuffix(name, "?") {
					name = name[:len(name)-1]
					typ = "?" + typ
				}

				doc.Fields = append(doc.Fields, LuaDocField{Name: name, Type: typ, Desc: desc})
			} else {
				name := strings.TrimSuffix(rest, "?")

				doc.Fields = append(doc.Fields, LuaDocField{Name: name})
			}
		} else if after, ok := strings.CutPrefix(line, "@deprecated"); ok {
			doc.IsDeprecated = true
			doc.DeprecatedMsg = strings.TrimSpace(after)
		} else if after, ok := strings.CutPrefix(line, "@class"); ok {
			name, parent, desc := extractNameParent(after)
			if name != "" {
				doc.Class = &LuaDocClass{Name: name, Parent: parent, Desc: desc}
			}
		} else if after, ok := strings.CutPrefix(line, "@type"); ok {
			typ, desc := extractTypeDesc(after)
			doc.Type = &LuaDocType{Type: typ, Desc: desc}
		} else if after, ok := strings.CutPrefix(line, "@alias"); ok {
			rest := strings.TrimSpace(after)

			idx := strings.IndexByte(rest, ' ')
			if idx != -1 {
				name := rest[:idx]
				rest = strings.TrimSpace(rest[idx:])

				typ, desc := extractTypeDesc(rest)
				doc.Alias = &LuaDocAlias{Name: name, Type: typ, Desc: desc}
			} else {
				doc.Alias = &LuaDocAlias{Name: rest}
			}
		} else if after, ok := strings.CutPrefix(line, "@generic"); ok {
			rest := strings.TrimSpace(after)
			for gStr := range strings.SplitSeq(rest, ",") {
				name, parent, _ := extractNameParent(gStr)
				if name != "" {
					doc.Generics = append(doc.Generics, LuaDocGeneric{Name: name, Parent: parent})
				}
			}
		} else if after, ok := strings.CutPrefix(line, "@overload"); ok {
			doc.Overloads = append(doc.Overloads, strings.TrimSpace(after))
		} else if after, ok := strings.CutPrefix(line, "@see"); ok {
			doc.See = append(doc.See, strings.TrimSpace(after))
		} else {
			if len(line) > 0 && !strings.HasPrefix(line, "@") {
				descLines = append(descLines, line)
			}
		}
	}

	doc.Description = strings.TrimSpace(strings.Join(descLines, "\n"))

	return doc
}
