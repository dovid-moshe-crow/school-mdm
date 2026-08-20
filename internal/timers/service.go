package timers

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	_ "time/tzdata"

	"github.com/dwdmsh/school-mdm/internal/activity"
	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/store"
)

// Reconciler pushes effective allowlists after a timer mutates assignments.
type Reconciler interface {
	ReconcileMany(ctx context.Context, enrollmentIDs []string) error
}

// Service stores, fires, and applies whitelist timers.
type Service struct {
	Store    store.Store
	Push     Reconciler
	Activity *activity.Logger
	Loc      *time.Location
	Log      *slog.Logger

	mu sync.Mutex
}

func (s *Service) loc() *time.Location {
	if s != nil && s.Loc != nil {
		return s.Loc
	}
	return Location()
}

// View is a timer plus the computed next run.
type View struct {
	store.PolicyTimer
	NextRunAt *time.Time `json:"next_run_at,omitempty"`
}

func (s *Service) view(t store.PolicyTimer, now time.Time) View {
	return View{PolicyTimer: t, NextRunAt: NextRun(t, now, s.loc())}
}

func (s *Service) List(ctx context.Context) ([]View, error) {
	list, err := s.Store.ListPolicyTimers(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	out := make([]View, 0, len(list))
	for _, t := range list {
		out = append(out, s.view(t, now))
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, id string) (View, error) {
	t, err := s.Store.GetPolicyTimer(ctx, id)
	if err != nil {
		return View{}, err
	}
	return s.view(t, time.Now()), nil
}

func (s *Service) Create(ctx context.Context, t store.PolicyTimer) (View, error) {
	t.LastRunAt = nil
	t.LastRunKey = ""
	created, err := s.Store.CreatePolicyTimer(ctx, t)
	if err != nil {
		return View{}, err
	}
	return s.view(created, time.Now()), nil
}

func (s *Service) Update(ctx context.Context, t store.PolicyTimer, resetRun bool) (View, error) {
	existing, err := s.Store.GetPolicyTimer(ctx, t.ID)
	if err != nil {
		return View{}, err
	}
	if resetRun {
		t.LastRunAt = nil
		t.LastRunKey = ""
	} else {
		t.LastRunAt = existing.LastRunAt
		t.LastRunKey = existing.LastRunKey
	}
	updated, err := s.Store.UpdatePolicyTimer(ctx, t)
	if err != nil {
		return View{}, err
	}
	return s.view(updated, time.Now()), nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.Store.DeletePolicyTimer(ctx, id)
}

// ApplyResult summarizes one timer fire (scheduled or manual).
type ApplyResult struct {
	TimerID     string `json:"timer_id"`
	Action      string `json:"action"`
	Assignments int    `json:"assignments"`
	Errors      int    `json:"errors"`
	Devices     int    `json:"devices"`
}

func (s *Service) Apply(ctx context.Context, t store.PolicyTimer) (ApplyResult, error) {
	res := ApplyResult{TimerID: t.ID, Action: t.Action}
	seenDev := map[string]struct{}{}
	var devices []string
	addDev := func(ids []string) {
		for _, id := range ids {
			if id == "" {
				continue
			}
			if _, ok := seenDev[id]; ok {
				continue
			}
			seenDev[id] = struct{}{}
			devices = append(devices, id)
		}
	}
	addDev(t.DeviceIDs)
	for _, gid := range t.GroupIDs {
		ids, err := s.Store.ListEnrollmentIDsForGroup(ctx, gid)
		if err != nil {
			res.Errors++
			continue
		}
		addDev(ids)
	}

	for _, packID := range t.PackIDs {
		for _, gid := range t.GroupIDs {
			if err := s.applyOne(ctx, t.Action, packID, policy.Target{Type: policy.TargetGroup, ID: gid}); err != nil {
				res.Errors++
				continue
			}
			res.Assignments++
		}
		for _, did := range t.DeviceIDs {
			if err := s.applyOne(ctx, t.Action, packID, policy.Target{Type: policy.TargetDevice, ID: did}); err != nil {
				res.Errors++
				continue
			}
			res.Assignments++
		}
	}
	for _, profileID := range t.ProfileIDs {
		for _, gid := range t.GroupIDs {
			if err := s.applyProfile(ctx, t.Action, profileID, policy.Target{Type: policy.TargetGroup, ID: gid}); err != nil {
				res.Errors++
				continue
			}
			res.Assignments++
		}
		for _, did := range t.DeviceIDs {
			if err := s.applyProfile(ctx, t.Action, profileID, policy.Target{Type: policy.TargetDevice, ID: did}); err != nil {
				res.Errors++
				continue
			}
			res.Assignments++
		}
	}
	res.Devices = len(devices)
	if s.Push != nil && len(devices) > 0 {
		copied := append([]string(nil), devices...)
		push := s.Push
		go func() {
			bg, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			_ = push.ReconcileMany(bg, copied)
		}()
	}
	return res, nil
}

func (s *Service) applyOne(ctx context.Context, action, packID string, target policy.Target) error {
	switch action {
	case store.TimerActionRemove:
		return s.Store.RemoveWhitelistPackAssignment(ctx, packID, target)
	default:
		return s.Store.SetWhitelistPackAssignment(ctx, store.WhitelistPackAssignment{
			PackID: packID, TargetType: target.Type, TargetID: target.ID,
		})
	}
}

func (s *Service) applyProfile(ctx context.Context, action, profileID string, target policy.Target) error {
	switch action {
	case store.TimerActionRemove:
		return s.Store.RemoveCustomProfileAssignment(ctx, profileID, target)
	default:
		return s.Store.SetCustomProfileAssignment(ctx, store.CustomProfileAssignment{
			ProfileID: profileID, TargetType: target.Type, TargetID: target.ID,
		})
	}
}

// RunDue fires every enabled timer that is due at now.
func (s *Service) RunDue(ctx context.Context, now time.Time) (fired, errs int, err error) {
	if s == nil || s.Store == nil {
		return 0, 0, errors.New("timers unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if now.IsZero() {
		now = time.Now()
	}
	list, err := s.Store.ListPolicyTimers(ctx)
	if err != nil {
		return 0, 0, err
	}
	loc := s.loc()
	for _, t := range list {
		key, ok := Due(t, now, loc)
		if !ok {
			continue
		}
		res, applyErr := s.Apply(ctx, t)
		if applyErr != nil {
			errs++
			s.logFire(ctx, t, res, applyErr, true)
			continue
		}
		if res.Assignments == 0 && res.Errors > 0 {
			errs++
			s.logFire(ctx, t, res, errors.New("all assignments failed"), true)
			continue
		}
		enabled := t.Enabled
		if t.Schedule == store.TimerOnce {
			enabled = false
		}
		if touchErr := s.Store.TouchPolicyTimerRun(ctx, t.ID, now.UTC(), key, enabled); touchErr != nil {
			errs++
		}
		fired++
		if res.Errors > 0 {
			errs++
		}
		s.logFire(ctx, t, res, nil, false)
	}
	return fired, errs, nil
}

// RunNow applies a timer immediately without consuming the scheduled occurrence.
func (s *Service) RunNow(ctx context.Context, id string) (ApplyResult, store.PolicyTimer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, err := s.Store.GetPolicyTimer(ctx, id)
	if err != nil {
		return ApplyResult{}, store.PolicyTimer{}, err
	}
	res, err := s.Apply(ctx, t)
	if err != nil {
		return res, t, err
	}
	return res, t, nil
}

func (s *Service) logFire(ctx context.Context, t store.PolicyTimer, res ApplyResult, applyErr error, scheduledFail bool) {
	if s.Activity == nil {
		return
	}
	result := store.ActivityResultOK
	summary := "טיימר רשימה מותרת הורץ"
	if applyErr != nil || scheduledFail {
		result = store.ActivityResultError
		summary = "טיימר רשימה מותרת נכשל"
	}
	detail := map[string]any{
		"timer_id":    t.ID,
		"name":        t.Name,
		"action":      t.Action,
		"pack_ids":    t.PackIDs,
		"profile_ids": t.ProfileIDs,
		"device_ids":  t.DeviceIDs,
		"group_ids":   t.GroupIDs,
		"assignments": res.Assignments,
		"devices":     res.Devices,
		"errors":      res.Errors,
	}
	if applyErr != nil {
		detail["error"] = applyErr.Error()
	}
	s.Activity.Log(ctx, activity.Event{
		Category:  store.ActivityCategoryPolicy,
		Action:    "timer_fire",
		ActorType: store.ActivityActorSystem,
		Actor:     "timer",
		Result:    result,
		Summary:   summary,
		Detail:    detail,
	})
}

// StartTicker runs due timers immediately, then on every interval until ctx is done.
func (s *Service) StartTicker(ctx context.Context, every time.Duration, log *slog.Logger) {
	if s == nil {
		return
	}
	if every <= 0 {
		every = time.Minute
	}
	if log == nil {
		log = s.Log
	}
	run := func() {
		fired, errs, err := s.RunDue(ctx, time.Now())
		if err != nil {
			if log != nil {
				log.Warn("timers run failed", "err", err)
			}
			return
		}
		if (fired > 0 || errs > 0) && log != nil {
			log.Info("timers run", "fired", fired, "errors", errs)
		}
	}
	run()
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
