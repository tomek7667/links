package http

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
)

// The whole UI is embedded so `go install` produces a single self-contained
// binary: no build step, no CDN, works offline on a LAN.
//
//go:embed ui/index.html ui/assets
var uiFS embed.FS

var (
	indexTmpl    = template.Must(template.ParseFS(uiFS, "ui/index.html"))
	assetsFS     = mustSub(uiFS, "ui/assets")
	assetVersion = hashAssets(assetsFS)
)

func mustSub(f fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

// hashAssets fingerprints the embedded assets so browsers can cache them
// forever and still pick up a new version after an update.
func hashAssets(f fs.FS) string {
	h := sha256.New()
	_ = fs.WalkDir(f, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(f, path)
		if err != nil {
			return err
		}
		h.Write([]byte(path))
		h.Write(b)
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))[:12]
}

func assetsHandler() http.HandlerFunc {
	files := http.StripPrefix("/assets/", http.FileServer(http.FS(assetsFS)))
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("v") == assetVersion {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	}
}
