package eh

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

func createEnumMaps[T ~string](values []T) (map[T]int64, map[int64]T) {
	if len(values) > 64 {
		panic("too many enum values")
	}
	encodingMap := make(map[T]int64)
	decodingMap := make(map[int64]T)
	for i := range values {
		encodingMap[values[i]] = 1 << i
		decodingMap[1<<i] = values[i]
	}
	encodingMap[T("*")] = -1
	return encodingMap, decodingMap
}

func CreateBitVector[T ~string](values []T, enc map[T]int64) int64 {
	var ret int64
	for _, v := range values {
		ret |= enc[v]
	}
	return ret
}

func CreateArray[T ~string](bv int64, dec map[int64]T) []T {
	if bv == -1 {
		return []T{T("*")}
	}
	var ret []T
	for i := 0; i < 64; i++ {
		if (bv & (1 << i)) != 0 {
			if item, ok := dec[1<<i]; ok {
				ret = append(ret, item)
			}
		}
	}
	return ret
}

// EffectType defines whether a policy allows or denies access.
type EffectType string

const (
	EffectAllow EffectType = "Allow"
	EffectDeny  EffectType = "Deny"
)

// PrincipalType defines the type of principal a policy applies to.
type PrincipalType string

const (
	PrincipalUser           PrincipalType = "User"
	PrincipalIAMRole        PrincipalType = "IAMRole"
	PrincipalService        PrincipalType = "Service"
	PrincipalServiceAccount PrincipalType = "ServiceAccount"
	PrincipalAgent          PrincipalType = "Agent"
)

// Action defines the actions that a policy can allow or deny.
type Action string

const (
	ActionPerformDelegatedAction    Action = "PerformDelegatedAction"
	ActionCreateTenant              Action = "CreateTenant"
	ActionGetTenant                 Action = "GetTenant"
	ActionGenerateWebUIToken        Action = "GenerateWebUIToken"
	ActionListPolicies              Action = "ListPolicies"
	ActionUpdateTurn                Action = "UpdateTurn"
	ActionUpdateTask                Action = "UpdateTask"
	ActionGetTask                   Action = "GetTask"
	ActionListTasks                 Action = "ListTasks"
	ActionGetTurn                   Action = "GetTurn"
	ActionUploadTurnLogs            Action = "UploadTurnLogs"
	ActionGetCurrentUser            Action = "GetCurrentUser"
	ActionCreateEnvironment         Action = "CreateEnvironment"
	ActionGetEnvironment            Action = "GetEnvironment"
	ActionListEnvironments          Action = "ListEnvironments"
	ActionUpdateEnvironment         Action = "UpdateEnvironment"
	ActionDeleteEnvironment         Action = "DeleteEnvironment"
	ActionGetLastTurn               Action = "GetLastTurn"
	ActionCreateTask                Action = "CreateTask"
	ActionGetLastTurnLog            Action = "GetLastTurnLog"
	ActionStreamLogs                Action = "StreamLogs"
	ActionListTurns                 Action = "ListTurns"
	ActionAddGithubOrg              Action = "AddGithubOrg"
	ActionUpdateGithubOrg           Action = "UpdateGithubOrg"
	ActionDeleteGithubOrg           Action = "DeleteGithubOrg"
	ActionListGithubOrgs            Action = "ListGithubOrgs"
	ActionGetGithubOrg              Action = "GetGithubOrg"
	ActionCreateFeatureFlag         Action = "CreateFeatureFlag"
	ActionGetTenantFeatureFlags     Action = "GetTenantFeatureFlags"
	ActionCreateFeatureFlagOverride Action = "CreateFeatureFlagOverride"
	ActionListFeatureFlags          Action = "ListFeatureFlags"
	ActionGetFeatureFlag            Action = "GetFeatureFlag"
	ActionUpdateFeatureFlag         Action = "UpdateFeatureFlag"
	ActionDeleteFeatureFlag         Action = "DeleteFeatureFlag"
	ActionDeleteFeatureFlagOverride Action = "DeleteFeatureFlagOverride"
	ActionGetFeatureFlagOverride    Action = "GetFeatureFlagOverride"
	ActionUpdateFeatureFlagOverride Action = "UpdateFeatureFlagOverride"
	ActionListFeatureFlagOverrides  Action = "ListFeatureFlagOverrides"
	ActionGetTenantGithubCreds      Action = "GetTenantGithubCreds"    // #nosec G101: This is not a credential.
	ActionUpdateTenantGithubCreds   Action = "UpdateTenantGithubCreds" // #nosec G101: This is not a credential.
	ActionFindGithubUser            Action = "FindGithubUser"
	ActionCreateWorkstream          Action = "CreateWorkstream"
	ActionGetWorkstream             Action = "GetWorkstream"
	ActionUpdateWorkstream          Action = "UpdateWorkstream"
	ActionListWorkstreams           Action = "ListWorkstreams"
	ActionDeleteWorkstream          Action = "DeleteWorkstream"
	ActionAddWorkstreamShortName    Action = "AddWorkstreamShortName"
	ActionListWorkstreamShortNames  Action = "ListWorkstreamShortNames"
	ActionDeleteWorkstreamShortName Action = "DeleteWorkstreamShortName"
	ActionMoveTask                  Action = "MoveTask"
	ActionMoveShortName             Action = "MoveShortName"
	ActionListTenants               Action = "ListTenants"
	ActionCreateWorkstreamTask      Action = "CreateWorkstreamTask"
	ActionListWorkstreamTasks       Action = "ListWorkstreamTasks"
	ActionDeleteWorkstreamTask      Action = "DeleteWorkstreamTask"
	ActionUpdateWorkstreamTask      Action = "UpdateWorkstreamTask"
	ActionGetWorkstreamTask         Action = "GetWorkstreamTask"
	ActionSearchTasks               Action = "SearchTasks"
	ActionCreateRunner              Action = "CreateRunner"
	ActionCreateGithubConnection    Action = "CreateGithubConnection"
	ActionListRunners               Action = "ListRunners"
)

// TokenType defines the type of token a principal used to authenticate.
type TokenType string

const (
	TokenTypeWebUI          TokenType = "WebUIToken"
	TokenTypeAuthProvider   TokenType = "AuthProviderToken"   // #nosec: G101: This is an enum value, not a hardcoded credential.
	TokenTypeServiceAccount TokenType = "ServiceAccountToken" // #nosec: G101: This is an enum value, not a hardcoded credential.
	TokenTypeAgent          TokenType = "AgentToken"
)

// MemberRole defines the role of a user in an organization or enterprise.
type MemberRole string

const (
	MemberRoleOwner  MemberRole = "Owner"
	MemberRoleMember MemberRole = "Member"
)

// PolicyPrincipal defines the principal that a policy applies to.
type PolicyPrincipal struct {
	Type             PrincipalType `json:"Type"`
	Name             *string       `json:"Name,omitempty"`
	RoleArn          *string       `json:"RoleArn,omitempty"`
	Tenant           *string       `json:"Tenant,omitempty"`
	TokenTypes       []TokenType   `json:"TokenTypes,omitempty"`
	Provider         *string       `json:"Provider,omitempty"`
	Organization     *string       `json:"Organization,omitempty"`
	OrganizationRole *MemberRole   `json:"OrganizationRole,omitempty"`
	Enterprise       *string       `json:"Enterprise,omitempty"`
	EnterpriseRole   *MemberRole   `json:"EnterpriseRole,omitempty"`

	TokenTypesBitVector int64 `json:"-"`
}

func (p *PolicyPrincipal) GetField(name string) (any, bool) {
	switch name {
	case "Type":
		return p.Type, true
	case "Name": //nolint:goconst
		return evalNullable(p.Name)
	case "RoleArn":
		return evalNullable(p.RoleArn)
	case "Tenant":
		return evalNullable(p.Tenant)
	case "TokenTypes":
		return p.TokenTypes, true
	case "Provider":
		return evalNullable(p.Provider)
	case "Organization":
		return evalNullable(p.Organization)
	case "OrganizationRole":
		return evalNullable(p.OrganizationRole)
	case "Enterprise":
		return evalNullable(p.Enterprise)
	case "EnterpriseRole":
		return evalNullable(p.EnterpriseRole)
	default:
		return nil, false
	}
}

func (p *PolicyPrincipal) UnmarshalJSON(b []byte) error {
	type Alias PolicyPrincipal
	var tmp Alias
	if err := json.Unmarshal(b, &tmp); err != nil {
		return err
	}
	*p = PolicyPrincipal(tmp)
	p.TokenTypesBitVector = CreateBitVector(p.TokenTypes, TokenTypeToBit)
	return nil
}

// Policy represents an authorization policy.
type Policy struct {
	PolicyID           string           `json:"PolicyID"`
	Name               string           `json:"Name"`
	Effect             EffectType       `json:"Effect"`
	Tenant             *string          `json:"Tenant"`
	Principal          PolicyPrincipal  `json:"Principal"`
	Actions            []Action         `json:"Actions"`
	DelegatedActions   []Action         `json:"DelegatedActions"`
	DelegatedPrincipal *PolicyPrincipal `json:"DelegatedPrincipal"`
	Constraints        []string         `json:"Constraints"`
	CreatedAt          time.Time        `json:"CreatedAt"`
	UpdatedAt          time.Time        `json:"UpdatedAt"`

	ActionsBitVector          int64 `json:"-"`
	DelegatedActionsBitVector int64 `json:"-"`
}

func (p *Policy) GetField(name string) (any, bool) {
	switch name {
	case "PolicyID":
		return p.PolicyID, true
	case "Name":
		return p.Name, true
	case "Effect":
		return p.Effect, true
	case "Tenant":
		return evalNullable(p.Tenant)
	case "Principal":
		return p.Principal, true
	case "Actions":
		return p.Actions, true
	case "DelegatedActions":
		return p.DelegatedActions, true
	case "DelegatedPrincipal":
		return evalNullable(p.DelegatedPrincipal)
	case "Constraints":
		return p.Constraints, true
	case "CreatedAt":
		return p.CreatedAt, true
	case "UpdatedAt":
		return p.UpdatedAt, true
	default:
		return nil, false
	}
}

func evalNullable[T any](ptr *T) (any, bool) {
	if ptr == nil {
		return nil, true
	}
	return *ptr, true
}

func (p *Policy) UnmarshalJSON(b []byte) error {
	type Alias Policy
	var tmp Alias
	if err := json.Unmarshal(b, &tmp); err != nil {
		return err
	}
	*p = Policy(tmp)
	p.ActionsBitVector = CreateBitVector(p.Actions, ActionToBit)
	p.DelegatedActionsBitVector = CreateBitVector(p.DelegatedActions, ActionToBit)
	return nil
}

var (
	ActionToBit    map[Action]int64
	BitToAction    map[int64]Action
	TokenTypeToBit map[TokenType]int64
	BitToTokenType map[int64]TokenType
)

func init() {
	ActionToBit, BitToAction = createEnumMaps([]Action{
		ActionPerformDelegatedAction,    // 0x0000_0000_0000_0001
		ActionCreateTenant,              // 0x0000_0000_0000_0002
		ActionGetTenant,                 // 0x0000_0000_0000_0004
		ActionGenerateWebUIToken,        // 0x0000_0000_0000_0008
		ActionListPolicies,              // 0x0000_0000_0000_0010
		ActionUpdateTurn,                // 0x0000_0000_0000_0020
		ActionUpdateTask,                // 0x0000_0000_0000_0040
		ActionGetTask,                   // 0x0000_0000_0000_0080
		ActionListTasks,                 // 0x0000_0000_0000_0100
		ActionGetTurn,                   // 0x0000_0000_0000_0200
		ActionUploadTurnLogs,            // 0x0000_0000_0000_0400
		ActionGetCurrentUser,            // 0x0000_0000_0000_0800
		ActionCreateEnvironment,         // 0x0000_0000_0000_1000
		ActionGetEnvironment,            // 0x0000_0000_0000_2000
		ActionListEnvironments,          // 0x0000_0000_0000_4000
		ActionUpdateEnvironment,         // 0x0000_0000_0000_8000
		ActionDeleteEnvironment,         // 0x0000_0000_0001_0000
		ActionGetLastTurn,               // 0x0000_0000_0002_0000
		ActionCreateTask,                // 0x0000_0000_0004_0000
		ActionGetLastTurnLog,            // 0x0000_0000_0008_0000
		ActionStreamLogs,                // 0x0000_0000_0010_0000
		ActionListTurns,                 // 0x0000_0000_0020_0000
		ActionAddGithubOrg,              // 0x0000_0000_0040_0000
		ActionUpdateGithubOrg,           // 0x0000_0000_0080_0000
		ActionDeleteGithubOrg,           // 0x0000_0000_0100_0000
		ActionListGithubOrgs,            // 0x0000_0000_0200_0000
		ActionGetGithubOrg,              // 0x0000_0000_0400_0000
		ActionCreateFeatureFlag,         // 0x0000_0000_0800_0000
		ActionGetTenantFeatureFlags,     // 0x0000_0000_1000_0000
		ActionCreateFeatureFlagOverride, // 0x0000_0000_2000_0000
		ActionListFeatureFlags,          // 0x0000_0000_4000_0000
		ActionGetFeatureFlag,            // 0x0000_0000_8000_0000
		ActionUpdateFeatureFlag,         // 0x0000_0001_0000_0000
		ActionDeleteFeatureFlag,         // 0x0000_0002_0000_0000
		ActionDeleteFeatureFlagOverride, // 0x0000_0004_0000_0000
		ActionGetFeatureFlagOverride,    // 0x0000_0008_0000_0000
		ActionUpdateFeatureFlagOverride, // 0x0000_0010_0000_0000
		ActionListFeatureFlagOverrides,  // 0x0000_0020_0000_0000
		ActionGetTenantGithubCreds,      // 0x0000_0040_0000_0000
		ActionUpdateTenantGithubCreds,   // 0x0000_0080_0000_0000
		ActionFindGithubUser,            // 0x0000_0100_0000_0000
		ActionCreateWorkstream,          // 0x0000_0200_0000_0000
		ActionGetWorkstream,             // 0x0000_0400_0000_0000
		ActionUpdateWorkstream,          // 0x0000_0800_0000_0000
		ActionListWorkstreams,           // 0x0000_1000_0000_0000
		ActionDeleteWorkstream,          // 0x0000_2000_0000_0000
		ActionAddWorkstreamShortName,    // 0x0000_4000_0000_0000
		ActionListWorkstreamShortNames,  // 0x0000_8000_0000_0000
		ActionDeleteWorkstreamShortName, // 0x0001_0000_0000_0000
		ActionMoveTask,                  // 0x0002_0000_0000_0000
		ActionMoveShortName,             // 0x0004_0000_0000_0000
		ActionListTenants,               // 0x0008_0000_0000_0000
		ActionCreateWorkstreamTask,      // 0x0010_0000_0000_0000
		ActionListWorkstreamTasks,       // 0x0020_0000_0000_0000
		ActionDeleteWorkstreamTask,      // 0x0040_0000_0000_0000
		ActionUpdateWorkstreamTask,      // 0x0080_0000_0000_0000
		ActionGetWorkstreamTask,         // 0x0100_0000_0000_0000
		ActionSearchTasks,               // 0x0200_0000_0000_0000
		ActionCreateRunner,              // 0x0400_0000_0000_0000
		ActionCreateGithubConnection,    // 0x0800_0000_0000_0000
		ActionListRunners,               // 0x1000_0000_0000_0000
	})
	TokenTypeToBit, BitToTokenType = createEnumMaps([]TokenType{
		TokenTypeWebUI,
		TokenTypeAuthProvider,
		TokenTypeServiceAccount,
		TokenTypeAgent,
	})
}

type AuthorizationType string

const (
	AuthorizationTypeWebUIToken           AuthorizationType = "WebUIToken"
	AuthorizationTypeAuthProviderToken    AuthorizationType = "AuthProviderToken" // #nosec: G101: This is an enum value, not a hardcoded credential.
	AuthorizationTypeServiceAccountToken  AuthorizationType = "ServiceAccountToken"
	AuthorizationTypeSTSGetCallerIdentity AuthorizationType = "sts:GetCallerIdentity"
	AuthorizationTypeAgentToken           AuthorizationType = "AgentToken"
)

// ListPoliciesRequest is the request for ListPolicies.
type ListPoliciesRequest struct {
	FeatureFlags
	DelegatedAuthInfo
	TenantID   string
	MaxResults *int
	Token      *string
}

// nolint: goconst
func (r *ListPoliciesRequest) GetField(name string) (any, bool) {
	switch name {
	case "TenantID":
		return r.TenantID, true
	case "MaxResults":
		return evalNullable(r.MaxResults)
	case "Token":
		return evalNullable(r.Token)
	default:
		return nil, false
	}
}

// ListPoliciesResponse is the response from ListPolicies.
type ListPoliciesResponse struct {
	Policies  []Policy `json:"Policies"`
	NextToken *string  `json:"NextToken"`
}

// ListPolicies retrieves the policies for a tenant.
func (c *Client) ListPolicies(ctx context.Context, req *ListPoliciesRequest) (*ListPoliciesResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}

	u := c.BaseURL.JoinPath("v1", "tenants", url.PathEscape(req.TenantID), "policies")
	q := u.Query()
	if req.MaxResults != nil {
		q.Set("maxResults", strconv.Itoa(*req.MaxResults))
	}
	if req.Token != nil {
		q.Set("token", *req.Token)
	}
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	processFeatureFlags(httpReq, req.FeatureFlags)

	if err := c.authenticate(req.DelegatedAuthInfo, httpReq); err != nil {
		return nil, err
	}

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}

	var out ListPoliciesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
