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

type LuaDoc struct {
	Description   string
	Params        []LuaDocParam
	Returns       []LuaDocReturn
	Fields        []LuaDocField
	IsDeprecated  bool
	DeprecatedMsg string
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
			parts := strings.SplitN(rest, " ", 3)

			var p LuaDocParam

			if len(parts) > 0 {
				p.Name = parts[0]
			}

			if len(parts) > 1 {
				p.Type = parts[1]
			}

			if len(parts) > 2 {
				p.Desc = strings.TrimPrefix(parts[2], "- ")
			}

			if strings.HasSuffix(p.Name, "?") {
				p.Name = p.Name[:len(p.Name)-1]
				p.Type += "?"
			}

			doc.Params = append(doc.Params, p)
		} else if after0, ok0 := strings.CutPrefix(line, "@return"); ok0 {
			rest := strings.TrimSpace(after0)
			parts := strings.SplitN(rest, " ", 2)

			var r LuaDocReturn

			if len(parts) > 0 {
				r.Type = parts[0]
			}

			if len(parts) > 1 {
				desc := strings.TrimPrefix(parts[1], "- ")
				desc = strings.TrimPrefix(desc, "# ")

				r.Desc = desc
			}

			doc.Returns = append(doc.Returns, r)
		} else if after, ok := strings.CutPrefix(line, "@field"); ok {
			rest := strings.TrimSpace(after)
			parts := strings.Fields(rest)

			var (
				f   LuaDocField
				idx int
			)

			if len(parts) > idx && (parts[idx] == "public" || parts[idx] == "private" || parts[idx] == "protected") {
				idx++
			}

			if len(parts) > idx {
				f.Name = parts[idx]
				idx++

				if strings.HasSuffix(f.Name, "?") {
					f.Name = f.Name[:len(f.Name)-1]
					f.Type = "?" // Will be prepended to the real type
				}
			}

			if len(parts) > idx {
				f.Type = f.Type + parts[idx]
				idx++
			}

			if len(parts) > idx {
				desc := strings.Join(parts[idx:], " ")
				f.Desc = strings.TrimPrefix(desc, "- ")
			}

			doc.Fields = append(doc.Fields, f)
		} else if after, ok := strings.CutPrefix(line, "@deprecated"); ok {
			doc.IsDeprecated = true
			doc.DeprecatedMsg = strings.TrimSpace(after)
		} else if strings.HasPrefix(line, "@class") || strings.HasPrefix(line, "@type") || strings.HasPrefix(line, "@alias") {
			descLines = append(descLines, "*`"+line+"`*")
		} else {
			if !strings.HasPrefix(line, "@meta") && !strings.HasPrefix(line, "@diagnostic") {
				descLines = append(descLines, line)
			}
		}
	}

	doc.Description = strings.TrimSpace(strings.Join(descLines, "\n"))

	return doc
}
