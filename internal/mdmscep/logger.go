package mdmscep

import (
	gklog "github.com/go-kit/log"
	"github.com/micromdm/nanolib/log"
)

// goKitLogger adapts nanolib/log.Logger to go-kit/log.Logger.
type goKitLogger struct {
	logger log.Logger
}

// newGoKitLogger creates a go-kit compatible logger from a nanolib logger.
func newGoKitLogger(l log.Logger) gklog.Logger {
	if l == nil {
		l = log.NopLogger
	}
	return &goKitLogger{logger: l}
}

// Log implements go-kit/log.Logger interface.
func (g *goKitLogger) Log(keyvals ...interface{}) error {
	g.logger.Info(keyvals...)
	return nil
}
