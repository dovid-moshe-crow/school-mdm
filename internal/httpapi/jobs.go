package httpapi

import (
	"context"
	"time"
)

const backgroundJobTimeout = 3 * time.Minute

// goJob runs fn in the background so HTTP handlers can return as soon as the
// request is valid. Tests set ForegroundJobs to run the work inline.
func (a *API) goJob(name string, fn func(ctx context.Context)) {
	if a == nil || fn == nil {
		return
	}
	run := func() {
		ctx, cancel := context.WithTimeout(context.Background(), backgroundJobTimeout)
		defer cancel()
		defer func() {
			if rec := recover(); rec != nil && a.Log != nil {
				a.Log.Error("background job panic", "job", name, "panic", rec)
			}
		}()
		fn(ctx)
	}
	if a.ForegroundJobs {
		run()
		return
	}
	go run()
}

func (a *API) pushManyLater(ids []string) {
	if a == nil || a.Push == nil || len(ids) == 0 {
		return
	}
	copied := append([]string(nil), ids...)
	a.goJob("reconcile", func(ctx context.Context) {
		if err := a.Push.ReconcileMany(ctx, copied); err != nil && a.Log != nil {
			a.Log.Warn("background reconcile", "err", err, "devices", len(copied))
		}
	})
}

func (a *API) pushOneLater(id string) {
	if id == "" {
		return
	}
	a.pushManyLater([]string{id})
}
