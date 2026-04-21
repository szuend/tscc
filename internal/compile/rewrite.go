package compile

import (
	"path/filepath"
	"strings"

	"github.com/szuend/tscc/internal/config"
)

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
