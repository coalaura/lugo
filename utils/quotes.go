package utils

import (
	"strings"
	"unicode/utf8"
)

func UnquoteLuaString(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return s
	}

	switch s[0] {
	case '"', '\'':
		return unquoteShortLua(s)
	case '[':
		out, ok := unquoteLongLua(s)
		if ok {
			return out
		}
	}

	return s
}

func unquoteShortLua(s string) string {
	quote := s[0]
	body := s[1:]

	end := strings.IndexByte(body, quote)
	esc := strings.IndexByte(body, '\\')

	if esc == -1 || (end != -1 && esc > end) {
		if end == -1 {
			return body // unterminated
		}

		return body[:end]
	}

	var (
		index int
		sb    strings.Builder
	)

	sb.Grow(len(body))

	for index < len(body) {
		char := body[index]
		if char == quote {
			break // closing quote
		}

		if char != '\\' {
			j := index + 1

			for j < len(body) && body[j] != '\\' && body[j] != quote {
				j++
			}

			sb.WriteString(body[index:j])

			index = j

			continue
		}

		index++ // consume backslash

		if index >= len(body) {
			sb.WriteByte('\\') // dangling backslash; keep

			break
		}

		char = body[index]

		index++

		switch char {
		case 'a':
			sb.WriteByte('\a')
		case 'b':
			sb.WriteByte('\b')
		case 'f':
			sb.WriteByte('\f')
		case 'n':
			sb.WriteByte('\n')
		case 'r':
			sb.WriteByte('\r')
		case 't':
			sb.WriteByte('\t')
		case 'v':
			sb.WriteByte('\v')
		case '\\', '"', '\'':
			sb.WriteByte(char)
		case '\n': // \<newline> -> newline; \n\r counts as one break
			sb.WriteByte('\n')
			if index < len(body) && body[index] == '\r' {
				index++
			}
		case '\r':
			sb.WriteByte('\n')
			if index < len(body) && body[index] == '\n' {
				index++
			}
		case 'x': // \xXX
			if index+1 < len(body) {
				h1, h2 := hexVal(body[index]), hexVal(body[index+1])
				if h1 >= 0 && h2 >= 0 {
					sb.WriteByte(byte(h1<<4 | h2))

					index += 2

					continue
				}
			}
			sb.WriteString(`\x`) // malformed; keep
		case 'z': // skip following whitespace
			for index < len(body) && isLuaSpace(body[index]) {
				index++
			}
		case 'u': // \u{XXXX}
			if index < len(body) && body[index] == '{' {
				var (
					j  = index + 1
					r  int
					ok bool
				)

				for j < len(body) && body[j] != '}' {
					h := hexVal(body[j])
					if h < 0 || r > utf8.MaxRune {
						break
					}

					r = r<<4 | h

					j++
				}

				if j < len(body) && body[j] == '}' && j > index+1 && r <= utf8.MaxRune {
					ok = true
				}

				if ok {
					sb.WriteRune(rune(r))

					index = j + 1

					continue
				}
			}

			sb.WriteString(`\u`) // malformed; keep
		default:
			if char >= '0' && char <= '9' { // \ddd (up to 3 digits, <= 255)
				n := int(char - '0')

				for k := 0; k < 2 && index < len(body) && body[index] >= '0' && body[index] <= '9'; k++ {
					n = n*10 + int(body[index]-'0')

					index++
				}

				if n <= 255 {
					sb.WriteByte(byte(n))
				} else {
					sb.WriteByte('\\') // out of range; keep
					sb.WriteByte(char)
				}
			} else {
				sb.WriteByte('\\') // unknown escape; keep
				sb.WriteByte(char)
			}
		}
	}

	return sb.String()
}

// unquoteLongLua handles [[...]], [=[...]=], [==[...]==], etc.
func unquoteLongLua(s string) (string, bool) {
	var (
		level int
		index = 1
	)

	for index < len(s) && s[index] == '=' {
		level++
		index++
	}

	if index >= len(s) || s[index] != '[' {
		return "", false // not a long bracket, e.g. a table literal
	}

	index++

	// Lua skips a line break immediately after the opening bracket.
	if index < len(s) {
		switch s[index] {
		case '\r':
			index++
			if index < len(s) && s[index] == '\n' {
				index++
			}
		case '\n':
			index++
			if index < len(s) && s[index] == '\r' {
				index++
			}
		}
	}

	// Find the matching closer: ']' + level*'=' + ']'.
	for j := index; ; {
		k := strings.IndexByte(s[j:], ']')
		if k == -1 {
			return s[index:], true // unterminated
		}

		k += j

		if k+level+1 < len(s) {
			match := s[k+1+level] == ']'

			for m := 0; match && m < level; m++ {
				match = s[k+1+m] == '='
			}

			if match {
				return s[index:k], true
			}
		}

		j = k + 1
	}
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}

	return -1
}

func isLuaSpace(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	}

	return false
}
