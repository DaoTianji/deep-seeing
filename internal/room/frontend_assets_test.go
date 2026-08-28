package room

import (
	"io/fs"
	"strings"
	"testing"
)

func builtFrontendAssets(t *testing.T) (string, string) {
	t.Helper()
	entries, err := fs.ReadDir(webFS, "web/dist/assets")
	if err != nil {
		t.Fatal(err)
	}
	var js, css strings.Builder
	for _, entry := range entries {
		raw, readErr := webFS.ReadFile("web/dist/assets/" + entry.Name())
		if readErr != nil {
			t.Fatal(readErr)
		}
		if strings.HasSuffix(entry.Name(), ".js") {
			js.Write(raw)
		}
		if strings.HasSuffix(entry.Name(), ".css") {
			css.Write(raw)
		}
	}
	return js.String(), css.String()
}
