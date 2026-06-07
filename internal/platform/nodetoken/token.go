package nodetoken

import (
	"time"

	"github.com/hasirciogluhq/atellar/internal/platform/pgutil"
)

const (
	DefaultTTL  = 90 * 24 * time.Hour
	RenewBefore = 7 * 24 * time.Hour
)

type IssuedToken struct {
	PlainToken string
	ExpiresAt  time.Time
}

func Issue() (*IssuedToken, error) {
	plainToken, err := pgutil.GenerateRandomHex(32)
	if err != nil {
		return nil, err
	}

	return &IssuedToken{
		PlainToken: plainToken,
		ExpiresAt:  time.Now().Add(DefaultTTL),
	}, nil
}

func ShouldRenew(expiresAt time.Time) bool {
	return time.Until(expiresAt) <= RenewBefore
}
