package compile

import (
	"testing"

	"github.com/szuend/tscc/internal/config"
)

func TestSourceMappingURLFor(t *testing.T) {
	cfg := &config.Config{OutMapPath: "/foo/bar/baz.js.map"}
	if got, want := sourceMappingURLFor(cfg), "baz.js.map"; got != want {
		t.Errorf("sourceMappingURLFor() = %q, want %q", got, want)
	}
}

func TestRewriteSourceMappingURL(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		pos    int
		newURL string
		want   string
	}{
		{
			name:   "basic rewrite",
			text:   "console.log();\n//# sourceMappingURL=old.js.map\n",
			pos:    15,
			newURL: "new.js.map",
			want:   "console.log();\n//# sourceMappingURL=new.js.map\n",
		},
		{
			name:   "rewrite at end of string without newline",
			text:   "//# sourceMappingURL=old.js.map",
			pos:    0,
			newURL: "new.js.map",
			want:   "//# sourceMappingURL=new.js.map",
		},
		{
			name:   "invalid pos (negative)",
			text:   "text",
			pos:    -1,
			newURL: "new",
			want:   "text",
		},
		{
			name:   "invalid pos (out of bounds)",
			text:   "text",
			pos:    10,
			newURL: "new",
			want:   "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rewriteSourceMappingURL(tt.text, tt.pos, tt.newURL); got != tt.want {
				t.Errorf("rewriteSourceMappingURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRewriteMapJSON(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		compilerMap string
		newJS       string
		want        string
	}{
		{
			name:        "basic rewrite file, empty sourceRoot, absolute sources",
			json:        `{"version":3,"file":"old.js","sourceRoot":"","sources":["../a.ts"],"names":[],"mappings":""}`,
			compilerMap: "/tmp/map-test/maps/old.js.map",
			newJS:       "new.js",
			want:        `{"version":3,"file":"new.js","sources":["/tmp/map-test/a.ts"],"names":[],"mappings":""}`,
		},
		{
			name:        "no JS name change, only sourceRoot and sources",
			json:        `{"version":3,"file":"old.js","sourceRoot":"some/root","sources":["a.ts"],"names":[],"mappings":""}`,
			compilerMap: "/tmp/map-test/old.js.map",
			newJS:       "",
			want:        `{"version":3,"file":"old.js","sources":["/tmp/map-test/a.ts"],"names":[],"mappings":""}`,
		},
		{
			name:        "already absolute sources are left alone",
			json:        `{"version":3,"file":"old.js","sources":["/abs/path/a.ts"],"names":[],"mappings":""}`,
			compilerMap: "/tmp/map-test/old.js.map",
			newJS:       "new.js",
			want:        `{"version":3,"file":"new.js","sources":["/abs/path/a.ts"],"names":[],"mappings":""}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rewriteMapJSON(tt.json, tt.compilerMap, tt.newJS); got != tt.want {
				t.Errorf("rewriteMapJSON() =\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}
