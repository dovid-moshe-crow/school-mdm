package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dwdmsh/school-mdm/internal/app"
)

func main() {
	loadDotEnv(".env")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := app.New(ctx)
	if err != nil {
		log.Fatalf("app: %v", err)
	}
	defer application.Close()

	if err := application.Run(ctx); err != nil {
		log.Fatalf("run: %v", err)
	}
}

// loadDotEnv is a tiny .env reader (KEY=VALUE lines) so we need no extra deps.
func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := splitLines(string(data))
	for _, line := range lines {
		if line == "" || line[0] == '#' {
			continue
		}
		key, val, ok := splitKV(line)
		if !ok {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, trimCR(s[start:i]))
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, trimCR(s[start:]))
	}
	return out
}

func trimCR(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\r' {
		return s[:len(s)-1]
	}
	return s
}

func splitKV(line string) (string, string, bool) {
	for i := 0; i < len(line); i++ {
		if line[i] == '=' {
			key := line[:i]
			val := line[i+1:]
			if len(val) >= 2 {
				if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
					val = val[1 : len(val)-1]
				}
			}
			return key, val, key != ""
		}
	}
	return "", "", false
}
