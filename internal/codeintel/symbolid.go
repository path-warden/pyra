package codeintel

import (
	"fmt"
	"strconv"
	"strings"
)

// FormatID builds a symbol-id "<lang>:<rel>#<name>@<line>" (line is 1-based),
// matching grove's format exactly.
func FormatID(lang, rel, name string, line int) string {
	return fmt.Sprintf("%s:%s#%s@%d", lang, rel, name, line)
}

// ParseID parses a symbol-id. It splits the "<lang>:" prefix at the FIRST colon
// (so the remaining relpath may itself contain colons), then splits path from
// the rest at '#', then name from line at '@'. The line is optional. A missing
// '#' means the id is malformed and ok is false. The lang segment is discarded,
// mirroring grove (which does not validate it).
func ParseID(id string) (path, name string, line int, ok bool) {
	rest := id
	if i := strings.IndexByte(rest, ':'); i >= 0 {
		rest = rest[i+1:]
	}
	hash := strings.IndexByte(rest, '#')
	if hash < 0 {
		return "", "", 0, false
	}
	path = rest[:hash]
	after := rest[hash+1:]
	if at := strings.LastIndexByte(after, '@'); at >= 0 {
		name = after[:at]
		if n, err := strconv.Atoi(after[at+1:]); err == nil {
			line = n
		}
	} else {
		name = after
	}
	if path == "" || name == "" {
		return "", "", 0, false
	}
	return path, name, line, true
}

// ParsePos parses a "file:line:col" position (1-based input) into a path plus
// 0-based row/col. It splits on the LAST two colons so a path may contain
// colons — a deliberately different parser from ParseID (grove keeps these two
// parsers separate). ok is false if line/col are missing or non-numeric.
func ParsePos(pos string) (path string, row, col int, ok bool) {
	lastColon := strings.LastIndexByte(pos, ':')
	if lastColon < 0 {
		return "", 0, 0, false
	}
	prevColon := strings.LastIndexByte(pos[:lastColon], ':')
	if prevColon < 0 {
		return "", 0, 0, false
	}
	path = pos[:prevColon]
	lineStr := pos[prevColon+1 : lastColon]
	colStr := pos[lastColon+1:]
	lineNum, err1 := strconv.Atoi(lineStr)
	colNum, err2 := strconv.Atoi(colStr)
	if err1 != nil || err2 != nil || path == "" {
		return "", 0, 0, false
	}
	// 1-based input -> 0-based row/col (saturating at 0).
	if lineNum > 0 {
		row = lineNum - 1
	}
	if colNum > 0 {
		col = colNum - 1
	}
	return path, row, col, true
}
