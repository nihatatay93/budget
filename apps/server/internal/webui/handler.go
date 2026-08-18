package webui

import (
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

func NewHandler() (http.Handler, error) {
	content, err := assets()
	if err != nil {
		return nil, fmt.Errorf("open embedded web assets: %w", err)
	}
	files := http.FileServer(http.FS(content))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if requested == "." || requested == "" {
			requested = "index.html"
		}
		if _, err := fs.Stat(content, requested); err != nil {
			clone := r.Clone(r.Context())
			clone.URL.Path = "/"
			files.ServeHTTP(w, clone)
			return
		}
		files.ServeHTTP(w, r)
	}), nil
}
