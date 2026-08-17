package mdmhub

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

var protocolLogOnce sync.Once
var protocolLogPath string

func protocolLogFile() string {
	protocolLogOnce.Do(func() {
		dir := filepath.Join("bin", "keepalive-logs")
		_ = os.MkdirAll(dir, 0o755)
		protocolLogPath = filepath.Join(dir, "protocol.log")
	})
	return protocolLogPath
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func logProtocol(name string, next http.Handler) http.Handler {
	if next == nil {
		return nil
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		slog.Info("mdm protocol",
			"handler", name,
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"status", sw.status,
			"ua", r.UserAgent(),
			"dur", time.Since(start),
		)
		line := time.Now().Format(time.RFC3339) + " " + name + " " + r.Method + " " + r.URL.RequestURI() +
			" status=" + strconv.Itoa(sw.status) + " ua=" + r.UserAgent() + "\n"
		_ = appendFile(protocolLogFile(), []byte(line))
	})
}

func rewriteSCEPPath(next http.Handler) http.Handler {
	if next == nil {
		return nil
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/scep" {
			clone := r.Clone(r.Context())
			clone.URL.Path = "/scep"
			r = clone
		}
		next.ServeHTTP(w, r)
	})
}

func appendFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	_ = f.Close()
	return err
}
