package siteimport

import (
	"errors"
	"strconv"
	"strings"
)

// A minimal parser for the YAML cPanel writes (YAML::Syck) in its userdata
// files: block mappings and sequences by indentation, "- key: value" items,
// plain, single- and double-quoted scalars, and the flow forms {} and []
// (also with plain items). Anchors, tags and multi-document streams are
// not needed and not supported. The result is made of map[string]any,
// []any and string.

const (
	maxYAMLLines = 20000
	maxYAMLDepth = 32
)

type yline struct {
	indent int
	text   string
}

type yparser struct {
	lines []yline
	depth int
}

var errYAML = errors.New("not a YAML file Nefo can read")

func parseYAML(b []byte) (any, error) {
	p := &yparser{}
	for _, raw := range strings.Split(string(b), "\n") {
		raw = strings.TrimRight(raw, "\r \t")
		t := strings.TrimLeft(raw, " ")
		if t == "" || strings.HasPrefix(t, "#") || t == "---" || t == "..." || strings.HasPrefix(t, "--- ") || strings.HasPrefix(t, "%") {
			continue
		}
		if strings.HasPrefix(t, "\t") {
			return nil, errYAML // tabs are not YAML indentation
		}
		p.lines = append(p.lines, yline{indent: len(raw) - len(t), text: t})
		if len(p.lines) > maxYAMLLines {
			return nil, errYAML
		}
	}
	if len(p.lines) == 0 {
		return map[string]any{}, nil
	}
	v, next, err := p.block(0, p.lines[0].indent)
	if err != nil {
		return nil, err
	}
	if next < len(p.lines) {
		return nil, errYAML
	}
	return v, nil
}

func isSeqItem(t string) bool { return t == "-" || strings.HasPrefix(t, "- ") }

// block parses the node that starts at line i, indented by ind.
func (p *yparser) block(i, ind int) (any, int, error) {
	if p.depth++; p.depth > maxYAMLDepth {
		return nil, 0, errYAML
	}
	defer func() { p.depth-- }()
	if isSeqItem(p.lines[i].text) {
		return p.seq(i, ind)
	}
	return p.mapping(i, ind)
}

func (p *yparser) seq(i, ind int) (any, int, error) {
	out := []any{}
	for i < len(p.lines) && p.lines[i].indent == ind && isSeqItem(p.lines[i].text) {
		rest := strings.TrimSpace(strings.TrimPrefix(p.lines[i].text, "-"))
		switch {
		case rest == "":
			// The item is the block on the next lines, indented further.
			if i+1 < len(p.lines) && p.lines[i+1].indent > ind {
				v, next, err := p.block(i+1, p.lines[i+1].indent)
				if err != nil {
					return nil, 0, err
				}
				out, i = append(out, v), next
			} else {
				out, i = append(out, ""), i+1
			}
		case isSeqItem(rest) || hasKey(rest):
			// "- key: value": a mapping whose first line starts after "- ".
			col := ind + len(p.lines[i].text) - len(rest)
			p.lines[i] = yline{indent: col, text: rest}
			v, next, err := p.block(i, col)
			if err != nil {
				return nil, 0, err
			}
			out, i = append(out, v), next
		default:
			v, err := scalar(rest)
			if err != nil {
				return nil, 0, err
			}
			out, i = append(out, v), i+1
		}
	}
	return out, i, nil
}

func (p *yparser) mapping(i, ind int) (any, int, error) {
	m := map[string]any{}
	for i < len(p.lines) && p.lines[i].indent == ind {
		t := p.lines[i].text
		if isSeqItem(t) {
			break
		}
		key, val, ok := splitKey(t)
		if !ok {
			return nil, 0, errYAML
		}
		i++
		switch {
		case val == "" || strings.HasPrefix(val, "#"):
			// The value is the block below: deeper, or a sequence at the
			// same indentation ("key:\n- a").
			if i < len(p.lines) && (p.lines[i].indent > ind || (p.lines[i].indent == ind && isSeqItem(p.lines[i].text))) {
				v, next, err := p.block(i, p.lines[i].indent)
				if err != nil {
					return nil, 0, err
				}
				m[key], i = v, next
			} else {
				m[key] = ""
			}
		case val == "|" || val == ">" || strings.HasPrefix(val, "|") || strings.HasPrefix(val, ">"):
			var parts []string
			for i < len(p.lines) && p.lines[i].indent > ind {
				parts = append(parts, p.lines[i].text)
				i++
			}
			sep := "\n"
			if val[0] == '>' {
				sep = " "
			}
			m[key] = strings.Join(parts, sep)
		default:
			v, err := scalar(val)
			if err != nil {
				return nil, 0, err
			}
			m[key] = v
		}
	}
	return m, i, nil
}

// hasKey tells whether a line is "key: value" or "key:".
func hasKey(t string) bool {
	_, _, ok := splitKey(t)
	return ok
}

// splitKey splits "key: value" (the key may be quoted).
func splitKey(t string) (key, val string, ok bool) {
	if t == "" {
		return "", "", false
	}
	if t[0] == '\'' || t[0] == '"' {
		k, rest, err := quoted(t)
		if err != nil {
			return "", "", false
		}
		rest = strings.TrimLeft(rest, " ")
		if !strings.HasPrefix(rest, ":") {
			return "", "", false
		}
		return k, strings.TrimSpace(rest[1:]), true
	}
	if t[0] == '[' || t[0] == '{' {
		return "", "", false
	}
	if i := strings.Index(t, ": "); i > 0 {
		return strings.TrimSpace(t[:i]), strings.TrimSpace(t[i+2:]), true
	}
	if strings.HasSuffix(t, ":") && len(t) > 1 {
		return strings.TrimSpace(t[:len(t)-1]), "", true
	}
	return "", "", false
}

// scalar parses a value: quoted, a flow collection or a plain scalar.
func scalar(s string) (any, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", nil
	case s[0] == '\'' || s[0] == '"':
		v, rest, err := quoted(s)
		if err != nil {
			return nil, err
		}
		if r := strings.TrimSpace(rest); r != "" && !strings.HasPrefix(r, "#") {
			return nil, errYAML
		}
		return v, nil
	case s[0] == '[':
		return flow(s, '[', ']')
	case s[0] == '{':
		return flow(s, '{', '}')
	}
	if i := strings.Index(s, " #"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	if s == "~" || s == "null" {
		return "", nil
	}
	return s, nil
}

// flow parses [a, b] or {a: b} with plain or quoted items (no nesting).
func flow(s string, open, close byte) (any, error) {
	if !strings.HasSuffix(s, string(close)) {
		return nil, errYAML
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	var items []string
	if inner != "" {
		if strings.ContainsAny(inner, "[]{}") {
			return nil, errYAML
		}
		items = splitFlow(inner)
	}
	if open == '[' {
		out := []any{}
		for _, it := range items {
			v, err := scalar(it)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	}
	out := map[string]any{}
	for _, it := range items {
		k, v, ok := splitKey(strings.TrimSpace(it))
		if !ok {
			return nil, errYAML
		}
		sv, err := scalar(v)
		if err != nil {
			return nil, err
		}
		out[k] = sv
	}
	return out, nil
}

// splitFlow splits on commas outside quotes.
func splitFlow(s string) []string {
	var out []string
	var q byte
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case q != 0:
			if c == q {
				q = 0
			}
		case c == '\'' || c == '"':
			q = c
		case c == ',':
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

// quoted reads a quoted scalar at the start of s and returns it with the
// rest of s.
func quoted(s string) (string, string, error) {
	q := s[0]
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		c := s[i]
		switch {
		case q == '\'' && c == '\'':
			if i+1 < len(s) && s[i+1] == '\'' {
				b.WriteByte('\'')
				i++
				continue
			}
			return b.String(), s[i+1:], nil
		case q == '"' && c == '"':
			return b.String(), s[i+1:], nil
		case q == '"' && c == '\\' && i+1 < len(s):
			i++
			switch e := s[i]; e {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'x':
				if i+2 < len(s) {
					if v, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil {
						b.WriteByte(byte(v))
						i += 2
						continue
					}
				}
				return "", "", errYAML
			default:
				b.WriteByte(e) // \" \\ \/ and the rest as themselves
			}
		default:
			b.WriteByte(c)
		}
	}
	return "", "", errYAML
}

// ---- reading values ----

func yMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func yString(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return strings.TrimSpace(s)
}

// yStrings returns a list value; a mapping gives its keys, a string its
// space-separated words.
func yStrings(m map[string]any, key string) []string {
	var out []string
	switch v := m[key].(type) {
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
	case map[string]any:
		for k := range v {
			out = append(out, k)
		}
	case string:
		out = strings.Fields(v)
	}
	return out
}
