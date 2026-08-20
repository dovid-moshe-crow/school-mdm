package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dwdmsh/school-mdm/internal/store"
)

const policyTimerCols = `
	id, name, action, COALESCE(pack_ids, '{}'), COALESCE(profile_ids, '{}'),
	COALESCE(device_ids, '{}'),
	COALESCE(group_ids, '{}'), schedule, run_at, COALESCE(weekdays, '{}'),
	time_of_day, enabled, last_run_at, last_run_key, created_at, updated_at
`

func (s *Store) ListPolicyTimers(ctx context.Context) ([]store.PolicyTimer, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+policyTimerCols+` FROM policy_timers ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]store.PolicyTimer, 0)
	for rows.Next() {
		t, err := scanPolicyTimer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) GetPolicyTimer(ctx context.Context, id string) (store.PolicyTimer, error) {
	id = strings.TrimSpace(id)
	row := s.pool.QueryRow(ctx, `SELECT `+policyTimerCols+` FROM policy_timers WHERE id=$1`, id)
	t, err := scanPolicyTimer(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.PolicyTimer{}, fmt.Errorf("timer %s: %w", id, store.ErrNotFound)
		}
		return store.PolicyTimer{}, err
	}
	return t, nil
}

func (s *Store) CreatePolicyTimer(ctx context.Context, t store.PolicyTimer) (store.PolicyTimer, error) {
	if err := store.NormalizePolicyTimer(&t); err != nil {
		return store.PolicyTimer{}, err
	}
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO policy_timers (
			id, name, action, pack_ids, profile_ids, device_ids, group_ids, schedule, run_at,
			weekdays, time_of_day, enabled, last_run_key
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING created_at, updated_at
	`, t.ID, t.Name, t.Action, t.PackIDs, t.ProfileIDs, t.DeviceIDs, t.GroupIDs, t.Schedule, t.RunAt,
		int32Slice(t.Weekdays), t.TimeOfDay, t.Enabled, t.LastRunKey,
	).Scan(&t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return store.PolicyTimer{}, err
	}
	return t, nil
}

func (s *Store) UpdatePolicyTimer(ctx context.Context, t store.PolicyTimer) (store.PolicyTimer, error) {
	t.ID = strings.TrimSpace(t.ID)
	if t.ID == "" {
		return store.PolicyTimer{}, fmt.Errorf("id is required")
	}
	if err := store.NormalizePolicyTimer(&t); err != nil {
		return store.PolicyTimer{}, err
	}
	err := s.pool.QueryRow(ctx, `
		UPDATE policy_timers
		SET name=$2, action=$3, pack_ids=$4, profile_ids=$5, device_ids=$6, group_ids=$7, schedule=$8,
		    run_at=$9, weekdays=$10, time_of_day=$11, enabled=$12,
		    last_run_at=$13, last_run_key=$14, updated_at=now()
		WHERE id=$1
		RETURNING created_at, updated_at
	`, t.ID, t.Name, t.Action, t.PackIDs, t.ProfileIDs, t.DeviceIDs, t.GroupIDs, t.Schedule, t.RunAt,
		int32Slice(t.Weekdays), t.TimeOfDay, t.Enabled, t.LastRunAt, t.LastRunKey,
	).Scan(&t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.PolicyTimer{}, fmt.Errorf("timer %s: %w", t.ID, store.ErrNotFound)
		}
		return store.PolicyTimer{}, err
	}
	return t, nil
}

func (s *Store) DeletePolicyTimer(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	ct, err := s.pool.Exec(ctx, `DELETE FROM policy_timers WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("timer %s: %w", id, store.ErrNotFound)
	}
	return nil
}

func (s *Store) TouchPolicyTimerRun(ctx context.Context, id string, at time.Time, runKey string, enabled bool) error {
	id = strings.TrimSpace(id)
	if at.IsZero() {
		at = time.Now().UTC()
	}
	ct, err := s.pool.Exec(ctx, `
		UPDATE policy_timers
		SET last_run_at=$2, last_run_key=$3, enabled=$4, updated_at=$2
		WHERE id=$1
	`, id, at, strings.TrimSpace(runKey), enabled)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("timer %s: %w", id, store.ErrNotFound)
	}
	return nil
}

func scanPolicyTimer(row scannable) (store.PolicyTimer, error) {
	var t store.PolicyTimer
	var days []int32
	err := row.Scan(
		&t.ID, &t.Name, &t.Action, &t.PackIDs, &t.ProfileIDs, &t.DeviceIDs, &t.GroupIDs,
		&t.Schedule, &t.RunAt, &days, &t.TimeOfDay, &t.Enabled,
		&t.LastRunAt, &t.LastRunKey, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return store.PolicyTimer{}, err
	}
	t.Weekdays = intSlice(days)
	if t.PackIDs == nil {
		t.PackIDs = []string{}
	}
	if t.ProfileIDs == nil {
		t.ProfileIDs = []string{}
	}
	if t.DeviceIDs == nil {
		t.DeviceIDs = []string{}
	}
	if t.GroupIDs == nil {
		t.GroupIDs = []string{}
	}
	if t.Weekdays == nil {
		t.Weekdays = []int{}
	}
	return t, nil
}

func int32Slice(in []int) []int32 {
	out := make([]int32, len(in))
	for i, v := range in {
		out[i] = int32(v)
	}
	return out
}

func intSlice(in []int32) []int {
	out := make([]int, len(in))
	for i, v := range in {
		out[i] = int(v)
	}
	return out
}
