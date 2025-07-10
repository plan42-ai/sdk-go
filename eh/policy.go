package eh

import (
	"encoding/json"
)

func createEnumMaps[T ~string](values []T) (map[T]int64, map[int64]T) {
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
)

// Action defines the actions that a policy can allow or deny.
type Action string

const (
	ActionPerformDelegatedAction Action = "PerformDelegatedAction"
	ActionCreateTenant           Action = "CreateTenant"
	ActionGetTenant              Action = "GetTenant"
	ActionGenerateWebUIToken     Action = "GenerateWebUIToken"
)

// TokenType defines the type of token a principal used to authenticate.
type TokenType string

const (
	TokenTypeWebUI          TokenType = "WebUIToken"
	TokenTypeAuthProvider   TokenType = "AuthProviderToken"   // #nosec: G101: This is an enum value, not a hardcoded credential.
	TokenTypeServiceAccount TokenType = "ServiceAccountToken" // #nosec: G101: This is an enum value, not a hardcoded credential.
)

// MemberRole defines the role of a user in an organization or enterprise.
type MemberRole string

const (
	MemberRoleOwner  MemberRole = "Owner"
	MemberRoleMember MemberRole = "Member"
)

// Expression represents a constraint expression.
type Expression string

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
	SchemaVersion      string           `json:"SchemaVersion"`
	Name               string           `json:"Name"`
	Effect             EffectType       `json:"Effect"`
	Tenant             *string          `json:"Tenant"`
	Principal          PolicyPrincipal  `json:"Principal"`
	Actions            []Action         `json:"Actions"`
	DelegatedActions   []Action         `json:"DelegatedActions"`
	DelegatedPrincipal *PolicyPrincipal `json:"DelegatedPrincipal"`
	Constraints        []Expression     `json:"Constraints"`

	ActionsBitVector          int64 `json:"-"`
	DelegatedActionsBitVector int64 `json:"-"`
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
	})
	TokenTypeToBit, BitToTokenType = createEnumMaps([]TokenType{
		TokenTypeWebUI,
		TokenTypeAuthProvider,
		TokenTypeServiceAccount,
	})
}
