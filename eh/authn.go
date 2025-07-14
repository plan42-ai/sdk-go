package eh

import (
	"net/url"
	"time"

	"github.com/google/uuid"
)

type AuthenticationProvider struct {
	TenantID   string
	ProviderID uuid.UUID
	Name       string
	Issuer     *url.URL
	Audience   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Version    int
}
