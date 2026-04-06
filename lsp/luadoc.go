package lsp

import (
	"bytes"
	"strings"
)

var (
	tagParam      = []byte("@param")
	tagReturn     = []byte("@return")
	tagField      = []byte("@field")
	tagDeprecated = []byte("@deprecated")
	tagClass      = []byte("@class")
	tagType       = []byte("@type")
	tagAlias      = []byte("@alias")
	tagGeneric    = []byte("@generic")
	tagOverload   = []byte("@overload")
	tagSee        = []byte("@see")

	kwPublic    = []byte("public ")
	kwPrivate   = []byte("private ")
	kwProtected = []byte("protected ")

	dashPrefix = []byte("- ")
	hashPrefix = []byte("# ")
	qSuffix    = []byte("?")
)

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
	DeprecatedMsg string
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
}

// findTypeEnd safely scans past complex types with spaces like 'fun(a: string): number'
func findTypeEnd(data []byte) int {
	var depth int

	for i := 0; i < len(data); i++ {
		char := data[i]

		if char == '(' || char == '<' || char == '{' || char == '[' {
			depth++
		} else if char == ')' || char == '>' || char == '}' || char == ']' {
			depth--
		} else if (char == ' ' || char == '\t') && depth <= 0 {
			var j int

			for j = i - 1; j >= 0 && (data[j] == ' ' || data[j] == '\t'); j-- {
			}

			if j >= 0 && (data[j] == ':' || data[j] == ',' || data[j] == '|') {
				continue // Type is still going (e.g., 'fun(): boolean' or 'string | number')
			}

			var k int

			for k = i + 1; k < len(data) && (data[k] == ' ' || data[k] == '\t'); k++ {
			}

			if k < len(data) && (data[k] == '|' || data[k] == ',') {
				i = k - 1

				continue
			}

			return i
		}
	}

	return -1
}

func extractTypeDesc(data []byte) (typeBytes, descBytes []byte) {
	data = bytes.TrimSpace(data)

	idx := findTypeEnd(data)
	if idx == -1 {
		return data, nil
	}

	return data[:idx], bytes.TrimSpace(data[idx:])
}

func formatAlerts(text string) string {
	if text == "" {
		return ""
	}

	var hasAlert bool

	prefixes := []string{"NOTE:", "TODO:", "INFO:", "WARNING:", "WARN:", "IMPORTANT:", "FIXME:", "BUG:", "TIP:", "CAUTION:"}

	for _, p := range prefixes {
		if strings.Contains(text, p) {
			hasAlert = true

			break
		}
	}

	if !hasAlert {
		return text
	}

	lines := strings.Split(text, "\n")

	var sb strings.Builder

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if len(trimmed) > 0 {
			leadingSpaces := line[:len(line)-len(trimmed)]

			if strings.HasPrefix(trimmed, "NOTE:") {
				sb.WriteString(leadingSpaces)
				sb.WriteString("ℹ️ **NOTE:**")
				sb.WriteString(trimmed[5:])
			} else if strings.HasPrefix(trimmed, "TODO:") {
				sb.WriteString(leadingSpaces)
				sb.WriteString("📝 **TODO:**")
				sb.WriteString(trimmed[5:])
			} else if strings.HasPrefix(trimmed, "INFO:") {
				sb.WriteString(leadingSpaces)
				sb.WriteString("ℹ️ **INFO:**")
				sb.WriteString(trimmed[5:])
			} else if strings.HasPrefix(trimmed, "WARNING:") {
				sb.WriteString(leadingSpaces)
				sb.WriteString("⚠️ **WARNING:**")
				sb.WriteString(trimmed[8:])
			} else if strings.HasPrefix(trimmed, "WARN:") {
				sb.WriteString(leadingSpaces)
				sb.WriteString("⚠️ **WARN:**")
				sb.WriteString(trimmed[5:])
			} else if strings.HasPrefix(trimmed, "IMPORTANT:") {
				sb.WriteString(leadingSpaces)
				sb.WriteString("❗ **IMPORTANT:**")
				sb.WriteString(trimmed[10:])
			} else if strings.HasPrefix(trimmed, "FIXME:") {
				sb.WriteString(leadingSpaces)
				sb.WriteString("🔧 **FIXME:**")
				sb.WriteString(trimmed[6:])
			} else if strings.HasPrefix(trimmed, "BUG:") {
				sb.WriteString(leadingSpaces)
				sb.WriteString("🐛 **BUG:**")
				sb.WriteString(trimmed[4:])
			} else if strings.HasPrefix(trimmed, "TIP:") {
				sb.WriteString(leadingSpaces)
				sb.WriteString("💡 **TIP:**")
				sb.WriteString(trimmed[4:])
			} else if strings.HasPrefix(trimmed, "CAUTION:") {
				sb.WriteString(leadingSpaces)
				sb.WriteString("🛑 **CAUTION:**")
				sb.WriteString(trimmed[8:])
			} else {
				sb.WriteString(line)
			}
		} else {
			sb.WriteString(line)
		}

		if i < len(lines)-1 {
			sb.WriteByte('\n')
		}
	}

	return sb.String()
}

func extractNameParent(data []byte) (name, parent, descBytes []byte) {
	data = bytes.TrimSpace(data)

	var nameEnd int

	for nameEnd < len(data) && data[nameEnd] != ' ' && data[nameEnd] != '\t' && data[nameEnd] != ':' {
		nameEnd++
	}

	if nameEnd == 0 {
		return nil, nil, data
	}

	name = data[:nameEnd]
	data = bytes.TrimSpace(data[nameEnd:])

	if bytes.HasPrefix(data, []byte(":")) {
		data = bytes.TrimSpace(data[1:])

		var parentEnd int

		for parentEnd < len(data) && data[parentEnd] != ' ' && data[parentEnd] != '\t' {
			parentEnd++
		}

		parent = data[:parentEnd]
		descBytes = bytes.TrimSpace(data[parentEnd:])
	} else {
		descBytes = data
	}

	return name, parent, descBytes
}

func parseLuaDoc(comments []byte, enableAlerts bool) LuaDoc {
	var (
		doc       LuaDoc
		descLines [][]byte
		activeTag string
	)

	for len(comments) > 0 {
		var line []byte

		idx := bytes.IndexByte(comments, '\n')
		if idx == -1 {
			line = comments
			comments = nil
		} else {
			line = comments[:idx]
			comments = comments[idx+1:]
		}

		line = bytes.TrimSpace(line)

		if after, ok := bytes.CutPrefix(line, tagParam); ok {
			activeTag = "param"

			rest := bytes.TrimSpace(after)
			spaceIdx := bytes.IndexByte(rest, ' ')

			if spaceIdx != -1 {
				name := rest[:spaceIdx]
				rest = bytes.TrimSpace(rest[spaceIdx:])

				typeBytes, descBytes := extractTypeDesc(rest)
				descBytes = bytes.TrimPrefix(descBytes, dashPrefix)

				nameStr := string(name)
				typStr := string(typeBytes)

				if bytes.HasSuffix(name, qSuffix) {
					nameStr = nameStr[:len(nameStr)-1]
					typStr += "?"
				}

				doc.Params = append(doc.Params, LuaDocParam{Name: nameStr, Type: typStr, Desc: string(descBytes)})
			} else {
				nameStr := string(rest)

				if bytes.HasSuffix(rest, qSuffix) {
					nameStr = nameStr[:len(nameStr)-1]
				}

				doc.Params = append(doc.Params, LuaDocParam{Name: nameStr})
			}
		} else if after, ok := bytes.CutPrefix(line, tagReturn); ok {
			activeTag = "return"

			typeBytes, descBytes := extractTypeDesc(bytes.TrimSpace(after))

			descBytes = bytes.TrimPrefix(descBytes, dashPrefix)
			descBytes = bytes.TrimPrefix(descBytes, hashPrefix)

			doc.Returns = append(doc.Returns, LuaDocReturn{Type: string(typeBytes), Desc: string(descBytes)})
		} else if after, ok := bytes.CutPrefix(line, tagField); ok {
			activeTag = "field"

			rest := bytes.TrimSpace(after)

			if bytes.HasPrefix(rest, kwPublic) {
				rest = bytes.TrimSpace(rest[7:])
			} else if bytes.HasPrefix(rest, kwPrivate) {
				rest = bytes.TrimSpace(rest[8:])
			} else if bytes.HasPrefix(rest, kwProtected) {
				rest = bytes.TrimSpace(rest[10:])
			}

			spaceIdx := bytes.IndexByte(rest, ' ')
			if spaceIdx != -1 {
				name := rest[:spaceIdx]
				rest = bytes.TrimSpace(rest[spaceIdx:])

				typeBytes, descBytes := extractTypeDesc(rest)
				descBytes = bytes.TrimPrefix(descBytes, dashPrefix)

				nameStr := string(name)
				typStr := string(typeBytes)

				if bytes.HasSuffix(name, qSuffix) {
					nameStr = nameStr[:len(nameStr)-1]
					typStr = "?" + typStr
				}

				doc.Fields = append(doc.Fields, LuaDocField{Name: nameStr, Type: typStr, Desc: string(descBytes)})
			} else {
				nameStr := string(rest)

				if bytes.HasSuffix(rest, qSuffix) {
					nameStr = nameStr[:len(nameStr)-1]
				}

				doc.Fields = append(doc.Fields, LuaDocField{Name: nameStr})
			}
		} else if after, ok := bytes.CutPrefix(line, tagDeprecated); ok {
			activeTag = ""

			doc.IsDeprecated = true
			doc.DeprecatedMsg = string(bytes.TrimSpace(after))
		} else if after, ok := bytes.CutPrefix(line, tagClass); ok {
			activeTag = ""

			name, parent, desc := extractNameParent(after)
			if len(name) > 0 {
				doc.Class = &LuaDocClass{Name: string(name), Parent: string(parent), Desc: string(desc)}
			}
		} else if after, ok := bytes.CutPrefix(line, tagType); ok {
			activeTag = ""

			typeBytes, descBytes := extractTypeDesc(bytes.TrimSpace(after))

			doc.Type = &LuaDocType{Type: string(typeBytes), Desc: string(descBytes)}
		} else if after, ok := bytes.CutPrefix(line, tagAlias); ok {
			activeTag = ""

			rest := bytes.TrimSpace(after)

			spaceIdx := bytes.IndexByte(rest, ' ')
			if spaceIdx != -1 {
				name := rest[:spaceIdx]
				rest = bytes.TrimSpace(rest[spaceIdx:])

				typeBytes, descBytes := extractTypeDesc(rest)

				doc.Alias = &LuaDocAlias{Name: string(name), Type: string(typeBytes), Desc: string(descBytes)}
			} else {
				doc.Alias = &LuaDocAlias{Name: string(rest)}
			}
		} else if after, ok := bytes.CutPrefix(line, tagGeneric); ok {
			activeTag = ""

			rest := bytes.TrimSpace(after)

			for gStr := range bytes.SplitSeq(rest, []byte(",")) {
				name, parent, _ := extractNameParent(gStr)
				if len(name) > 0 {
					doc.Generics = append(doc.Generics, LuaDocGeneric{Name: string(name), Parent: string(parent)})
				}
			}
		} else if after, ok := bytes.CutPrefix(line, tagOverload); ok {
			activeTag = ""

			doc.Overloads = append(doc.Overloads, string(bytes.TrimSpace(after)))
		} else if after, ok := bytes.CutPrefix(line, tagSee); ok {
			activeTag = ""

			doc.See = append(doc.See, string(bytes.TrimSpace(after)))
		} else {
			if len(line) > 0 {
				if line[0] != '@' { // Ignore @meta, @diagnostic, etc.
					switch activeTag {
					case "param":
						if doc.Params[len(doc.Params)-1].Desc != "" {
							doc.Params[len(doc.Params)-1].Desc += "\n" + string(line)
						} else {
							doc.Params[len(doc.Params)-1].Desc = string(line)
						}
					case "return":
						if doc.Returns[len(doc.Returns)-1].Desc != "" {
							doc.Returns[len(doc.Returns)-1].Desc += "\n" + string(line)
						} else {
							doc.Returns[len(doc.Returns)-1].Desc = string(line)
						}
					case "field":
						if doc.Fields[len(doc.Fields)-1].Desc != "" {
							doc.Fields[len(doc.Fields)-1].Desc += "\n" + string(line)
						} else {
							doc.Fields[len(doc.Fields)-1].Desc = string(line)
						}
					default:
						descLines = append(descLines, line)
					}
				} else {
					activeTag = ""

					descLines = append(descLines, line)
				}
			} else {
				activeTag = ""

				descLines = append(descLines, line) // Preserve empty lines for paragraph gaps
			}
		}
	}

	// Clean up leading/trailing empty description lines
	for len(descLines) > 0 && len(descLines[0]) == 0 {
		descLines = descLines[1:]
	}

	for len(descLines) > 0 && len(descLines[len(descLines)-1]) == 0 {
		descLines = descLines[:len(descLines)-1]
	}

	if len(descLines) > 0 {
		doc.Description = string(bytes.Join(descLines, []byte("\n")))
	}

	if enableAlerts {
		doc.Description = formatAlerts(doc.Description)

		for i := range doc.Params {
			doc.Params[i].Desc = formatAlerts(doc.Params[i].Desc)
		}

		for i := range doc.Returns {
			doc.Returns[i].Desc = formatAlerts(doc.Returns[i].Desc)
		}

		for i := range doc.Fields {
			doc.Fields[i].Desc = formatAlerts(doc.Fields[i].Desc)
		}
	}

	return doc
}
