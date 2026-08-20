package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/profiles"
	"github.com/dwdmsh/school-mdm/internal/store"
)

const customProfileMetaCols = `
	id, name, description, filename, payload_identifier, payload_uuid,
	payload_display_name, payload_type, octet_length(payload),
	(SELECT count(*) FROM custom_profile_assignments a WHERE a.profile_id = custom_profiles.id),
	created_at, updated_at
`

func (s *Store) ListCustomProfiles(ctx context.Context) ([]store.CustomProfile, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+customProfileMetaCols+`
		FROM custom_profiles
		ORDER BY created_at DESC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]store.CustomProfile, 0)
	for rows.Next() {
		p, err := scanCustomProfileMeta(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetCustomProfile(ctx context.Context, id string) (store.CustomProfile, error) {
	id = strings.TrimSpace(id)
	row := s.pool.QueryRow(ctx, `
		SELECT `+customProfileMetaCols+`
		FROM custom_profiles WHERE id=$1`, id)
	p, err := scanCustomProfileMeta(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.CustomProfile{}, store.ErrNotFound
		}
		return store.CustomProfile{}, err
	}
	return p, nil
}

func (s *Store) GetCustomProfilePayload(ctx context.Context, id string) ([]byte, error) {
	id = strings.TrimSpace(id)
	var payload []byte
	err := s.pool.QueryRow(ctx, `SELECT payload FROM custom_profiles WHERE id=$1`, id).Scan(&payload)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	return payload, nil
}

func (s *Store) CreateCustomProfile(ctx context.Context, p store.CustomProfile) (store.CustomProfile, error) {
	parsed, err := profiles.ParseMobileconfig(p.Payload)
	if err != nil {
		return store.CustomProfile{}, err
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		p.Name = parsed.DisplayName
	}
	if p.Name == "" {
		p.Name = parsed.Identifier
	}
	if p.Name == "" {
		return store.CustomProfile{}, fmt.Errorf("name is required")
	}
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	p.Description = strings.TrimSpace(p.Description)
	p.Filename = strings.TrimSpace(p.Filename)
	p.PayloadIdentifier = parsed.Identifier
	p.PayloadUUID = parsed.UUID
	p.PayloadDisplayName = parsed.DisplayName
	p.PayloadType = parsed.PayloadType
	if p.PayloadType == "" {
		p.PayloadType = "Configuration"
	}
	err = s.pool.QueryRow(ctx, `
		INSERT INTO custom_profiles (
			id, name, description, filename, payload_identifier, payload_uuid,
			payload_display_name, payload_type, payload
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING created_at, updated_at`,
		p.ID, p.Name, p.Description, p.Filename, p.PayloadIdentifier, p.PayloadUUID,
		p.PayloadDisplayName, p.PayloadType, p.Payload,
	).Scan(&p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return store.CustomProfile{}, fmt.Errorf("a profile with identifier %s already exists", p.PayloadIdentifier)
		}
		return store.CustomProfile{}, err
	}
	p.SizeBytes = len(p.Payload)
	p.Payload = nil
	p.AssignmentCount = 0
	return p, nil
}

func (s *Store) UpdateCustomProfile(ctx context.Context, p store.CustomProfile) (store.CustomProfile, error) {
	p.ID = strings.TrimSpace(p.ID)
	p.Name = strings.TrimSpace(p.Name)
	if p.ID == "" {
		return store.CustomProfile{}, fmt.Errorf("id is required")
	}
	if p.Name == "" {
		return store.CustomProfile{}, fmt.Errorf("name is required")
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE custom_profiles
		SET name=$2, description=$3, updated_at=now()
		WHERE id=$1`, p.ID, p.Name, strings.TrimSpace(p.Description))
	if err != nil {
		return store.CustomProfile{}, err
	}
	if tag.RowsAffected() == 0 {
		return store.CustomProfile{}, store.ErrNotFound
	}
	return s.GetCustomProfile(ctx, p.ID)
}

func (s *Store) ReplaceCustomProfilePayload(ctx context.Context, id string, payload []byte, filename string) (store.CustomProfile, error) {
	id = strings.TrimSpace(id)
	parsed, err := profiles.ParseMobileconfig(payload)
	if err != nil {
		return store.CustomProfile{}, err
	}
	payloadType := parsed.PayloadType
	if payloadType == "" {
		payloadType = "Configuration"
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE custom_profiles
		SET filename=$2, payload_identifier=$3, payload_uuid=$4, payload_display_name=$5,
		    payload_type=$6, payload=$7, updated_at=now(),
		    name = CASE WHEN trim(name) = '' THEN $8 ELSE name END
		WHERE id=$1`,
		id, strings.TrimSpace(filename), parsed.Identifier, parsed.UUID, parsed.DisplayName,
		payloadType, payload, parsed.DisplayName,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return store.CustomProfile{}, fmt.Errorf("a profile with identifier %s already exists", parsed.Identifier)
		}
		return store.CustomProfile{}, err
	}
	if tag.RowsAffected() == 0 {
		return store.CustomProfile{}, store.ErrNotFound
	}
	return s.GetCustomProfile(ctx, id)
}

func (s *Store) DeleteCustomProfile(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM custom_profiles WHERE id=$1`, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *Store) ListCustomProfileAssignments(ctx context.Context, profileID string) ([]store.CustomProfileAssignment, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT profile_id, target_type, target_id
		FROM custom_profile_assignments
		WHERE profile_id=$1
		ORDER BY target_type, target_id`, strings.TrimSpace(profileID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.CustomProfileAssignment
	for rows.Next() {
		var a store.CustomProfileAssignment
		var tt string
		if err := rows.Scan(&a.ProfileID, &tt, &a.TargetID); err != nil {
			return nil, err
		}
		a.TargetType = policy.TargetType(tt)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) SetCustomProfileAssignment(ctx context.Context, a store.CustomProfileAssignment) error {
	t := policy.Target{Type: a.TargetType, ID: a.TargetID}
	normalizeTarget(&t)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO custom_profile_assignments (profile_id, target_type, target_id)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`, a.ProfileID, string(t.Type), t.ID)
	return err
}

func (s *Store) RemoveCustomProfileAssignment(ctx context.Context, profileID string, target policy.Target) error {
	normalizeTarget(&target)
	_, err := s.pool.Exec(ctx, `
		DELETE FROM custom_profile_assignments
		WHERE profile_id=$1 AND target_type=$2 AND target_id=$3`,
		strings.TrimSpace(profileID), string(target.Type), target.ID)
	return err
}

func (s *Store) ListCustomProfilesForDevice(ctx context.Context, enrollmentID string, groupIDs []string) ([]store.CustomProfile, error) {
	if groupIDs == nil {
		groupIDs = []string{}
	}
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (p.id)
			p.id, p.name, p.description, p.filename, p.payload_identifier, p.payload_uuid,
			p.payload_display_name, p.payload_type, octet_length(p.payload), p.payload,
			p.created_at, p.updated_at
		FROM custom_profiles p
		INNER JOIN custom_profile_assignments a ON a.profile_id = p.id
		WHERE a.target_type = 'global'
		   OR (a.target_type = 'device' AND a.target_id = $1)
		   OR (a.target_type = 'group' AND a.target_id = ANY($2::text[]))
		ORDER BY p.id, p.name`, strings.TrimSpace(enrollmentID), groupIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.CustomProfile
	for rows.Next() {
		var p store.CustomProfile
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Description, &p.Filename, &p.PayloadIdentifier, &p.PayloadUUID,
			&p.PayloadDisplayName, &p.PayloadType, &p.SizeBytes, &p.Payload,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func scanCustomProfileMeta(row scannable) (store.CustomProfile, error) {
	var p store.CustomProfile
	err := row.Scan(
		&p.ID, &p.Name, &p.Description, &p.Filename, &p.PayloadIdentifier, &p.PayloadUUID,
		&p.PayloadDisplayName, &p.PayloadType, &p.SizeBytes, &p.AssignmentCount,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return store.CustomProfile{}, err
	}
	return p, nil
}

func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}
