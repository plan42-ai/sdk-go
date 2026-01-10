package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/plan42-ai/sdk-go/p42"
)

type GithubMigrateTenantCredsOptions struct {
	TenantID *string `help:"Only migrate GitHub credentials for the specified tenant." name:"tenant-id" short:"i" optional:""`
}

func (o *GithubMigrateTenantCredsOptions) Run(ctx context.Context, s *SharedOptions) error {
	if s.DelegatedAuthType != nil || s.DelegatedToken != nil {
		return fmt.Errorf(delegatedAuthNotSupported, "github migrate-tenant-creds")
	}
	if err := ensureNoFeatureFlags(s, "github migrate-tenant-creds"); err != nil {
		return err
	}

	tenantIDs, err := o.resolveTenantIDs(ctx, s)
	if err != nil {
		return err
	}
	if len(tenantIDs) == 0 {
		fmt.Println("No tenants to migrate.")
		return nil
	}

	for _, tenantID := range tenantIDs {
		if err := o.migrateTenant(ctx, s, tenantID); err != nil {
			return err
		}
	}
	return nil
}

func (o *GithubMigrateTenantCredsOptions) resolveTenantIDs(ctx context.Context, s *SharedOptions) ([]string, error) {
	if o.TenantID != nil {
		return []string{*o.TenantID}, nil
	}
	var ids []string
	var token *string

	for {
		resp, err := s.Client.ListTenants(ctx, &p42.ListTenantsRequest{Token: token})
		if err != nil {
			return nil, err
		}
		for _, tenant := range resp.Items {
			if tenant == nil || tenant.Deleted {
				continue
			}
			ids = append(ids, tenant.TenantID)
		}
		if resp.NextToken == nil {
			break
		}
		token = resp.NextToken
	}

	return ids, nil
}

func (o *GithubMigrateTenantCredsOptions) migrateTenant(ctx context.Context, s *SharedOptions, tenantID string) error {
	fmt.Printf("Tenant %s: starting migration.\n", tenantID)

	tenant, err := o.fetchTenant(ctx, s, tenantID)
	if err != nil {
		return err
	}
	if tenant.Deleted {
		fmt.Printf("Tenant %s: deleted tenant skipped.\n", tenantID)
		return nil
	}

	creds, err := o.fetchTenantGithubCreds(ctx, s, tenantID)
	if err != nil {
		if is404(err) {
			fmt.Printf("Tenant %s: no GitHub credentials found, skipping.\n", tenantID)
			return nil
		}
		return err
	}

	if creds.GithubUserLogin == nil || creds.GithubUserID == nil {
		fmt.Printf("Tenant %s: missing GitHub login or user ID, skipping.\n", tenantID)
		return nil
	}
	if creds.OAuthToken == nil && creds.RefreshToken == nil {
		fmt.Printf("Tenant %s: no OAuth or refresh token, skipping.\n", tenantID)
		return nil
	}

	login := *creds.GithubUserLogin
	userID := *creds.GithubUserID

	connection, err := o.ensureGithubConnection(ctx, s, tenantID, login, userID)
	if err != nil {
		return err
	}

	connection, err = o.updateGithubConnection(ctx, s, tenantID, connection, creds)
	if err != nil {
		return err
	}

	if err := o.ensureDefaultGithubConnection(ctx, s, tenant, connection); err != nil {
		return err
	}

	fmt.Printf("Tenant %s: migration complete.\n", tenantID)
	return nil
}

func (o *GithubMigrateTenantCredsOptions) fetchTenant(ctx context.Context, s *SharedOptions, tenantID string) (*p42.Tenant, error) {
	req := &p42.GetTenantRequest{TenantID: tenantID}
	if err := loadFeatureFlags(s, &req.FeatureFlags); err != nil {
		return nil, err
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)
	return s.Client.GetTenant(ctx, req)
}

func (o *GithubMigrateTenantCredsOptions) fetchTenantGithubCreds(ctx context.Context, s *SharedOptions, tenantID string) (*p42.TenantGithubCreds, error) {
	req := &p42.GetTenantGithubCredsRequest{TenantID: tenantID}
	if err := loadFeatureFlags(s, &req.FeatureFlags); err != nil {
		return nil, err
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)
	return s.Client.GetTenantGithubCreds(ctx, req)
}

func (o *GithubMigrateTenantCredsOptions) ensureGithubConnection(
	ctx context.Context,
	s *SharedOptions,
	tenantID string,
	login string,
	userID int,
) (*p42.GithubConnection, error) {
	existing, err := o.findExistingGithubConnection(ctx, s, tenantID, login, userID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		fmt.Printf("Tenant %s: reusing connection %s.\n", tenantID, existing.ConnectionID)
		return existing, nil
	}

	req := &p42.CreateGithubConnectionRequest{
		TenantID:        tenantID,
		ConnectionID:    uuid.NewString(),
		Private:         false,
		GithubUserLogin: &login,
		GithubUserID:    &userID,
	}
	if err := loadFeatureFlags(s, &req.FeatureFlags); err != nil {
		return nil, err
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	conn, err := s.Client.CreateGithubConnection(ctx, req)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Tenant %s: created connection %s.\n", tenantID, conn.ConnectionID)
	return conn, nil
}

func (o *GithubMigrateTenantCredsOptions) findExistingGithubConnection(
	ctx context.Context,
	s *SharedOptions,
	tenantID string,
	login string,
	userID int,
) (*p42.GithubConnection, error) {
	private := false
	var token *string
	for {
		req := &p42.ListGithubConnectionsRequest{
			TenantID: tenantID,
			Token:    token,
			Private:  &private,
		}
		if err := loadFeatureFlags(s, &req.FeatureFlags); err != nil {
			return nil, err
		}
		processDelegatedAuth(s, &req.DelegatedAuthInfo)

		resp, err := s.Client.ListGithubConnections(ctx, req)
		if err != nil {
			return nil, err
		}
		for _, connection := range resp.Items {
			if connection == nil {
				continue
			}
			if connection.GithubUserID != nil && *connection.GithubUserID == userID {
				return connection, nil
			}
			if connection.GithubUserLogin != nil && strings.EqualFold(*connection.GithubUserLogin, login) {
				return connection, nil
			}
		}
		if resp.NextToken == nil {
			break
		}
		token = resp.NextToken
	}
	return nil, nil
}

func (o *GithubMigrateTenantCredsOptions) updateGithubConnection(
	ctx context.Context,
	s *SharedOptions,
	tenantID string,
	connection *p42.GithubConnection,
	creds *p42.TenantGithubCreds,
) (*p42.GithubConnection, error) {
	req := &p42.UpdateGithubConnectionRequest{
		TenantID:     tenantID,
		ConnectionID: connection.ConnectionID,
		Version:      connection.Version,
	}
	var needsUpdate bool
	if creds.OAuthToken != nil {
		req.OAuthToken = creds.OAuthToken
		needsUpdate = true
	}
	if creds.RefreshToken != nil {
		req.RefreshToken = creds.RefreshToken
		needsUpdate = true
	}
	if creds.GithubUserLogin != nil {
		if connection.GithubUserLogin == nil || !strings.EqualFold(*connection.GithubUserLogin, *creds.GithubUserLogin) {
			req.GithubUserLogin = creds.GithubUserLogin
			needsUpdate = true
		}
	}
	if creds.GithubUserID != nil {
		if connection.GithubUserID == nil || *connection.GithubUserID != *creds.GithubUserID {
			req.GithubUserID = creds.GithubUserID
			needsUpdate = true
		}
	}
	if !needsUpdate {
		fmt.Printf("Tenant %s: connection %s already up to date.\n", tenantID, connection.ConnectionID)
		return connection, nil
	}

	if err := loadFeatureFlags(s, &req.FeatureFlags); err != nil {
		return nil, err
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	updated, err := s.Client.UpdateGithubConnection(ctx, req)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Tenant %s: updated connection %s.\n", tenantID, connection.ConnectionID)
	return updated, nil
}

func (o *GithubMigrateTenantCredsOptions) ensureDefaultGithubConnection(
	ctx context.Context,
	s *SharedOptions,
	tenant *p42.Tenant,
	connection *p42.GithubConnection,
) error {
	if tenant.DefaultGithubConnectionID != nil && *tenant.DefaultGithubConnectionID == connection.ConnectionID {
		fmt.Printf(
			"Tenant %s: default GitHub connection already set to %s.\n",
			tenant.TenantID,
			connection.ConnectionID,
		)
		return nil
	}
	connID := connection.ConnectionID
	req := &p42.UpdateTenantRequest{
		TenantID:                  tenant.TenantID,
		Version:                   tenant.Version,
		DefaultGithubConnectionID: &connID,
	}
	if err := loadFeatureFlags(s, &req.FeatureFlags); err != nil {
		return err
	}
	processDelegatedAuth(s, &req.DelegatedAuthInfo)

	_, err := s.Client.UpdateTenant(ctx, req)
	if err != nil {
		return err
	}
	fmt.Printf(
		"Tenant %s: set default GitHub connection to %s.\n",
		tenant.TenantID,
		connection.ConnectionID,
	)
	return nil
}
