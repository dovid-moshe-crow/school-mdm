package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/dwdmsh/school-mdm/internal/store"
)

// depNameSource reads the active DEP slot name from school DB settings.
type depNameSource struct {
	store store.Store
}

func (d *depNameSource) DEPName(ctx context.Context) (string, error) {
	s, err := d.store.GetMDMSettings(ctx)
	if err != nil {
		return "", err
	}
	return s.DepName, nil
}

func ensureMDMSettings(ctx context.Context, st store.Store, seedName string) error {
	_, err := st.GetMDMSettings(ctx)
	if err == nil {
		return nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	name := strings.TrimSpace(seedName)
	if name == "" {
		name = "nanok"
	}
	if err := validateDepName(name); err != nil {
		return err
	}
	_, err = st.UpsertMDMSettings(ctx, store.MDMSettings{
		DepName:            name,
		CompanionBundleID:  "com.kfilter.portal",
		CompanionEnabled:   true,
		LockScreenEnabled:  true,
		LockScreenFootnote: "מכשיר בית ספר · KFilter",
	})
	return err
}

func validateDepName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("dep_name required")
	}
	if len(name) > 64 {
		return fmt.Errorf("dep_name too long")
	}
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			continue
		}
		return fmt.Errorf("dep_name may only contain letters, digits, - and _")
	}
	return nil
}
