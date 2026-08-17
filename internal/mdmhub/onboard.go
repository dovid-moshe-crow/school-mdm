package mdmhub

import (
	"context"

	"github.com/micromdm/nanomdm/mdm"
	"github.com/micromdm/nanomdm/service"
)

// OnTokenUpdateFunc runs after a successful TokenUpdate (device enrolled / push token refreshed).
type OnTokenUpdateFunc func(ctx context.Context, enrollmentID string)

// tokenUpdateOnboard is a NanoMDM side-service that triggers school onboarding.
type tokenUpdateOnboard struct {
	service.NopService
	fn OnTokenUpdateFunc
}

func (o *tokenUpdateOnboard) TokenUpdate(r *mdm.Request, _ *mdm.TokenUpdate) error {
	if o == nil || o.fn == nil || r == nil || r.EnrollID == nil || r.ID == "" {
		return nil
	}
	o.fn(r.Context(), r.ID)
	return nil
}
