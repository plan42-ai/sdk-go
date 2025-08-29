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
	ActionPerformDelegatedAction Action = "PerformDelegatedAction"
	ActionCreateTenant           Action = "CreateTenant"
	ActionGetTenant              Action = "GetTenant"
	ActionGenerateWebUIToken     Action = "GenerateWebUIToken"
	ActionListPolicies           Action = "ListPolicies"
	ActionUpdateTurn             Action = "UpdateTurn"
	ActionUpdateTask             Action = "UpdateTask"
	ActionGetTask                Action = "GetTask"
	ActionListTasks              Action = "ListTasks"
	ActionGetTurn                Action = "GetTurn"
	ActionUploadTurnLogs         Action = "UploadTurnLogs"
	ActionGetCurrentUser         Action = "GetCurrentUser"
	ActionCreateEnvironment      Action = "CreateEnvironment"
	ActionGetEnvironment         Action = "GetEnvironment"
	ActionListEnvironments       Action = "ListEnvironments"
	ActionUpdateEnvironment      Action = "UpdateEnvironment"
	ActionDeleteEnvironment      Action = "DeleteEnvironment"
	ActionGetLastTurn            Action = "GetLastTurn"
	ActionCreateTask             Action = "CreateTask"
	ActionGetLastTurnLog         Action = "GetLastTurnLog"
	ActionStreamLogs             Action = "StreamLogs"
	ActionListTurns              Action = "ListTurns"
	ActionAddGithubOrg           Action = "AddGithubOrg"
	ActionUpdateGithubOrg        Action = "UpdateGithubOrg"
	ActionDeleteGithubOrg        Action = "DeleteGithubOrg"
	ActionListGithubOrgs         Action = "ListGithubOrgs"
	ActionGetGithubOrg           Action = "GetGithubOrg"
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
		ActionPerformDelegatedAction,
		ActionCreateTenant,
		ActionGetTenant,
		ActionGenerateWebUIToken,
		ActionListPolicies,
		ActionUpdateTurn,
		ActionUpdateTask,
		ActionGetTask,
		ActionListTasks,
		ActionGetTurn,
		ActionUploadTurnLogs,
		ActionGetCurrentUser,
		ActionCreateEnvironment,
		ActionGetEnvironment,
		ActionListEnvironments,
		ActionUpdateEnvironment,
		ActionDeleteEnvironment,
		ActionGetLastTurn,
		ActionCreateTask,
		ActionGetLastTurnLog,
		ActionStreamLogs,
		ActionListTurns,
		ActionAddGithubOrg,
		ActionUpdateGithubOrg,
		ActionDeleteGithubOrg,
		ActionListGithubOrgs,
		ActionGetGithubOrg,
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
