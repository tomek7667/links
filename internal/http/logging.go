package http

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

func newRequestLogger(ignoredPaths ...string) func(next http.Handler) http.Handler {
	ignored := make(map[string]struct{}, len(ignoredPaths))
	for _, p := range ignoredPaths {
		ignored[p] = struct{}{}
	}

	base := &middleware.DefaultLogFormatter{
		Logger:  log.New(os.Stdout, "", log.LstdFlags),
		NoColor: false,
	}

	return middleware.RequestLogger(&selectiveLogFormatter{
		ignoredPaths: ignored,
		base:         base,
	})
}

type selectiveLogFormatter struct {
	ignoredPaths map[string]struct{}
	base         middleware.LogFormatter
}

func (f *selectiveLogFormatter) NewLogEntry(r *http.Request) middleware.LogEntry {
	if _, ok := f.ignoredPaths[r.URL.Path]; ok {
		return noopLogEntry{}
	}
	return f.base.NewLogEntry(r)
}

type noopLogEntry struct{}

func (noopLogEntry) Write(status, bytes int, header http.Header, elapsed time.Duration, extra interface{}) {
}

func (noopLogEntry) Panic(v interface{}, stack []byte) {}
