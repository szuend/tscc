package compile

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/szuend/tscc/internal/config"
)

type rawSourceMap struct {
	Version        int       `json:"version"`
	File           string    `json:"file"`
	SourceRoot     string    `json:"sourceRoot,omitempty"`
	Sources        []string  `json:"sources"`
	Names          []string  `json:"names"`
	Mappings       string    `json:"mappings"`
	SourcesContent []*string `json:"sourcesContent,omitempty"`
}

// sourceMappingURLFor computes the source map URL to embed in the .js file.
func sourceMappingURLFor(cfg *config.Config) string {
	return filepath.Base(cfg.OutMapPath)
}

// rewriteSourceMappingURL replaces the existing source map URL in the emitted
// JS text with the specified newURL. It splices newURL starting at urlPos.
func rewriteSourceMappingURL(text string, urlPos int, newURL string) string {
	if urlPos < 0 || urlPos > len(text) {
		return text
	}

	endPos := strings.IndexAny(text[urlPos:], "\r\n")
	if endPos == -1 {
		endPos = len(text)
	} else {
		endPos += urlPos
	}

	return text[:urlPos] + "//# sourceMappingURL=" + newURL + text[endPos:]
}

// rewriteMapJSON parses the source map JSON and ensures it conforms to our output rules:
// 1. "file" matches the specified new JS basename.
// 2. "sourceRoot" is completely unset.
// 3. "sources" contains absolute paths (resolving any relative paths against the compiler's map file path).
func rewriteMapJSON(mapJSON, compilerMapPath, newJSName string) string {
	var m rawSourceMap
	if err := json.Unmarshal([]byte(mapJSON), &m); err != nil {
		return mapJSON
	}

	if newJSName != "" {
		m.File = newJSName
	}
	m.SourceRoot = ""

	mapDir := filepath.Dir(compilerMapPath)
	for i, s := range m.Sources {
		if !filepath.IsAbs(s) {
			m.Sources[i] = filepath.Clean(filepath.Join(mapDir, s))
		}
	}

	b, err := json.Marshal(m)
	if err != nil {
		return mapJSON
	}
	return string(b)
}
