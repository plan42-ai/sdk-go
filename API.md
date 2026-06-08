# 1. Authentication

We support authentication using either JWT tokens or Sigv4 Auth using AWS IAM credentials. For JWT, we support the
following types of tokens:

1. Web UI Tokens
2. Auth Provider Tokens (i.e. Google Identity Tokens)
3. Service Account Tokens
4. Agent Tokens

## 1.1 Web UI Tokens

Web UI Tokens are JWT tokens signed by Plan 42 that authenticate users of the web ui. When users login,
the website will exchange their login-provider credentials for a Web UI Auth Token, which can be stored in the browser
and used to authenticate callbacks from JavaScript.

We do this because Google Identity Tokens are only valid for 1 hour, and we don't want users to have to re-authenticate
that often. Web UI Token are generally valid for 15 days, and are automatically refreshed by the web ui after 7 days.
If a user goes more than 7 days without using the web ui, they may need to re-authenticate via their auth provider to
get a new token.

## 1.2. Auth Provider Tokens 

Auth Provider Tokens are JWT tokens signed by an external identity provider. Currently only Google Identity Tokens
are supported.  

Generally, AuthProvider tokens are used for 2 purposes:

1. To create new accounts via [CreateTenant](#3-createtenant)
2. To create Web UI tokens via [GenerateWebUIToken](#5-generatewebuitoken).

## 1.3 Service Account Tokens

Service Account Tokens are JWT tokens signed by Plan 42 that authenticate automation scripts that interact with
the API. Service Account Tokens are typically long-lived (up to 366 days). 

If you automation has access to AWS IAM credentials, consider using Sigv4 Auth instead.

## 1.4 Agent Tokens

Agent Tokens are JWT tokens signed by Plan 42 that authenticate agents that run tasks on behalf of users.
They are used to update turn status and to upload turn logs. 

## 1.5 Sigv4 Auth

Sigv4 Auth uses AWS IAM Role credentials to sign requests to the API using Sigv4. For automation scripts that have access to
AWS, this is the preferred method of authentication, as it does not require explicit secret management or rotation.

This is also the mechanism Plan 42 uses internally, for example, to authenticate between the web ui and the API.

## 1.6 Delegation

We support "delegated authentication". When ever a service (like the Web UI) performs an action on behalf of a user,
it will supply both its own authentication information, and that of the user it is acting on behalf of (the delegating principal).
Both the credentials of the calling principal and that of the delegating principal will be verified.

When authorizing the request, the api will verify the following:

1. That the "delegating principal" has permission to perform the requested operation.
2. That the "calling principal" has "PerformDelegatedAction" permission for the delegating principal and the requested
   action.

See [Authorization](#6-authorization) for more details on how policies are defined and evaluated.

Both "Web UI" and "Auth Provider" tokens are only usable in delegated contexts. They cannot be used to invoke the api
directly.

## 1.7 Authentication Headers

The following HTTP headers are used for authentication:

Authentication: <type> <token>
X-Event-Horizon-Delegating-Authorization: <type> <token>
X-Event-Horizon-Signed-Headers: <signed headers>


| Header                                   | Description                                                                                                                                                                    |
|------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Authentication                           | The authorization header for the request. See [Authorization Types](#17-authorization-types) for the list of valid <type values.                                               |
| X-Event-Horizon-Delegating-Authorization | The authorization header for the delegating principal. This is only used when the request is delegated. It is optional, but if provided, must be a valid authorization header. |
| X-Event-Horizon-Signed-Headers           | The signed headers for the request, when authenticating with Sigv4. This is only used when the request is signed using Sigv4. It is optional, but if provided, must be valid.  |

## 1.7 Authorization Types

| Value                 | Description                                                                                                                                                                                     |
|-----------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| WebUIToken            | Uses for Web UI tokens. The token should be the base64 encoding of the Web UI Token json.                                                                                                       |
| AuthProviderToken     | Used for Auth Provider tokens, such as Google Identity Tokens. The token should be the base64 encoding of the Auth Provider Token json.                                                         |
| ServiceAccountToken   | Used for Service Account tokens. The token should be the base64 encoding of the Service Account Token json.                                                                                     |
| sts:GetCallerIdentity | Used for Sigv4 authentication. The token should be the base64 encoding of a a valid signed http request to sts:GetCallerIdentity. See https://github.com/plan42-ai/sigv4util for details. |
| AgentToken            | Used for Agent Tokens. The token should be the base64 encoding of the Agent Token json.                                                                                                         |

# 2. Error Handling

When an error occurs, the API will return a 4xx or 5xx HTTP status code, along with the json body shown blow:

```json
{
  "ResponseCode": "int",
  "Message": "string",
  "ErrorType": "string"
}
```

| Field        | Type   | Description                                                                                                                                                                                                           |
|--------------|--------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| ResponseCode | int    | The HTTP status code of the error response. It's repeated in the body for convenience. This matches the value in the HTTP stats line.                                                                                 |
| Message      | string | A human readable message describing the error. The value is not stable and is subject to change. Do not use this to programmatically determine the type of error. If you do, we will break your code unceremoniously. |
| ErrorType    | string | The type of error that occurred. This value is stable, and can be used to programmatically handle errors.                                                                                                             |

On a 409 response, a conflict error will be returned. This uses the following JSON body:

```json
{
   "ResponseCode": "int",
   "Message": "string",
   "ErrorType": "string",
   "CurrentType": "string",
   "Current": {}
}
```

This is the same as the standard error response, but with additional fields:

| Field       | Type   | Description                                                                                                                                                                                       |
|-------------|--------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| CurrentType | string | The type of object that caused the conflict. Use this value to determine how to parse the fields of Object.                                                                                       |
| Current     | object | The object that caused the conflict. This is the object that was being created or updated when the conflict occurred. The type of this object is determined by the value in the ObjectType field. |

We return the current state of conflict objects in the Object field, so that you can retry the operation without needing
to make a separate request to get the object state.

# 3. CreateTenant

The CreateTenant API is used to create a new tenant. Tenants are used to isolate resources and data within the Event
Horizon platform. There are 3 types of tenants:

1. Users

   When a user logs in to Plan 42 for the first time, a new tenant is created for them. Resources created under
   the user's tenant are private to that user. 

2. Organizations

   Organizations allow groups of users to collaborate. By default, resources created under an organization are visible
   to the other members of the organization. 

3. Enterprises

   Enterprises are useful to group organizations together, so that they may share authentication, billing, and security
   policies. In larger companies it may be useful to have a single Enterprise that manages billing and security policies,
   while defining many Organizations, perhaps one per team.

Creating user tenant requires authenticating via a delegated Auth Provider Token. When
a new user tenant is created, the provided Auth Provider Token is automatically added as a Principal for the tenant.

When creating an organization or enterprise tenant, if InitialOwner is not provided, it is inferred from the caller, if
possible. If the owner is not specified and cannot be inferred, an error will be returned.

## 3.1 Request 

```http request
PUT /v1/tenants/{tenant_id} HTTP/1.1
Content-Type: application/json; charset=utf-8
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>

{
  "Type": "TenantType",
  "FullName": "*string",
  "OrgName": "*string",
  "EnterpriseName": "*string",
  "Email": "*string",
  "FirstName": "*string",
  "LastName": "*string",
  "InitialOwner" : "*string"
  "PictureURL": "*string"
}
```

| Parameter                                | Location | Type                         | Description                                                                                                                                                                                                                                                                                                                |
|------------------------------------------|----------|------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string                       | The ID of the tenant to create. This must be a v4 UUID.                                                                                                                                                                                                                                                                    |
| Authorization                            | header   | string                       | The authorization header for the request.                                                                                                                                                                                                                                                                                  |
| X-Event-Horizon-Delegating-Authorization | header   | *string                      | The authorization header for the delegating principal.                                                                                                                                                                                                                                                                     |
| X-Event-Horizon-Signed-Headers           | header   | *string                      | The signed headers for the request, when authenticating with Sigv4.                                                                                                                                                                                                                                                        |
| Type                                     | body     | [TenantType](#33-tenanttype) | The type of tenant to create. Valid values are "User", "Organization", and "Enterprise".                                                                                                                                                                                                                                   |
| FullName                                 | body     | *string                      | For user tenants: the user's full name.                                                                                                                                                                                                                                                                                    |
| OrgName                                  | body     | *string                      | For organization tenants: the organization name.                                                                                                                                                                                                                                                                           |
| EnterpriseName                           | body     | *string                      | For enterprise tenants: the enterprise name.                                                                                                                                                                                                                                                                               |
| Email                                    | body     | *string                      | For user tenants: the user's email address.                                                                                                                                                                                                                                                                                |
| FirstName                                | body     | *string                      | For user tenants: the user's first name.                                                                                                                                                                                                                                                                                   |
| LastName                                 | body     | *string                      | For user tenants: the user's last name.                                                                                                                                                                                                                                                                                    |
| InitialOwner                             | body     | *string                      | The tenant ID of the initial owner of the organization or enterprise. Optional. Only valid for organization and enterprise tenants. If not provided, the initial owner will be inferred from the delegating principal if possible. If not supplied and no delegating principal can be inferred, an error will be returned. |
| PictureURL                               | body     | *string                      | The URL of the picture for the tenant. Optional.                                                                                                                                                                                                                                                                           |

## 3.2 Response

On success a 201 CREATED is returned with the following JSON body:

```http request
HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8

{
  "TenantID": "string",
  "Type": "TenantType",
  "Version": int,
  "Deleted": boolean,
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "FullName": "*string",
  "OrgName": "*string",
  "EnterpriseName": "*string",
  "Email": "*string",
  "FirstName": "*string",
  "LastName": "*string",
  "PictureURL": "*string",
  "DefaultRunnerID" : "*string",
  "DefaultGithubConnectionID": "*string",
  "LatestEncryptionKeyVersion": "*int"
}
```

| Field                     | Type                         | Description                                                                                                     |
|---------------------------|------------------------------|-----------------------------------------------------------------------------------------------------------------|
| TenantID                  | string                       | The ID of the tenant that was created. This is a v4 UUID.                                                       |
| Type                      | [TenantType](#33-tenanttype) | The type of tenant that was created. Valid values are "user", "organization", and "enterprise".                 |
| Version                   | int                          | The version of the tenant object. Will be 1 on create. This is incremented each time the tenant is updated.     |
| Deleted                   | boolean                      | Whether the tenant is deleted. This is false on create.                                                         |
| CreatedAt                 | string                       | The timestamp when the tenant was created, in ISO 8601 format.                                                  |
| UpdatedAt                 | string                       | The timestamp when the tenant was last updated, in ISO 8601 format.                                             |
| FullName                  | *string                      | For user tenants: the user's full name.                                                                         |
| OrgName                   | *string                      | For organization tenants: the organization name.                                                                |
| EnterpriseName            | *string                      | For enterprise tenants: the enterprise name.                                                                    |
| Email                     | *string                      | For user tenants: the user's email address.                                                                     |
| FirstName                 | *string                      | For user tenants: the user's first name.                                                                        |
| LastName                  | *string                      | For user tenants: the user's last name.                                                                         |
| PictureURL                | *string                      | The URL of the picture for the tenant. Optional.                                                                |
| DefaultRunnerID           | *string                      | The ID of the default runner for the tenant. Will be nil if no default runner is defined.                       |
| DefaultGithubConnectionID | *string                      | The ID of the default github connection for the tenant. Will be nil if no default github connection is defined. |
| LatestEncryptionKeyVersion | *int                        | The version of the tenant encryption key used for new writes. Nil if no key has been created yet.               |

See [Error Handling](#2-error-handling) for details on error responses.

## 3.3 TenantType

TenantType is an enum that defines the valid type of tenants.

| Value        | 
|--------------|
| User         |
| Organization |
| Enterprise   |

## 3.5 Authorization Requirements

The caller must have CreateTenant permission.

## 3.6 Implementation Notes

When creating a tenant, the API must also create 2 KMS keys for the tenant:

1. event-horizon/${tenant_id}/logs - used to encrypt task logs.

This key must specify a resource policy that provides encrypt and decrypt permissions to the API's IAM role in addition
to allowing * on the root account.

2. event-horizon/${tenant_id}/creds - used to encrypt / decrypt github tokens. 

The key must specify a resource policy that allows:
  1. * for the root account.
  2. Encrypt for the API's IAM role.
  3. Decrypt for the "Agent Wrapper" role in the compute account.


# 4. GetTenant

The GetTenant API is used to retrieve information about a tenant. 

# 4.1 Request

```http request
GET /v1/tenants/{tenant_id} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
```

| Parameter                                | Location | Type   | Description                                                                                                                     |
|------------------------------------------|----------|--------|---------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string | The ID of the tenant to retrieve. This must be a v4 UUID.                                                                       |
| Authorization                            | header   | string | The authorization header for the request.                                                                                       |
| X-Event-Horizon-Delegating-Authorization | header   | string | The authorization header for the delegating principal. This is optional, but if provided, must be a valid authorization header. |

# 4.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "TenantID": "string",
  "Type": "TenantType",
  "Version": int,
  "Deleted": boolean,
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "FullName": "*string",
  "OrgName": "*string",
  "EnterpriseName": "*string",
  "Email": "*string",
  "FirstName": "*string",
  "LastName": "*string",
  "PictureURL": "*string",
  "DefaultRunnerID" : "*string",
  "DefaultGithubConnectionID": "*string",
  "LatestEncryptionKeyVersion": "*int"  
}
```

| See Also                            | Description                     |
|-------------------------------------|---------------------------------|
| [CreateTenant](#32-response)        | For details on response fields. |
| [Error Handling](#2-error-handling) | For details on error responses. |

# 5. GenerateWebUIToken

GenerateWebUIToken creates and signs a new WebUI token for the calling user.

## 5.1 Request

```http request
PUT /v1/tenants/{tenant_id}/ui-tokens/{token_id} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type   | Description                                                         |
|------------------------------------------|----------|--------|---------------------------------------------------------------------|
| tenant_id                                | path     | string | The ID of the tenant to create the Web UI token for.                |
| token_id                                 | path     | string | The ID of the Web UI token to create. This must be a v4 UUID.       |
| Authorization                            | header   | string | The authorization header for the request.                           |
| X-Event-Horizon-Delegating-Authorization | header   | string | The authorization header for the delegating principal.              |
| X-Event-Horizon-Signed-Headers           | header   | string | The signed headers for the request, when authenticating with Sigv4. |

NOTE: We use PUT and supply the token_id, so that retries are idempotent. If the token already exists, we will return a 409 CONFLICT
error. The response body's `Current` field will contain the existing token object.

## 5.2 Response
On success a 201 CREATED is returned with the following JSON body:

```http request
HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8

{
    "JWT": "string"
}
```

| Field | Type   | Description               |
|-------|--------|---------------------------|
| JWT   | string | The signed Web UI token.  |

# 6. Authorization

The API implements authorization using a policy-based model. For the MVP, the set of policies used is fixed, and
we do not define any policy management apis. This will be changed in the future as we add support for service accounts.
We have 2 sets of default policies:

1. Global policies that apply to all tenants. See [Default Global Policies](#7-default-global-policies) for details.
2. Default tenant policies that are created when a new tenant is created. The specific policies created depend on the
   type of tenant.

   - For user tenants, see [Default User Tenant Policies](#8-default-user-tenant-policies).
   - For organization tenants, see [Default Organization Tenant Policies](#9-default-organization-tenant-policies).
   - For enterprise tenants, see [Default Enterprise Tenant Policies](#10-default-enterprise-tenant-policies).

## 6.1 Policy Schema

Policies are defined using JSON. 

```json
{

  "PolicyID" : "string",
  "Name":   "string",
  "Effect" : "EffectType",
  "Tenant" : "*string",
  "Principal": {},
  "Actions": [],   
  "DelegatedActions":  [],
  "DelegatedPrincipal": {},
  "Constraints":  [],
  "CreatedAt": "string", 
  "UpdatedAt": "string"
}
```

| Field              | Type                              | Description                                                                                                                                                                                                                                                                |
|--------------------|-----------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| PolicyID           | string                            | The unique identifier for the policy. This is a v4 UUID.                                                                                                                                                                                                                   |
| Name               | string                            | The name of the policy. This must be unique within the tenant.                                                                                                                                                                                                             |
| Effect             | [EffectType](#62-effecttype)      | The effect of the policy. This can be "Allow" or "Deny".                                                                                                                                                                                                                   |
| Tenant             | *string                           | The TennantID that the policy applies to. If this is null, the policy applies to contexts that do not specify a tenant (such as CreateTenant). If this is "*", the policy applies to all tenants. If this is a specific tenant ID, the policy applies only to that tenant. |
| Principal          | [Principal](#63-policyprincipal)  | The principal that the policy applies to.                                                                                                                                                                                                                                  |
| Actions            | [][Action](#65-action)            | The actions the policy allows or denies.                                                                                                                                                                                                                                   |
| DelegatedActions   | [][Action](#65-action)            | Only valid when action is `PerformDelegatedAction`. It identifies the section of actions that can be delegated.                                                                                                                                                            |
| DelegatedPrincipal | [*Principal](#63-policyprincipal) | Only valid when action is `PerformDelegatedAction`. It identifies the principal for which delegation is enabled                                                                                                                                                            |
| Constraints        | [][Expression](#66-expressions)   | A list of constraints expressions that must be satisfied for the policy to apply. They are dynamic and are evaluated at policy evaluation time.                                                                                                                            |
| CreatedAt          | string                            | The timestamp when the policy was created, in ISO 8601 format.                                                                                                                                                                                                             |
| UpdatedAt          | string                            | The timestamp when the policy was last updated, in ISO 8601 format.                                                                                                                                                                                                        |

## 6.2 EffectType

EffectType is an enum that defines whether a policy allows or denies access to a resource.

| Value |
|-------|
| Allow |
| Deny  |

## 6.3 PolicyPrincipal

A PolicyPrincipal is an object that defines the principal that a policy applies to. 

```json
{
  "Type": "PrincipalType",
  "Name": "*string",
  "RoleArn": "*string",
  "Tenant": "*string",
  "TokenTypes": [],
  "Provider": "*string",
  "MemberOf": "*string",
  "MemberRole": "*string"
}
```

| Field            | Type                               | Description                                                                                                                                                                                                                                                              |
|------------------|------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Type             | [PrincipalType](#64-principaltype) | The type of principal.                                                                                                                                                                                                                                                   |
| Name             | *string                            | The name of the principal. Only used for `Service` and `ServiceAccount` principals.                                                                                                                                                                                      |
| RoleArn          | *string                            | The ARN of the IAM role. Only used for `IAMRole` principals.                                                                                                                                                                                                             |
| Tenant           | *string                            | The TenantID of the principal. Only used for `User` and `ServiceAccount` principals. May also be an [Expression](#116-expressions or the value `*`.                                                                                                                      |
| TokenTypes       | [][TokenType](#67-tokentype)       | When specified, restricts the policy to only apply to principals that authenticated using one of the specified token types.                                                                                                                                              |
| Provider         | *string                            | The name of the authentication provider for the principal. Only valid for `AuthProviderToken` token types. Currently only "Google" is supported.                                                                                                                         |
| Organization     | *string                            | The TenantID of the organization that the principal is a member of. When set restricts the policy to only apply to principals that are members of the provided org. Only valid for `User` and `ServiceAccount` principals. May also be an [Expression](#66-expressions). |
| OrganizationRole | [*MemberRole](#68-memberrole)      | The role of the principal in the organization. Only valid for `User` principals. Valid values are "Owner" and "Member".                                                                                                                                                  |
| Enterprise       | *string                            | The TenantID of the enterprise that the principal is a member of. When set restricts the policy to only apply to principals that are members of the provided enterprise. Only valid for `User` and `ServiceAccount` principals.                                          |
| EnterpriseRole   | [*MemberRole](#68-memberrole)      | The role of the principal in the enterprise. Only valid for `User` principals. Valid values are "Owner" and "Member".                                                                                                                                                    |

## 6.4 PrincipalType

PrincipalType is an enum that defines the type of principal that a policy applies to.

| Value          | Description                                                                                                                                                                                                                                      |
|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| User           | A human user.                                                                                                                                                                                                                                    |
| IAMRole        | An AWS IAM Role authenticating via Sigv4.                                                                                                                                                                                                        |
| Service        | An named alias for an IAM Role. This is used to enable policies to refer to Plan 42 services without exposing our role arns to customers (which would make it impossible to ever change them). Valid Services names are 'WebUI' and 'AdminRole'. |
| ServiceAccount | A service account.                                                                                                                                                                                                                               |
| Agent          | An executing agent invocation.                                                                                                                                                                                                                   |
| Runner         | A runner instance.                                                                                                                                                                                                                               |

## 6.5 Action

Action is an enum that defines the actions that a policy can allow or deny.

| Value                     |
|---------------------------|
| PerformDelegatedAction    |
| CreateTenant              |
| GetTenant                 |
| GenerateWebUIToken        |
| ListPolicies              |
| UpdateTurn                |
| UpdateTask                |
| GetTask                   |
| ListTasks                 |
| GetTurn                   |
| UploadTurnLogs            |
| GetCurrentUser            |
| CreateEnvironment         |
| GetEnvironment            |
| ListEnvironments          |
| UpdateEnvironment         |
| DeleteEnvironment         |
| GetLastTurn               |
| CreateTask                |
| GetLastTurnLog            |
| StreamLogs                |
| ListTurns                 |
| AddGithubOrg              |
| UpdateGithubOrg           |
| DeleteGithubOrg           |
| ListGithubOrgs            |
| GetGithubOrg              |
| CreateFeatureFlag         |
| GetTenantFeatureFlags     |
| CreateFeatureFlagOverride |
| ListFeatureFlags          |
| GetFeatureFlag            |
| UpdateFeatureFlag         |
| DeleteFeatureFlag         |
| DeleteFeatureFlagOverride |
| GetFeatureFlagOverride    |
| UpdateFeatureFlagOverride |
| ListFeatureFlagOverrides  |
| GetTenantGithubCreds      |
| UpdateTenantGithubCreds   |
| FindGithubUser            |
| CreateWorkstream          |
| GetWorkstream             |
| UpdateWorkstream          |
| ListWorkstreams           |
| DeleteWorkstream          |
| AddWorkstreamShortName    |
| ListWorkstreamShortNames  |
| DeleteWorkstreamShortName |
| MoveTask                  |
| MoveShortName             |
| ListTenants               |
| CreateWorkstreamTask      |
| ListWorkstreamTasks       |
| DeleteWorkstreamTask      |
| UpdateWorkstreamTask      |
| GetWorkstreamTask         |
| SearchTasks               |
| CreateRunner              |
| CreateGithubConnection    |
| ListRunners               |
| DeleteRunner              |
| ListGithubConnections     |
| GetRunner                 |
| UpdateRunner              |
| DeleteGithubConnection    |
| GenerateRunnerToken       |
| GetGithubConnection       |
| RevokeRunnerToken         |
| UpdateGithubConnection    |
| ListRunnerTokens          |
| GetMessagesBatch          |
| RegisterRunnerQueue       |
| WriteResponse             |
| CreateTurn                |

## 6.6 Expressions

We support evaluating expressions in policies. Eventually we should define a full expression grammar here. For MVP we
only need to support the following expressions:

| Expression             | Description                                                                                 |
|------------------------|---------------------------------------------------------------------------------------------|
| $request.<FieldName>   | A field from the request object for an api call.                                            |
| $policy.<FieldName>    | A field from the policy object being evaluated.                                             |
| 'StringLiteral'        | A string literal.                                                                           |
| uuid                   | A uuid litera. For example, 42B996AB-D130-45A6-B9D6-085313CFB0DF                            |
| expr == expr           | An expression that evaluates to true if the left-hand side is equal to the right-hand side. |

## 6.7 TokenType

TokenType is an enum that defines the type of token that a principal used to authenticate.

| Value               | Description                                                                                              |
|---------------------|----------------------------------------------------------------------------------------------------------|
| WebUIToken          | A token issued by the web ui.                                                                            |
| AuthProviderToken   | A token issued by an external identity provider, such as Google Identity Tokens.                         |
| ServiceAccountToken | A token issued by a service account. This is used for automation scripts that interact with the API.     |
| AgentToken          | A token representing an invocation of agent. Used to update Turn and Task state and to update turn logs. |

## 6.8 MemberRole

MemberRole is an enum that defines the role of a user in an organization or enterprise.

| Value  | 
|--------|
| Owner  |
| Member |

# 7. Default Global Policies

The policies below are defined globally (on either the null tenant or the "*" tenant) and apply to all tenants.

## 7.1 Enable Account Creation From UI

This policy allows the web ui to create new accounts for users that authenticate via Google Identity Tokens.
There are some interesting things to note about the policy definition:

1. Its `Tenant` value is `null`, which means the policy only applies in contexts that don't specify a tenant. This is limited to `CreateTenant`.
2. The `DelegatedPrincipal` also specifies a `null` tenant. This means it can only be used with Google Identity Tokens which are not yet associated with a tenant.
3. It specifies a constraints that this only applies to requests where the `TenantType` in the request is `User`.

```json
{
  "Name": "EnableAccountCreationFromUI",
  "Effect": "Allow",
  "Tenant": null,
  "Principal": {
    "Type": "Service",
    "Name": "WebUI"
  },
  "Actions": ["PerformDelegatedAction"],
  "DelegatedActions": ["CreateTenant"],
  "DelegatedPrincipal": {
    "Type": "User",
    "Tenant" : null,
    "TokenTypes": ["AuthProviderToken"],
    "Provider": "Google"
  },
  "Constraints" : ["$request.Type == 'User'"] 
}
```

## 7.2 Enable Account Creation Via the Admin Role

```json
{
  "Name": "EnableAccountCreationFromAdminRole",
  "Effect": "Allow",
  "Tenant": null,
  "Principal": {
    "Type": "Service",
    "Name": "AdminRole"
  },
  "Actions": ["PerformDelegatedAction"],
  "DelegatedActions": ["CreateTenant"],
  "DelegatedPrincipal": {
    "Type": "User",
    "Tenant" : null,
    "TokenTypes": ["AuthProviderToken"],
    "Provider": "Google"
  },
  "Constraints" : ["$request.Type == 'User'"] 
}
```

## 7.3 Enable Admin Access

This policy allows our internal admin role to perform any action on any tenant.

```json
{
  "Name": "EnableAdminAccess",
  "Effect": "Allow",
  "Tenant": "*",
  "Principal": {
    "Type": "Service",
    "Name": "AdminRole"
  },
  "Actions": ["*"]
}
```

## 7.4 Enable Account Creation

```json
{
   "Name": "EnableAccountCreation",
   "Effect": "Allow",
   "Tenant": "null",
   "Principal": {
     "Type": "User",
     "Tenant": null,
     "TokenTypes": [
       "AuthProviderToken"
     ],
     "Provider": "Google"
   },
   "Actions": ["CreateTenant"],
   "Constraints" : ["$request.Type == 'User'"]
}
```
# 8. Default User Tenant Policies

## 8.1 EnableWebUIDelegation

This policy allows the Web UI to perform any delegated actions on behalf of user tenants that authenticate via Web UI
Tokens.

```json
{
  "Name": "EnableWebUIDelegation",
  "Effect": "Allow",
  "Tenant": "tenant_id",
  "Principal": {
    "Type": "Service",
    "Name": "WebUI"
  },
  "Actions": ["PerformDelegatedAction"],
  "DelegatedActions": ["*"],
  "DelegatedPrincipal": {
    "Type": "User",
    "Tenant" : "$policy.Tenant",
    "TokenTypes": ["WebUIToken"]
  }
}
```

## 8.2 EnableAdminDelegation

This policy allows the Admin Role to perform any delegated actions on behalf of user tenants that authenticate via Web UI
Tokens.

```json
{
  "Name": "EnableWebUIDelegation",
  "Effect": "Allow",
  "Tenant": "tenant_id",
  "Principal": {
    "Type": "Service",
    "Name": "AdminRole"
  },
  "Actions": ["PerformDelegatedAction"],
  "DelegatedActions": ["*"],
  "DelegatedPrincipal": {
    "Type": "User",
    "Tenant" : "$policy.Tenant",
    "TokenTypes": ["WebUIToken"]
  }
}
```

## 8.3 GenerateWebUIToken

This policy allows the Web UI to generate Web UI tokens for users that authenticate via Google Identity Tokens.

```json
{
  "Name": "GenerateWebUIToken",
  "Effect": "Allow", 
  "Tenant": "tenant_id",
  "Principal":  {
    "Type": "Service",
    "Name": "WebUI"
  },
  "Actions": ["PerformDelegatedAction"],   
  "DelegatedActions": ["GenerateWebUIToken"], 
  "DelegatedPrincipal": {
    "Type": "User",
    "Tenant": "$policy.Tenant",
    "TokenTypes": ["AuthProviderToken"],
    "Provider": "Google"
  }    
}
```

## 8.4 UserAccess

This policy allows users to access their own tenant.

```json
{
  "Name": "UserAccess",
  "Effect": "Allow",
  "Tenant": "tenant_id",
  "Principal": {
    "Type": "User",
    "Tenant" : "$policy.Tenant"
  },
  "Actions": ["*"]
}
```

## 8.5 AgentTurnAccess

```json
{
    "Name": "AgentTurnAccess",
    "Effect": "Allow",
    "Tenant": "tenant_id",
    "Principal": {
        "Type": "Agent"
    },
    "Actions": ["UpdateTurn", "UploadTurnLogs", "GetLastTurnLog"],
    "Constraints": [
        "$request.TenantID == $policy.Tenant",
        "$request.TenantID == $principal.Tenant",
        "$request.TaskID == $principal.TaskID",
        "$request.TurnIndex == $principal.TurnIndex"
    ]
}
```

## 8.6 AgentTaskAccess

```json
{
    "Name": "AgentTaskAccess",
    "Effect": "Allow",
    "Tenant": "tenant_id",
    "Principal": {
        "Type": "Agent"
    },
    "Actions": ["UpdateTask"],
    "Constraints": [
        "$request.TenantID == $policy.Tenant",
        "$request.TenantID == $principal.Tenant",
        "$request.TaskID == $principal.TaskID"
    ]
}
```

## 8.7 GetCurrentUser

```json
{
   "Name": "GetCurrentUserFromWebUI",
    "Effect": "Allow",
    "Tenant": "tenant_id",
    "Principal":  {
      "Type": "Service",
      "Name": "WebUI"
    },
   "Actions": ["PerformDelegatedAction"],
   "DelegatedActions": ["GetCurrentUser"],
   "DelegatedPrincipal": {
      "Type": "User",
      "Tenant": "$policy.Tenant",
      "TokenTypes": ["AuthProviderToken"],
      "Provider": "Google"
   }
}
```

## 8.8 GetCurrentUser

```json
{
   "Name": "GetCurrentUserWithAdminRole",
    "Effect": "Allow",
    "Tenant": "tenant_id",
    "Principal":  {
      "Type": "Service",
      "Name": "WebUI"
    },
   "Actions": ["PerformDelegatedAction"],
   "DelegatedActions": ["GetCurrentUser"],
   "DelegatedPrincipal": {
      "Type": "User",
      "Tenant": "$policy.Tenant",
      "TokenTypes": ["AuthProviderToken"],
      "Provider": "Google"
   }
}
```

## 8.9 Runner Access

```json
{
  "Name": "RunnerAccess",
  "Effect": "Allow",
  "Tenant": "tenant_id",
  "Principal": {
    "Type": "Runner"
  },
  "Actions" : [
    "RegisterRunnerQueue",
    "GetMessagesBatch",
    "WriteResponse"
  ],
  "Constraints": [
    "$request.Tenant == $policy.Tenant",
    "$request.RunnerID == $principal.RunnerID"
  ]
}
```

# 11. ListPolicies

The ListPolicies API is used to list all policies for a tenant. 

## 11.1 Request

```http request
GET /v1/tenants/{tenant_id}/policies?maxResults={maxResults}&token={token} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                                                                                                                                                    |
|------------------------------------------|----------|---------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant to list policies for. The can be a tenant ID, "*" for global policies that apply to all tenants, or "_" for policies that apply to contexts that do not specify a tenant. |
| maxResults                               | query    | *int    | The maximum number of policies to return. Optional. Default is 10. Must be >=1 and <= 500.                                                                                                     |
| token                                    | query    | *string | A token to retrieve the next page of results. Optional. If not provided, the first page of results is returned.                                                                                |
| Authorization                            | header   | string  | The authorization header for the request.                                                                                                                                                      |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                                                                                                         |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                                                                                            |

## 11.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "Policies": [],
  "NextToken": "*string"
}
```

| Field     | Type                          | Description                                                                                          |
|-----------|-------------------------------|------------------------------------------------------------------------------------------------------|
| Policies  | [][Policy](#61-policy-schema) | A list of policies for the tenant. See [Policy](#61-policy-schema) for details on the policy object. |
| NextToken | *string                       | A token to retrieve the next page of results. If there are no more results, this will be null.       |

# 12. GetCurrentUser

The GetCurrentUser API is similar to the GetTenant API, but it returns information about the currently authenticated user.
If the caller is not a user, it returns a 403 Forbidden error.

## 12.1 Request

```http request
GET /v1/current-user HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                         |
|------------------------------------------|----------|---------|---------------------------------------------------------------------|
| Authorization                            | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.              |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4. |

## 12.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "TenantID": "string",
  "Type": "TenantType",
  "Version": int,
  "Deleted": boolean,
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "FullName": "*string",
  "OrgName": "*string",
  "EnterpriseName": "*string",
  "Email": "*string",
  "FirstName": "*string",
  "LastName": "*string"
}
```

| See Also                            | Description                     |
|-------------------------------------|---------------------------------|
| [GetTenant](#32-response)           | For details on response fields. |

# 13. CreateEnvironment

The CreateEnvironment API is used to create a new environment for a tenant. An environment describes the cloud 
environment used to execute tasks.

## 13.1 Request

```http request
PUT /v1/tenants/{tenant_id}/environments/{environment_id} HTTP/1.1
Accept: application/json
Content-Type: application/json; charset=utf-8
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>

{
    "Name": "string",
    "Description": "string",
    "Context": "string",
    "Repos": [],
    "SetupScript": "string",
    "DockerImage": "*string",
    "AllowedHosts": [],
    "EnvVars": [],
    "RunnerID" : "*string",
    "GithubConnectionID" : "*string"
}
```

| Parameter                                | Location | Type                    | Description                                                                                                                                                                                                           |
|------------------------------------------|----------|-------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string                  | The ID of the tenant to create the environment for.                                                                                                                                                                   |
| environment_id                           | path     | string                  | The ID of the environment to create. This must be a v4 UUID.                                                                                                                                                          |
| Authorization                            | header   | string                  | The authorization header for the request.                                                                                                                                                                             |
| X-Event-Horizon-Delegating-Authorization | header   | *string                 | The authorization header for the delegating principal.                                                                                                                                                                |
| X-Event-Horizon-Signed-Headers           | header   | *string                 | The signed headers for the request, when authenticating with Sigv4.                                                                                                                                                   |
| Name                                     | body     | string                  | The name of the environment.                                                                                                                                                                                          |
| Description                              | body     | string                  | A description of the environment.                                                                                                                                                                                     |
| Context                                  | body     | string                  | Context describing the environment to provide to AI agents that use this environment.                                                                                                                                 |
| Repos                                    | body     | []string                | A list of repositories to use in the environment, of the form org/repo. At most 50 repos can be specified.                                                                                                            |
| SetupScript                              | body     | string                  | A script to run to set up the environment. Size must be <= 512 KB                                                                                                                                                     |
| DockerImage                              | body     | *string                 | The Docker image to use for the environment. Optional. Defaults to the latest Plan 42 agent wrapper image.                                                                                                      |
| AllowedHosts                             | body     | []string                | A list of outbound hostnames the environment is allowed to connect to. Only TLS connections to hosts with public trusted certs or internal event-horizon oss mirrors are allowed.  At most 50 hosts can be specified. |
| EnvVars                                  | body     | [][EnvVar](#132-envvar) | A list of environment variables to set in the environment. At most 50 env vars may be specified.                                                                                                                      |
| RunnerID                                 | body     | *string                 | Optional. The ID of the runner to use for the environment. Defaults to "default" when omitted. Must be the id of a runner or the value "default".                                                                    |
| GithubConnectionID                       | body     | *string                 | Optional. The ID of the GitHub connection to use for checking out code when running tasks in this environment. Defaults to "default" when omitted. Must be the ID of a Github Connection or the value "default".      |

## 13.2 EnvVar

EnvVar is an object that defines an environment variable to set in the environment.

```json
{
  "Name": "string",
  "Value": "string",
  "IsSecret": bool
}
```

| Field    | Type   | Description                                                                                                                           |
|----------|--------|---------------------------------------------------------------------------------------------------------------------------------------|
| Name     | string | The name of the environment variable.                                                                                                 |
| Value    | string | The value of the environment variable.                                                                                                |
| IsSecret | bool   | Whether the value is a secret. Secret environment variables are only made available to setup scripts, are not available to the agent. |

## 13.3 Response

On success a 201 CREATED is returned with the following JSON body:

```http request
HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8

{
  "TenantID": "string",
  "EnvironmentID": "string",
  "Name": "string",
  "Description": "string",
  "Context": "string",
  "Repos": [],
  "SetupScript": "string",
  "DockerImage": "string",
  "AllowedHosts": [],
  "EnvVars": [],
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "Deleted": bool,
  "Version": int,
  "RunnerID": "string",
  "GithubConnectionID": "string"
}
```
| Field              | Type                    | Description                                                                                                                                                                       |
|--------------------|-------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| TenantID           | string                  | The ID of the tenant the environment was created for.                                                                                                                             |
| EnvironmentID      | string                  | The ID of the environment that was created. This is a v4 UUID.                                                                                                                    |
| Name               | string                  | The name of the environment.                                                                                                                                                      |
| Description        | string                  | A description of the environment.                                                                                                                                                 |
| Context            | string                  | Context describing the environment to provide to AI agents that use this environment.                                                                                             |
| Repos              | []string                | A list of repositories to use in the environment, of the form org/repo.                                                                                                           |
| SetupScript        | string                  | A script to run to set up the environment.                                                                                                                                        |
| DockerImage        | string                  | The Docker image to use for the environment.                                                                                                                                      |
| AllowedHosts       | []string                | A list of outbound hostnames the environment is allowed to connect to. Only TLS connections to hosts with public trusted certs or internal event-horizon oss mirrors are allowed. |
| EnvVars            | [][EnvVar](#132-envvar) | A list of environment variables set in the environment.                                                                                                                           |
| CreatedAt          | string                  | The timestamp when the environment was created, in ISO 8601 format.                                                                                                               |
| UpdatedAt          | string                  | The timestamp when the environment was last updated, in ISO 8601 format.                                                                                                          |
| Deleted            | bool                    | Whether the environment has been deleted.                                                                                                                                         |
| Version            | int                     | The version of the environment. This is incremented every time the environment is updated.                                                                                        |
| RunnerID           | string                  | The ID of the runner used for the environment.                                                                                                                                    |
| GithubConnectionID | string                  | The ID of the GitHub connection used for checking out code when running tasks in this environment.                                                                                |

# 14. ListEnvironments

The ListEnvironments API is used to list all environments for a tenant.

## 14.1 Request

```http request
GET /v1/tenants/{tenant_id}/environments?maxResults={maxResults}&token={token}&includeDeleted={includeDeleted} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                                                                     |
|------------------------------------------|----------|---------|-----------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant to list environments for.                                                                  |
| maxResults                               | query    | *int    | The maximum number of environments to return. Optional. Default is 10. Must be >=1 and <= 500.                  |
| token                                    | query    | *string | A token to retrieve the next page of results. Optional. If not provided, the first page of results is returned. |
| includeDeleted                           | query    | *bool   | Whether to include deleted environments in the results. Optional. Default is false.                             |
| Authorization                            | header   | string  | The authorization header for the request.                                                                       |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                          |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                             |

## 14.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "Environments": [],
  "NextToken": "*string"
}
```

| Field        | Type                              | Description                                                                                    |
|--------------|-----------------------------------|------------------------------------------------------------------------------------------------|
| Environments | [][Environment](#131-environment) | A list of environments for the tenant.                                                         |
| NextToken    | *string                           | A token to retrieve the next page of results. If there are no more results, this will be null. |

# 15. GetEnvironment

The GetEnvironment API is used to get an environment for a tenant.

## 15.1 Request

```http request
GET /v1/tenants/{tenant_id}/environments/{environment_id}?includeDeleted={&includeDeleted} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                                          |
|------------------------------------------|----------|---------|--------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant to get the environment for.                                     |
| environment_id                           | path     | string  | The ID of the environment to get.                                                    |
| includeDeleted                           | query    | *bool   | Whether to include deleted environments in the response. Optional. Default is false. |
| Authorization                            | header   | string  | The authorization header for the request.                                            |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                               |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                  |

## 15.2 Response
On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "TenantID": "string",
  "EnvironmentID": "string",
  "Name": "string",
  "Description": "string",
  "Context": "string",
  "Repos": [],
  "SetupScript": "string",
  "DockerImage": "string",
  "AllowedHosts": [],
  "EnvVars": [],
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "Deleted": bool,
  "Version": int,
  "RunnerID": "string",
  "GithubConnectionID": "string"
}
```

See [CreateEnvironment](#133-response) for details on the response fields.

# 16. UpdateEnvironment

The UpdateEnvironment API is used to update an existing environment for a tenant.

## 16.1 Request

```http request
PATCH /v1/tenants/{tenant_id}/environments/{environment_id} HTTP/1.1
Accept: application/json
Content-Type: application/json; charset=utf-8
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>

{
    "Name": "*string",
    "Description": "*string",
    "Context": "*string",
    "Repos": [],
    "SetupScript": "string",
    "DockerImage": "string",
    "AllowedHosts": [],
    "EnvVars": [],
    "Deleted": *bool,
    "RunnerID" : "*string",
    "GithubConnectionID" : "*string"
}
```

| Parameter                                | Location | Type      | Description                                                                                                                                                              |
|------------------------------------------|----------|-----------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string    | The ID of the tenant to update the environment for.                                                                                                                      |
| environment_id                           | path     | string    | The ID of the environment to update.                                                                                                                                     |
| Authorization                            | header   | string    | The authorization header for the request.                                                                                                                                |
| X-Event-Horizon-Delegating-Authorization | header   | *string   | The authorization header for the delegating principal.                                                                                                                   |
| X-Event-Horizon-Signed-Headers           | header   | *string   | The signed headers for the request, when authenticating with Sigv4.                                                                                                      |
| version                                  | header   | string    | The version of the environment to update. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned.              |
| Name                                     | body     | *string   | If set, update the environment's name                                                                                                                                    |
| Description                              | body     | *string   | If set, update the environment' description.                                                                                                                             |
| Context                                  | body     | *string   | If set, update the environment's context.                                                                                                                                |
| Repos                                    | body     | *[]string | If set, update the set of repos associated with the environment. Note that `null` means 'don't update the rpos', where as `[]` means 'set the repos to empty'.           |
| SetupScript                              | body     | *string   | If set, update the setup script used to configure the environment.                                                                                                       |
| DockerImage                              | body     | *string   | If set, update the docker image used by the environment.                                                                                                                 |
| Deleted                                  | body     | *bool     | If set to false, undelete the enviornment. May not be set to true. Use DeleteEnvironment instead.                                                                        |
| RunnerID                                 | body     | *string   | If set, update the ID of the runner used for the environment. Set to "default" to use the default runner.                                                                |
| GithubConnectionID                       | body     | *string   | If set, update the ID of the GitHub connection used for checking out code when running tasks in this environment. Set to "default" to use the default github connection. |

## 16.2 Response

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "TenantID": "string",
  "EnvironmentID": "string",
  "Name": "string",
  "Description": "string",
  "Context": "string",
  "Repos": [],
  "SetupScript": "string",
  "DockerImage": "string",
  "AllowedHosts": [],
  "EnvVars": [],
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "Deleted": bool,
  "Version": int,
  "RunnerID": "string",
  "GithubConnectionID": "string"
}
```

# 17. DeleteEnvironment

The DeleteEnvironment api soft-deletes an environment.

## 17.1 Request

```http request
DELETE /v1/tenants/{tenant_id}/environments/{environment_id}
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>
```

| Parameter                                | Location | Type      | Description                                                                                                                                                   |
|------------------------------------------|----------|-----------|---------------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string    | The ID of the tenant that owns the environment being deleted.                                                                                                 |
| environment_id                           | path     | string    | The ID of the environment to delete.                                                                                                                          |
| Authorization                            | header   | string    | The authorization header for the request.                                                                                                                     |
| X-Event-Horizon-Delegating-Authorization | header   | *string   | The authorization header for the delegating principal.                                                                                                        |
| X-Event-Horizon-Signed-Headers           | header   | *string   | The signed headers for the request, when authenticating with Sigv4.                                                                                           |
| version                                  | header   | string    | The version of the environment to delete. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned.   |

## 17.2 Response

```http request
HTTP/1.1 204 NO CONTENT
```

On success a 204 NO CONTENT is returned with no body.

# 18. CreateTask

CreateTask creates a new task. If the task is executable (assigned to AI and not blocked on another task), a Turn
will also be created and scheduled for execution.

## 18.1 Request

```http request
PUT /v1/tenants/{tenant_id}/tasks/{task_id} HTTP/1.1
Accept: application/json
Content-Type: application/json; charset=utf-8
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>

{
  "Title": "string",
  "EnvironmentID": "string",
  "Prompt": "string",
  "Model": "*ModelType",
  "RepoInfo" : {}
  "FileIDs" :[]
}
```

| Parameter                                | Location | Type                                  | Description                                                                           |
|------------------------------------------|----------|---------------------------------------|---------------------------------------------------------------------------------------|
| tenant_id                                | path     | string                                | The ID of the tenant to create the task for.                                          |
| task_id                                  | path     | string                                | The ID of the task to create. This must be a v4 UUID.                                 |
| Authorization                            | header   | string                                | The authorization header for the request.                                             |
| X-Event-Horizon-Delegating-Authorization | header   | *string                               | The authorization header for the delegating principal.                                |
| X-Event-Horizon-Signed-Headers           | header   | *string                               | The signed headers for the request, when authenticating with Sigv4.                   |
| Title                                    | body     | string                                | The title of the task.                                                                |
| EnvironmentID                            | body     | string                                | The ID of the environment to execute the task in.                                     |
| Prompt                                   | body     | string                                | The prompt to use for the task.                                                       |
| Model                                    | body     | [ModelType](#182-modeltype)           | The model to use for the task. Required if the task is not assigned to a human.       |
| RepoInfo                                 | body     | map[string][*RepoInfo](#185-repoinfo) | A map of "org/repo" to repo info.                                                     |
| FileIDs                                  | body     | []string                              | A list of file IDs to attach to the task. At most 25 files can be attached to a task. |

## 18.2 ModelType

ModelType is an enum that defines the type of model to use for the task. 

| Value             |
|-------------------|
| Codex Mini        |
| GPT-5.1 Codex     |
| GPT-5.1 Codex Max |
| GPT-5.2 Codex     |
| GPT-5.3 Codex     |
| GPT 5.4           |
| GPT 5.4 (1M)      |
| Claude 4.5 Opus   |
| Claude 4.6 Opus   |

## 18.3 Response

On success a 201 CREATED is returned with the following JSON body:

```http request
HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8

{
  "TenantID": "string",
  "WorkstreamID": "*string",  
  "TaskID": "string", 
  "Title": "string",
  "EnvironmentID": "*string",
  "Prompt": "string",
  "Parallel": bool,
  "Model": "*ModelType",
  "AssignedToTenantID": "*string",
  "AssignedToAI" : bool,  
  "RepoInfo: {},
  "LastTurnStatus": "*string",
  "LastTurnIndex": "*int",
  "State": "TaskState",
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "Deleted": bool,
  "Version": int,
  "TaskNumber": int,
  "FileIDs": [],
  "NewFileIDs": [],
}
```

| Field              | Type                                    | Description                                                                                                                                                           |
|--------------------|-----------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| TenantID           | string                                  | The ID of the tenant that owns the task.                                                                                                                              |
| WorkstreamID       | *string                                 | The ID of the workstream the task is a member. Is null if the task is not associated with a workstream.                                                               |
| TaskID             | string                                  | The ID of the task.                                                                                                                                                   |
| Title              | string                                  | The title of the task.                                                                                                                                                |
| EnvironmentID      | *string                                 | The ID of the environment the task is executed in.                                                                                                                    |
| Prompt             | string                                  | The prompt / description of the task.                                                                                                                                 |
| Parallel           | bool                                    | If true, the task can be executed in parallel with other tasks in the same workstream. Can only be true if the task is part of a workstream.                          |
| Model              | [ModelType](#182-modeltype)             | The model to use for the task. Required if the task is not assigned to a human.                                                                                       |
| AssignedToTenantID | *string                                 | The ID of the human user the task is assigned to. Only valid if the task is part of a workstream.                                                                     |
| AssignedToAI       | bool                                    | If true, the task is assigned to an AI agent. Must be true if the task is not part of a workstream. If false and `AssignedToTenantID` is nul, the task is unassigned. |
| RepoInfo           | map[string][[*RepoInfo](#185-repoinfo)] | A map of "org/repo" to repository info. This tracks branch names and PR links for each repo used in the environment.                                                  |
| LastTurnStatus     | *string                                 | The status of the latest turn for the task, if any. Non-terminal values (for example `Pending`) indicate work is still running.                                       |
| LastTurnIndex      | *int                                    | The index of the latest turn for the task, if any.                                                                                                                    |
| State              | [TaskState](#186-taskstate)             | The current state of the task.                                                                                                                                        |
| CreatedAt          | string                                  | The timestamp when the task was created, in ISO 8601 format.                                                                                                          |
| UpdatedAt          | string                                  | The timestamp when the task was last updated, in ISO 8601 format.                                                                                                     |
| Deleted            | bool                                    | Whether the task has been deleted.                                                                                                                                    |
| Version            | int                                     | The version of the task. This is incremented every time the task is updated.                                                                                          |
| TaskNumber         | *int                                    | The number of the task within the workstream. This is a sequential number assigned when the task is created. Is nil if the task is not part of a workstream.          |
| FileIDs            | []string                                | A list of file IDs attached to the task.                                                                                                                              |
| NewFileIDs         | []string                                | A list of new file IDS that were attached to a task, but have not yet been sent to the model.                                                                         |

## 18.5 RepoInfo

RepoInfo is an object that contains information about a repository used in a task's environment.


```json
{
   "PRLink": "*string",
   "PRID": "*string",
   "PRNumber": *int,
   "PRStatus": "*string",
   "PRStatusUpdatedAt": "*string",
   "FeatureBranch": "string",
   "TargetBranch": "string"
}
```

| Field         | Type    | Description                                                                                                     |
|---------------|---------|-----------------------------------------------------------------------------------------------------------------|
| PRLink        | *string | The link to the pull request for the feature branch. Will be null if no pr has been generated.                  |
| PRID          | *string | The ID of the pull request for the feature branch. Will be null if no pr has been generated.                    |
| PRNumber      | *int    | The number of the pull request for the feature branch. Will be null if no pr has been generated.                |
| PRStatus      | *string | The latest known status of the pull request (e.g., `open`, `closed`, `merged`, `draft`). Null if unknown.       |
| PRStatusUpdatedAt | *string | ISO 8601 timestamp for when the pull request status was last updated. Null if unknown.                       |
| FeatureBranch | string  | The name of the feature branch created for the task. This is the branch where the task's code changes are made. |
| TargetBranch  | string  | The name of the target branch for the pull request. This is the branch the feature branch will be merged into.  |


## 18.6 TaskState

TaskState is an enum that defines the current state of a task.

| Value                |
|----------------------|
| Pending              |
| Executing            |
| Awaiting Code Review |
| Completed            |
| Failed               |

# 19. ListTasks

The ListTasks API is used to list all tasks for a tenant, optionally filtered by workstream.

## 19.1 Request

```http request
GET /v1/tenants/{tenant_id}/tasks?workstreamID={workstreamID}&maxResults={maxResults}&token={token}&includeDeleted={includeDeleted} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                                                                     |
|------------------------------------------|----------|---------|-----------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant to list tasks for.                                                                         |
| maxResults                               | query    | *int    | Optional. The maximum number of tasks to return. Default is 10. Must be >=1 and <= 500.                         |
| token                                    | query    | *string | Optional. A token to retrieve the next page of results. If not provided, the first page of results is returned. |
| includeDeleted                           | query    | *bool   | Optional. Whether to include deleted tasks in the results. Default is false.                                    |
| Authorization                            | header   | string  | The authorization header for the request.                                                                       |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                          |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                             |

## 19.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "Tasks": [],
  "NextToken": "*string"
}
```
| Field     | Type                    | Description                                                                                    |
|-----------|-------------------------|------------------------------------------------------------------------------------------------|
| Tasks     | [][Task](#183-response) | A list of tasks for the tenant, filtered by workstream if provided.                            |
| NextToken | *string                 | A token to retrieve the next page of results. If there are no more results, this will be null. |

# 20. GetTask

The GetTask API is used to get a specific task for a tenant.

## 20.1 Request

```http request
GET /v1/tenants/{tenant_id}/tasks/{task_id}?includeDeleted={includeDeleted} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                         |
|------------------------------------------|----------|---------|---------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant to get the task for.                           |
| task_id                                  | path     | string  | The ID of the task to get.                                          |
| includeDeleted                           | query    | *bool   | Optional. Whether to return a deleted tasks. Default is false.      |
| Authorization                            | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.              |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4. |

## 20.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "TenantID": "string",
  "WorkstreamID": "*string",  
  "TaskID": "string", 
  "Title": "string",
  "EnvironmentID": "*string",
  "Prompt": "string",
  "Parallel": bool,
  "Model": "ModelType",
  "AssignedToTenantID": "*string",
  "AssignedToAI" : bool,  
  "RepoInfo: {},
  "LastTurnStatus": "*string",
  "LastTurnIndex": "*int",
  "State": "TaskState",
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "Deleted": bool,
  "Version": int,
  "FileIDs" : [],
  "NewFileIDs" : []
}
```

See [CreateTask](#183-response) for details on the response fields.

# 21. UpdateTask

The UpdateTask API is used to update an existing task for a tenant.

## 21.1 Request

```http request
PATCH /v1/tenants/{tenant_id}/tasks/{task_id} HTTP/1.1
Accept: application/json
Content-Type: application/json; charset=utf-8
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>

{
    "Title": "*string",
    "Prompt": "*string",
    "Model": "*ModelType",
    "RepoInfo" : {},
    "Deleted": *bool,
    "NewFileIDs" : []string
}
```

| Parameter                                | Location | Type                   | Description                                                                                                                                                                                                                                                                                                                                                                   |
|------------------------------------------|----------|------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string                 | The ID of the tenant to update the task for.                                                                                                                                                                                                                                                                                                                                  |
| task_id                                  | path     | string                 | The ID of the task to update.                                                                                                                                                                                                                                                                                                                                                 |
| Authorization                            | header   | string                 | The authorization header for the request.                                                                                                                                                                                                                                                                                                                                     |
| X-Event-Horizon-Delegating-Authorization | header   | *string                | The authorization header for the delegating principal.                                                                                                                                                                                                                                                                                                                        |
| X-Event-Horizon-Signed-Headers           | header   | *string                | The signed headers for the request, when authenticating with Sigv4.                                                                                                                                                                                                                                                                                                           |
| version                                  | header   | string                 | The version of the task to update. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned.                                                                                                                                                                                                                          |
| Title                                    | body     | *string                | If set, update the task's title.                                                                                                                                                                                                                                                                                                                                              |
| Prompt                                   | body     | *string                | If set, update the task's prompt.                                                                                                                                                                                                                                                                                                                                             |
| Model                                    | body     | *ModelType             | If set, update the task's model type.                                                                                                                                                                                                                                                                                                                                         |
| RepoInfo                                 | body     | map[string][*RepoInfo] | If set, update the task's repository info. This tracks branch names and PR links for each repo used in the environment.                                                                                                                                                                                                                                                       |
| Deleted                                  | body     | *bool                  | If set to false, undelete the task. May not be set to true. Use DeleteTask instead.                                                                                                                                                                                                                                                                                           |
| NewFileIDS                               | body     | *[]string              | If set, updates the list of new file IDs to attach to the task. Entries added to this list are added to the FileIDs[] array. Removing an entry from this list communicate to the next iteration of the agent that the file has already been uploaded to the model and does not need to be resent. At most 25 files can be attached to a task, including existing attachments. |

## 21.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "TenantID": "string",
  "WorkstreamID": "*string",  
  "TaskID": "string", 
  "Title": "string",
  "EnvironmentID": "*string",
  "Prompt": "string",
  "Parallel": bool,
  "Model": "*ModelType",
  "AssignedToTenantID": "*string",
  "AssignedToAI" : bool,  
  "RepoInfo: {},
  "State": "TaskState",
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "Deleted": bool,
  "Version": int,
  "TaskNumber": int,
  "FileIDs": []
}
```
See [CreateTask](#183-response) for details on the response fields.

# 22. DeleteTask

The DeleteTask API soft-deletes a task.

## 22.1 Request

```http request
DELETE /v1/tenants/{tenant_id}/tasks/{task_id} HTTP/1.1
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>
```

| Parameter                                | Location | Type    | Description                                                                                                                                          |
|------------------------------------------|----------|---------|------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the task being deleted.                                                                                               |
| task_id                                  | path     | string  | The ID of the task to delete.                                                                                                                        |
| Authorization                            | header   | string  | The authorization header for the request.                                                                                                            |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                                                               |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                                                  |
| version                                  | header   | string  | The version of the task to delete. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned. |

## 22.2 Response

On success a 204 NO CONTENT is returned with no body.

# 23. CreateTurn

CreateTurn creates a new turn for a task. The first turn for a task is created automatically when the tasks becomes
ready for execution. Subsequent turns are created by calling CreateTurn. 

If any of the following are true, a 409 Conflict error is returned:

1. The tasks is not yet executable (e.g. it is blocked on another task, or it's workstream is paused).
2. The task is not assigned to an AI agent.
3. The task has 0 turns.
4. The latest turn on the task is not in a terminal state (i.e. it is not "Done" or "Failed").
5. A turn with the given index already exists.
6. The provided turn index is not the next index in the sequence (i.e. it is not the latest turn index + 1).

## 23.1 Request

```http request
PUT /v1/tenants/{tenant_id}/tasks/{task_id}/turns/{turnIndex} HTTP/1.1
Accept: application/json
Content-Type: application/json; charset=utf-8
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <taskVersion>

{
    "Prompt": "string",
    "WorkstreamID": "*string",
    "AdditionalFileIDs": []
}
```

| Parameter                                | Location | Type     | Description                                                                                                                                                                                                                                    |
|------------------------------------------|----------|----------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string   | The ID of the tenant to create the turn for.                                                                                                                                                                                                   |
| task_id                                  | path     | string   | The ID of the task to create the turn for.                                                                                                                                                                                                     |
| turnIndex                                | path     | int      | The index of the turn to create. This must be the next index in the sequence (i.e. latest turn index + 1).                                                                                                                                     |
| Authorization                            | header   | string   | The authorization header for the request.                                                                                                                                                                                                      |
| X-Event-Horizon-Delegating-Authorization | header   | *string  | The authorization header for the delegating principal.                                                                                                                                                                                         |
| X-Event-Horizon-Signed-Headers           | header   | *string  | The signed headers for the request, when authenticating with Sigv4.                                                                                                                                                                            |
| taskVersion                              | header   | string   | The version of the task to create the turn for. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned. Adding a turn to a task will increment it's version number.                  |
| WorkstreamID                             | body     | *string  | Optional. The ID of the workstream the task belongs to.                                                                                                                                                                                        |
| Prompt                                   | body     | string   | The prompt to use for the turn.                                                                                                                                                                                                                |
| AdditionalFileIDs                        | body     | []string | A list of additional file IDs to attach to the task when creating the turn. These entries are added to both the FileIDs and NewFieldIDs arrays in the task object. At most 25 files can be attached to a turn, including existing attachments. |

## 23.2 Response

On success a 201 CREATED is returned with the following JSON body:

```http request
HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8

{
  "TenantID": "string",
  "WorkstreamID": "*string",
  "TaskID": "string",
  "TurnIndex": int,
  "Prompt": "string",
  "PreviousResponseID": "*string",
  "CommitInfo": {}
  "BaselineCommitHash": "*string",
  "LastCommitHash": "*string",
  "Status": "string",
  "OutputMessage": "*string", 
  "ErrorMessage": "*string"
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "Version": int
  "CompletedAt": "*string"
}
```

| Field              | Type                                     | Description                                                                                                                                                             |
|--------------------|------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| TenantID           | string                                   | The ID of the tenant that owns the turn.                                                                                                                                |
| WorkstreamID       | *string                                  | The ID of the workstream the task belongs to. Null for non-workstream tasks.                                                                                            |
| TaskID             | string                                   | The ID of the task the turn belongs to.                                                                                                                                 |
| TurnIndex          | int                                      | The index of the turn. This is the next index in the sequence (i.e. latest turn index + 1).                                                                             |
| Prompt             | string                                   | The prompt used for the turn.                                                                                                                                           |
| PreviousResponseID | *string                                  | The ID of the previous response for the turn. Used to enable AI to resume with the context of the previous turn.                                                        |
| CommitInfo         | map[string][CommitInfo](#233-commitinfo) | A map of "org/repo" to commit hash info for that repo.                                                                                                                  |  
| BaselineCommitHash | *string                                  | The baseline commit hash of the task.                                                                                                                                   |
| LastCommitHash     | *string                                  | The last commit hash of the task.                                                                                                                                       |
| Status             | string                                   | The status of the turn. This may be arbtirary text set by the agent while it runs. The values "Succeeded", and "Failed" are used to idetnify when turns have completed. |
| OutputMessage      | *string                                  | The output message from the agent. This is the final response from the agent after it has completed its work.                                                           |
| ErrorMessage       | *string                                  | The error message from the agent, if any. This is set if Status == `Failed`.                                                                                            |
| CreatedAt          | string                                   | The timestamp when the turn was created, in ISO 8601 format.                                                                                                            |
| UpdatedAt          | string                                   | The timestamp when the turn was last updated, in ISO 8601 format.                                                                                                       |
| Version            | int                                      | The version of the turn. This is incremented every time the turn is updated.                                                                                            |
| CompletedAt        | *string                                  | The timestamp when the turn was completed, in ISO 8601 format. This is set when the turn's status is set to "Succeeded" or "Failed".                                    |

## 23.3 CommitInfo

```json
{
  "BaselineCommitHash": "*string",
  "LastCommitHash": "*string"
}
```

# 24. ListTurns
The ListTurns API is used to list all turns for a task.

## 24.1 Request

```http request
GET /v1/tenants/{tenant_id}/tasks/{task_id}/turns?workstreamID={workstreamID}&maxResults={maxResults}&token={token}&includeDeleted={includeDeleted} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                                                                     |
|------------------------------------------|----------|---------|-----------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant to list turns for.                                                                         |
| task_id                                  | path     | string  | The ID of the task to list turns for.                                                                           |
| workstreamID                             | query    | *string | Optional. The ID of the workstream the task belongs to.                                                         |
| maxResults                               | query    | *int    | Optional. The maximum number of turns to return. Default is 10. Must be >=1 and <= 500.                         |
| token                                    | query    | *string | Optional. A token to retrieve the next page of results. If not provided, the first page of results is returned. |
| includeDeleted                           | query    | *bool   | Optional. Set to true to return turns for a deleted task.                                                       |
| Authorization                            | header   | string  | The authorization header for the request.                                                                       |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                          |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                             |

## 24.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "Turns": [],
  "NextToken": "*string"
}
```

| Field     | Type                    | Description                                                                                    |
|-----------|-------------------------|------------------------------------------------------------------------------------------------|
| Turns     | [][Turn](#232-response) | A list of turns for the task.                                                                  |
| NextToken | *string                 | A token to retrieve the next page of results. If there are no more results, this will be null. |

# 25. GetTurn

GetTurn retrieves a specific turn for a task.

## 25.1 Request

```http request
GET /v1/tenants/{tenant_id}/tasks/{task_id}/turns/{turnIndex}?workstreamID={workstreamID}&includeDeleted={includeDeleted} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                         |
|------------------------------------------|----------|---------|---------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant to get the turn for.                           |
| task_id                                  | path     | string  | The ID of the task to get the turn for.                             |
| turnIndex                                | path     | int     | The index of the turn to get.                                       |
| workstreamID                             | query    | *string | Optional. The ID of the workstream the task belongs to.             |
| includeDeleted                           | query    | *bool   | Optional. Set to true to return a turn for a deleted task.          |
| Authorization                            | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.              |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4. |

## 25.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "TenantID": "string",
  "TaskID": "string",
  "TurnIndex": int,
  "Prompt": "string",
  "PreviousResponseID": "*string",
  "CommitInfo": {}
  "Status": "string",
  "OutputMessage": "*string", 
  "ErrorMessage": "*string"
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "Version": int,
  "CompletedAt": "*string"
}
```

See [CreateTurn](#232-response) for details on the response fields.

# 26. GetLastTurn

GetLastTurn retrieves the last turn for a task. This is useful for quickly getting the most recent turn without having
to list all turns.

## 26.1 Request

```http request
GET /v1/tenants/{tenant_id}/tasks/{task_id}/turns/last?workstreamID={workstreamID}&includeDeleted={includeDeleted} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                         |
|------------------------------------------|----------|---------|---------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant to get the last turn for.                      |
| task_id                                  | path     | string  | The ID of the task to get the last turn for.                        |
| workstreamID                             | query    | *string | Optional. The ID of the workstream the task belongs to.             |
| includeDeleted                           | query    | *bool   | Optional. Set to true to return the last turn for a deleted task.   |
| Authorization                            | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.              |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4. |

## 26.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "TenantID": "string",
  "TaskID": "string",
  "TurnIndex": int,
  "Prompt": "string",
  "PreviousResponseID": "*string",
  "CommitInfo": {},
  "Status": "string",
  "OutputMessage": "*string", 
  "ErrorMessage": "*string"
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "Version": int,
  "CompletedAt": "*string"
}
```

See [CreateTurn](#232-response) for details on the response fields.

# 26. UpdateTurn

The UpdateTurn API is used to update an existing turn for a task.

## 26.1 Request

```http request
PATCH /v1/tenants/{tenant_id}/tasks/{task_id}/turns/{turnIndex} HTTP/1.1
Accept: application/json
Content-Type: application/json; charset=utf-8
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>

{
    "PreviousResponseID": "*string",
    "CommitInfo": {},    
    "WorkstreamID": "*string",
    "Status": "*string",
    "OutputMessage": "*string", 
    "ErrorMessage": "*string"
}
```

| Parameter                                | Location | Type                                      | Description                                                                                                                                          |
|------------------------------------------|----------|-------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string                                    | The ID of the tenant to update the turn for.                                                                                                         |
| task_id                                  | path     | string                                    | The ID of the task to update the turn for.                                                                                                           |
| turnIndex                                | path     | int                                       | The index of the turn to update.                                                                                                                     |
| Authorization                            | header   | string                                    | The authorization header for the request.                                                                                                            |
| X-Event-Horizon-Delegating-Authorization | header   | *string                                   | The authorization header for the delegating principal.                                                                                               |
| X-Event-Horizon-Signed-Headers           | header   | *string                                   | The signed headers for the request, when authenticating with Sigv4.                                                                                  |
| version                                  | header   | string                                    | The version of the turn to update. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned. |
| PreviousResponseID                       | body     | *string                                   | If set, update the turn's previous response ID.                                                                                                      |
| CommitInfo                               | body     | *map[string][CommitInfo](#233-commitinfo) | If set, update the turn's commit info. This is a map of "org/repo" to commit hash info for that repo.                                                |
| WorkstreamID                             | body     | *string                                   | Optional. The ID of the workstream the task belongs to.                                                                                               |
| Status                                   | body     | *string                                   | If set, update the turn's status.                                                                                                                    |
| OutputMessage                            | body     | *string                                   | If set, update the turn's output message.                                                                                                            |
| ErrorMessage                             | body     | *string                                   | If set, update the turn's error message.                                                                                                             |

## 26.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "TenantID": "string",
  "TaskID": "string",
  "TurnIndex": int,
  "Prompt": "string",
  "PreviousResponseID": "*string",
  "CommitInfo": {},
  "Status": "string",
  "OutputMessage": "*string", 
  "ErrorMessage": "*string"
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "Version": int,
  "CompletedAt": "*string"
}
```
See [CreateTurn](#232-response) for details on the response fields.

# 27. UploadTurnLogs

The UploadTurnLogs API is used to upload a batch logs for a turn.

The requested is limited to a maximum of 500 logs and a maximum of 1MB in size. If the request exceeds these limits, 
a `413 Content Too Large` error is returned.

NOTE: Logs cannot be uploaded to a turn that is in a terminal state (i.e. "Done" or "Failed"). In that case a 409
conflict will be returned.

## 27.1 Request

```http request
POST /v1/tenants/{tenant_id}/tasks/{task_id}/turns/{turnIndex}/logs HTTP/1.1
Accept: application/json
Content-Type: application/json; charset=utf-8
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>

{
    "Index": int,
    "WorkstreamID": "*string",
    "Logs": [
        {
            "Timestamp": "string",
            "Message": "string"
        }
    ]
}
```

| Parameter                                | Location | Type    | Description                                                                                                                                                   |
|------------------------------------------|----------|---------|---------------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant to upload logs for.                                                                                                                      |
| task_id                                  | path     | string  | The ID of the task to upload logs for.                                                                                                                        |
| turnIndex                                | path     | int     | The index of the turn to upload logs for.                                                                                                                     |
| Authorization                            | header   | string  | The authorization header for the request.                                                                                                                     |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                                                                        |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                                                           |
| version                                  | header   | string  | The version of the turn to upload logs for. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned. |
| WorkstreamID                             | body     | *string | Optional. The ID of the workstream the task belongs to.                                                                                                       |
| Index                                    | body     | int     | The log index of the first entry in the log batch. This should be the last index + 1 of the previous log batch, or 0 for the first batch.                     |
| Logs                                     | body     | []Log   | The list of logs to upload. Each log entry should have a timestamp and message.                                                                               |

## 27.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "Version": int
}
```

| Field   | Type | Description                                                            |
|---------|------|------------------------------------------------------------------------|
| Version | int  | The incremented version of the turn after the logs have been uploaded. |

# 28. StreamTurnLogs

StreamTurns streams logs for a turn using Server-Sent Events (SSE). 

## 28.1 Request

```http request
GET /v1/tenants/{tenant_id}/tasks/{task_id}/turns/{turnIndex}/logs?workstreamID={workstreamID}&includeDeleted={includeDeleted} HTTP/1.1
Last-Event-ID: <last-event-id>
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
Accept: text/event-streamS
```

| Parameter                                | Location | Type    | Description                                                                                       |
|------------------------------------------|----------|---------|---------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant to stream logs for.                                                          |
| task_id                                  | path     | string  | The ID of the task to stream logs for.                                                            |
| turnIndex                                | path     | int     | The index of the turn to stream logs for.                                                         |
| workstreamID                             | query    | *string | Optional. The ID of the workstream the task belongs to.                                           |
| includeDeleted                           | query    | *bool   | Optional. Set to true to return logs for turns on deleted tasks. |
| Last-Event-ID                            | header   | *string | Optional. The last event ID received by the client. Used to resume streaming from the last event. |
| Authorization                            | header   | string  | The authorization header for the request.                                                         |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                            |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                               |

## 28.2 Response

If there are no more logs to stream, a 204 NO CONTENT is returned with no body. Otherwise, on success a 200 OK is returned.
The response is formatted as an SSE stream.

```http response
HTTP/1.1 200 OK
Content-Type: text/event-stream; charset=utf-8

event: log
data : {}
id: 1
retry: 1000

event: log
data : {}
id: 2
retry: 1000

...
```

| Field | Type   | Description                                 |
|-------|--------|---------------------------------------------|
| event | string | The event type. This is always "log".       |
| data  | string | Json data encoding a [Log entry](#283-log). |
| id    | int    | The event ID.                               |
| retry | *int   | The retry interval in milliseconds.         |

## 28.3 Log

```json
{
  "Timestamp": "string", 
  "Message": "string"
}
```

# 29. GetLastTurnLog

GetLastTurnLog retrieves the last log entry for a turn. 

## 29.1 Request

```http request
GET /v1/tenants/{tenant_id}/tasks/{task_id}/turns/{turnIndex}/logs/last?workstreamID={workstreamID}&includeDeleted={includeDeleted} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                                                       |
|------------------------------------------|----------|---------|---------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant to get the last log for.                                                     |
| task_id                                  | path     | string  | The ID of the task to get the last log for.                                                       |
| turnIndex                                | path     | int     | The index of the turn to get the last log for.                                                    |
| workstreamID                             | query    | *string | Optional. The ID of the workstream the task belongs to.                                           |
| includeDeleted                           | query    | *bool   | Optional. Set to true to return logs for turns on deleted tasks. |
| Authorization                            | header   | string  | The authorization header for the request.                                                         |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                            |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                               |
## 29.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "Index": int,
  "Timestamp": "string",
  "Message": "string"
}
```

| Field     | Type   | Description                                              |
|-----------|--------|----------------------------------------------------------|
| Index     | int    | The index of the last log entry.                         |
| Timestamp | string | The timestamp of the last log entry, in ISO 8601 format. |
| Message   | string | The message of the last log entry.                       |

# 30. AddGithubOrg

AddGithubOrg adds a github org and installation id to the service.

## 30.1 Request

```http request
PUT /v1/github/orgs/{org_id}
HTTP/1.1
Content-Type: application/json; charset=utf-8
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>

{
    "OrgName": "string",
    "ExternalOrgID": "int",
    "InstallationID": int,    
}
```

| Parameter                      | Location | Type    | Description                                                         |
|--------------------------------|----------|---------|---------------------------------------------------------------------|
| org_id                         | path     | string  | The ID of the github org to add. This must be a v4 UUID.            |
| Authorization                  | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Signed-Headers | header   | *string | The signed headers for the request, when authenticating with Sigv4. |
| OrgName                        | body     | string  | The name of the github org to add.                                  |
| ExternalOrgID                  | body     | int     | The ID of the github org in Github.                                 |
| InstallationID                 | body     | int     | The installation ID of the github app for the org.                  |

## 30.2 Response

On success a 201 CREATED is returned with the following JSON body:

```http request
HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8

{
  "OrgID": "string",
  "OrgName": "string",
  "ExternalOrgID": int,
  "InstallationID": int,
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "Version": int,
  "Deleted": bool
}
```

| Field          | Type   | Description                                                                |
|----------------|--------|----------------------------------------------------------------------------|
| OrgID          | string | The ID of the github org.                                                  |
| OrgName        | string | The name of the github org.                                                |
| ExternalOrgID  | int    | The ID of the github org in Github.                                        |
| InstallationID | int    | The installation ID of the github app for the org.                         |
| CreatedAt      | string | The timestamp when the org was created, in ISO 8601 format.                |
| UpdatedAt      | string | The timestamp when the org was last updated, in ISO 8601 format.           |
| Version        | int    | The version of the org. This is incremented every time the org is updated. |
| Deleted        | bool   | Whether the org has been deleted.                                          |

# 31. ListGithubOrgs

ListGithubOrgs lists all github orgs in the service. When the optional `name`
query parameter is provided, only the GitHub org whose name matches the
supplied value exactly is returned. If no org name matches, the response is
empty.

## 31.1 Request

```http request
GET /v1/github/orgs?maxResults={maxResults}&token={token}&name={name}&includeDeleted={includeDeleted} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                      | Location | Type    | Description                                                                                                                                      |
|--------------------------------|----------|---------|--------------------------------------------------------------------------------------------------------------------------------------------------|
| maxResults                     | query    | *int    | Optional. The maximum number of orgs to return. Default is 10. Must be >=1 and <= 500.                                                           |
| token                          | query    | *string | Optional. A token to retrieve the next page of results. If not provided, the first page of results is returned.                                  |
| name                           | query    | *string | Optional. When provided, only the GitHub org whose name matches the value exactly is returned. If no org matches, the response contains no orgs. |
| includeDeleted                 | query    | *bool   | Optional. Set to true to return deleted orgs.                                                                                                    |
| Authorization                  | header   | string  | The authorization header for the request.                                                                                                        |
| X-Event-Horizon-Signed-Headers | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                                              |

## 31.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "Orgs": [],
  "NextToken": "*string"
}
```

| Field     | Type                         | Description                                                                                     |
|-----------|------------------------------|-------------------------------------------------------------------------------------------------|
| Orgs      | [][GithubOrg](#302-response) | A list of github orgs.                                                                          |
| NextToken | *string                      | A token to retrieve the next page of results. If there are no more results, this will be null.  |

# 32. GetGithubOrg

GetGithubOrg retrieves a specific github org by ID.

## 32.1 Request

```http request
GET /v1/github/orgs/{org_id}?includeDeleted={includeDeleted} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                      | Location | Type    | Description                                                         |
|--------------------------------|----------|---------|---------------------------------------------------------------------|
| org_id                         | path     | string  | The ID of the github org to get. This must be a v4 UUID.            |
| includeDeleted                 | query    | *bool   | Optional. Set to true to return a deleted org.                      |
| Authorization                  | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Signed-Headers | header   | *string | The signed headers for the request, when authenticating with Sigv4. |

## 32.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "OrgID": "string",
  "OrgName": "string",
  "InstallationID": int,
  "ExternalOrgID": int,
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "Version": int,
  "Deleted": bool
}
```

| Field          | Type   | Description                                                                |
|----------------|--------|----------------------------------------------------------------------------|
| OrgID          | string | The ID of the github org.                                                  |
| OrgName        | string | The name of the github org.                                                |
| InstallationID | int    | The installation ID of the github app for the org.                         |
| CreatedAt      | string | The timestamp when the org was created, in ISO 8601 format.                |
| UpdatedAt      | string | The timestamp when the org was last updated, in ISO                        |
| Version        | int    | The version of the org. This is incremented every time the org is updated. |
| Deleted        | bool   | Whether the org has been deleted.                                          |

# 33. UpdateGithubOrg

The UpdateGithubOrg API is used to update an existing github org.

## 33.1 Request

```http request
PATCH /v1/github/orgs/{org_id} HTTP/1.1
Accept: application/json
Content-Type: application/json; charset=utf-8
Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>

{
    "OrgName": "*string",
    "InstallationID": "*int",
    "Deleted":  "*bool"  
}
```

| Parameter                      | Location | Type    | Description                                                                                                                                         |
|--------------------------------|----------|---------|-----------------------------------------------------------------------------------------------------------------------------------------------------|
| org_id                         | path     | string  | The ID of the github org to update.                                                                                                                 |
| Authorization                  | header   | string  | The authorization header for the request.                                                                                                           |
| X-Event-Horizon-Signed-Headers | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                                                 |
| version                        | header   | string  | The version of the org to update. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned. |
| OrgName                        | body     | *string | If set, update the org's name.                                                                                                                      |
| InstallationID                 | body     | *int    | If set, update the org's installation ID.                                                                                                           |
| Deleted                        | body     | *bool   | If set to false, undelete the org.                                                                                                                  |

## 33.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "OrgID": "string",
  "OrgName": "string",
  "InstallationID": int,
  "ExternalOrgID": int,
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "Version": int,
  "Deleted": bool
}
```

| Field          | Type   | Description                                                                |
|----------------|--------|----------------------------------------------------------------------------|
| OrgID          | string | The ID of the github org.                                                  |
| OrgName        | string | The name of the github org.                                                |
| InstallationID | int    | The installation ID of the github app for the org.                         |
| ExternalOrgID  | int    | The ID of the github org in Github.                                        |
| CreatedAt      | string | The timestamp when the org was created, in ISO 8601 format.                |
| UpdatedAt      | string | The timestamp when the org was last updated, in ISO                        |
| Version        | int    | The version of the org. This is incremented every time the org is updated. |
| Deleted        | bool   | Whether the org has been deleted.                                          |

# 34. DeleteGithubOrg

DeleteGithubOrg soft deletes a github org from the service.

## 34.1 Request

```http request
DELETE /v1/github/orgs/{org_id} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>
```

| Parameter                      | Location | Type    | Description                                                                                                                                         |
|--------------------------------|----------|---------|-----------------------------------------------------------------------------------------------------------------------------------------------------|
| org_id                         | path     | string  | The ID of the github org to delete.                                                                                                                 |
| Authorization                  | header   | string  | The authorization header for the request.                                                                                                           |
| X-Event-Horizon-Signed-Headers | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                                                 |
| version                        | header   | string  | The version of the org to delete. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned. |

## 34.2 Response

On success a 204 NO CONTENT is returned with no body.

# 35. CreateWorkstream 

CreateWorkstream creates a new workstream for the given tenant.

## 35.1 Request

```http request
PUT /v1/tenants/{tenant_id}/workstreams/{workstream_id} HTTP/1.1
Accept: application/json
Content-Type: application/json; charset=utf-8
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>

{
    "Name": "string",
    "Description": "string"
    "DefaultShortName": "string",
}
```

| Parameter                                | Location | Type    | Description                                                                 |
|------------------------------------------|----------|---------|-----------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant to create the workstream for.                          |
| workstream_id                            | path     | string  | The ID of the workstream to create. This must be a v4 UUID.                 |
| Authorization                            | header   | string  | The authorization header for the request.                                   |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                      |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.         |
| Name                                     | body     | string  | The name of the workstream.                                                 |
| Description                              | body     | string  | The description of the workstream.                                          |
| DefaultShortName                         | body     | string  | Optional. A default short name to use for tasks created in this workstream. |

## 35.2 Response
On success a 201 CREATED is returned with the following JSON body:

```http request
HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8

{
  "WorkstreamID": "string",
  "TenantID": "string",
  "Name": "string",
  "Description": "string",
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "Version": int,
  "Paused": bool,
  "Deleted": bool,
  "DefaultShortName": "string",
  "TaskCounter": int,
}
```

| Field            | Type   | Description                                                                              |
|------------------|--------|------------------------------------------------------------------------------------------|
| WorkstreamID     | string | The ID of the workstream.                                                                |
| TenantID         | string | The ID of the tenant that owns the workstream.                                           |
| Name             | string | The name of the workstream.                                                              |
| Description      | string | The description of the workstream.                                                       |
| CreatedAt        | string | The timestamp when the workstream was created, in ISO 8601 format.                       |
| UpdatedAt        | string | The timestamp when the workstream was last updated, in ISO 8601 format.                  |
| Version          | int    | The version of the workstream. This is incremented every time the workstream is updated. |
| Paused           | bool   | Whether the workstream is paused. Defaults to true for new workstreams.                  |
| Deleted          | bool   | Whether the workstream has been deleted.                                                 |
| DefaultShortName | string | The default short name to use for tasks created in this workstream.                      |
| TaskCounter      | int    | The counter used to generate unique short names for tasks in this workstream.            |

# 36. ListWorkstreams

ListWorkstreams lists all workstreams for a given tenant.

## 36.1 Request

```http request
GET /v1/tenants/{tenant_id}/workstreams?maxResults={maxResults}&token={token}&includeDeleted={includeDeleted}&shortName={shortName} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                                                                                           |
|------------------------------------------|----------|---------|---------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant to list workstreams for.                                                                                         |
| maxResults                               | query    | *int    | Optional. The maximum number of workstreams to return. Default is 10. Must be >=1 and <= 500.                                         |
| token                                    | query    | *string | Optional. A token to retrieve the next page of results. If not provided, the first page of results is returned.                       |
| includeDeleted                           | query    | *bool   | Optional. Set to true to include deleted workstreams in the results.                                                                  |
| shortName                                | query    | *string | Optional. When set, returns for the workstream with the provided short name. If no such workstream exists, an empty list is returned. |
| Authorization                            | header   | string  | The authorization header for the request.                                                                                             |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                                                |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                                   |

## 36.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "Workstreams": [],
  "NextToken": "*string"
}
```

| Field       | Type                          | Description                                                                                    |
|-------------|-------------------------------|------------------------------------------------------------------------------------------------|
| Workstreams | [][Workstream](#362-response) | A list of workstreams for the tenant.                                                          |
| NextToken   | *string                       | A token to retrieve the next page of results. If there are no more results, this will be null. |

# 37. GetWorkstream

GetWorkstream retrieves a workstream by ID for a given tenant.

## 37.1 Request

```http request
GET /v1/tenants/{tenant_id}/workstreams/{workstream_id}?includeDeleted={includeDeleted} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                         |
|------------------------------------------|----------|---------|---------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant to get the workstream for.                     |
| workstream_id                            | path     | string  | The ID of the workstream to get.                                    |
| includeDeleted                           | query    | *bool   | Optional. Set to true to return a deleted workstream.               |
| Authorization                            | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.              |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4. |

## 37.2 Response
On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "WorkstreamID": "string",
  "TenantID": "string",
  "Name": "string",
  "Description": "string",
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "Version": int,
  "Paused": bool,
  "Deleted": bool,
  "DefaultShortName": "string",
  "TaskCounter": int
}
```

See the [Workstream](#362-response) type for field descriptions.

# 38. UpdateWorkstream
UpdateWorkstream updates a workstream for a given tenant.

## 38.1 Request

```http request
PATCH /v1/tenants/{tenant_id}/workstreams/{workstream_id} HTTP/1.1
Accept: application/json
Content-Type: application/json; charset=utf-8
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>

{
    "Name": "*string",
    "Description": "*string",
    "Paused": "*bool",
    "Deleted": "*bool",
    "DefaultShortName": "*string"
}
```

| Parameter                                | Location | Type    | Description                                                                                                                                               |
|------------------------------------------|----------|---------|-----------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant to update the workstream for.                                                                                                        |
| workstream_id                            | path     | string  | The ID of the workstream to update.                                                                                                                       |
| Authorization                            | header   | string  | The authorization header for the request.                                                                                                                 |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                                                                    |
| X-Event-Horizon-Signed-Headers           | header   | *string |                                                                                                                                                           | The signed headers for the request, when authenticating with Sigv4.                                                                                         |
| version                                  | header   | string  | The version of the workstream to update. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned |
| Name                                     | body     | *string | If set, update the name of the workstream.                                                                                                                |
| Description                              | body     | *string | If set, update the description of the workstream.                                                                                                         |
| Paused                                   | body     | *bool   | If set, update whether the workstream is paused.                                                                                                          |
| Deleted                                  | body     | *bool   | If set to false, undelete the workstream.                                                                                                                 |
| DefaultShortName                         | body     | *string | If set, update the default short name to use for tasks created in this workstream.                                                                        |

## 38.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "WorkstreamID": "string",
  "TenantID": "string",
  "Name": "string",
  "Description": "string",
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "Version": int,
  "Paused": bool,
  "Deleted": bool
}
```

See the [Workstream](#362-response) type for field descriptions.

# 39. DeleteWorkstream
DeleteWorkstream soft deletes a workstream for a given tenant.

## 39.1 Request

```http request
DELETE /v1/tenants/{tenant_id}/workstreams/{workstream_id} HTTP/1.1
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>
```

| Parameter                                | Location | Type    | Description                                                                                                                                               |
|------------------------------------------|----------|---------|-----------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant to delete the workstream for.                                                                                                        |
| workstream_id                            | path     | string  | The ID of the workstream to delete.                                                                                                                       |
| Authorization                            | header   | string  | The authorization header for the request.                                                                                                                 |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                                                                    |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                                                       |
| version                                  | header   | string  | The version of the workstream to delete. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned |

## 39.2 Response

On success a 204 NO CONTENT is returned with no body.

# 40. CreateFeatureFlag

CreateFeatureFlag is an admin api that creates a new feature flag.

## 40.1 Request

```http request
PUT /v1/featureflags/{flag_name} HTTP/1.1
Accept: application/json
Content-Type: application/json; charset=utf-8
Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>

{
    "Description": "string",
    "DefaultPct" " float
}
```

| Parameter                      | Location | Type    | Description                                                                                          |
|--------------------------------|----------|---------|------------------------------------------------------------------------------------------------------|
| flag_name                      | path     | string  | The name of the feature flag to create.                                                              |
| Authorization                  | header   | string  | The authorization header for the request.                                                            |
| X-Event-Horizon-Signed-Headers | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                  | 
| Description                    | body     | string  | The description of the feature flag.                                                                 |
| DefaultPct                     | body     | float   | The default percentage of users that will have the feature flag enabled. Must be between 0.0 and 1.0 |

## 41.2 Response
On success a 201 CREATED is returned with the following JSON body:

```http request
HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8

{
  "Name": "string",
  "Description": "string",
  "DefaultPct": float,
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "Version": int,
  "Deleted": bool
}
```

| Field       | Type   | Description                                                                                  |
|-------------|--------|----------------------------------------------------------------------------------------------|
| Name        | string | The name of the feature flag.                                                                |
| Description | string | The description of the feature flag.                                                         |
| DefaultPct  | float  | The default percentage of users that will have the feature flag enabled.                     |
| CreatedAt   | string | The timestamp when the feature flag was created, in ISO 8601 format.                         |
| UpdatedAt   | string | The timestamp when the feature flag was last updated, in ISO 8601 format.                    |
| Version     | int    | The version of the feature flag. This is incremented every time the feature flag is updated. |
| Deleted     | bool   | Whether the feature flag has been deleted.                                                   |

# 42. CreateFeatureFlagOverride

CreateFeatureFlagOverride creates a new override for a feature flag for a specific tenant.

## 42.1 Request

```http request
PUT /v1/tenants/{tenant_id}/featureFlagOverrides/{flagName} HTTP/1.1
Accept: application/json
Content-Type: application/json; charset=utf-8
Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>

{
    "Enabled": bool
}
```

| Parameter                      | Location | Type    | Description                                                         |
|--------------------------------|----------|---------|---------------------------------------------------------------------|
| tenant_id                      | path     | string  | The ID of the tenant to create the override for.                    |
| flag_name                      | path     | string  | The name of the feature flag to create the override for.            |
| Authorization                  | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Signed-Headers | header   | *string | The signed headers for the request, when authenticating with Sigv4. |
| Enabled                        | body     | bool    | Whether the feature flag is enabled for the tenant.                 |

## 43.2 Response
On success a 201 CREATED is returned with the following JSON body:

```http request
HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8

{
  "FlagName": "string",
  "TenantID": "string",
  "Enabled": bool,
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "Version": int,
  "Deleted": bool
}
```

| Field     | Type   | Description                                                                          |
|-----------|--------|--------------------------------------------------------------------------------------|
| FlagName  | string | The name of the feature flag.                                                        |
| TenantID  | string | The ID of the tenant.                                                                |
| Enabled   | bool   | Whether the feature flag is enabled for the tenant.                                  |
| CreatedAt | string | The timestamp when the override was created, in ISO 8601 format.                     |
| UpdatedAt | string | The timestamp when the override was last updated, in ISO 8601 format.                |
| Version   | int    | The version of the override. This is incremented every time the override is updated. |
| Deleted   | bool   | Whether the override has been deleted.                                               |

# 44. GetTenantFeatureFlags

GetTenantFeatureFlags returns the values of all active feature flags for a given tenant.

## 44.1 Request

```http request
GET /v1/tenants/{tenant_id}/featureflags HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                         |
|------------------------------------------|----------|---------|---------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant to get feature flags for.                      |
| Authorization                            | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.              |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4. |

## 44.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "FeatureFlags": {}
}
```

| Field        | Type            | Description                                                                  |
|--------------|-----------------|------------------------------------------------------------------------------|
| FeatureFlags | map[string]bool | A map of feature flag names to their enabled/disabled status for the tenant. |

# 45. ListFeatureFlags

ListFeatureFlags is an admin api that lists all feature flags.

## 45.1 Request

```http request
GET /v1/featureflags?maxResults={maxResults}&token={token}&includeDeleted={includeDeleted} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                      | Location | Type    | Description                                                                                                     |
|--------------------------------|----------|---------|-----------------------------------------------------------------------------------------------------------------|
| maxResults                     | query    | *int    | Optional. The maximum number of feature flags to return. Default is 10. Must be >=1 and <= 500.                 |
| token                          | query    | *string | Optional. A token to retrieve the next page of results. If not provided, the first page of results is returned. |
| includeDeleted                 | query    | *bool   | Optional. Set to true to include deleted feature flags in the results.                                          |
| Authorization                  | header   | string  | The authorization header for the request.                                                                       |
| X-Event-Horizon-Signed-Headers | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                             |

## 45.2 Response
On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "FeatureFlags": [],
  "NextToken": "*string"
}
```

| Field        | Type                           | Description                                                                                    |
|--------------|--------------------------------|------------------------------------------------------------------------------------------------|
| FeatureFlags | [][FeatureFlag](#412-response) | A list of feature flags.                                                                       |
| NextToken    | *string                        | A token to retrieve the next page of results. If there are no more results, this will be null. |

# 46. GetFeatureFlag

GetFeatureFlag is an admin api that retrieves a feature flag by name.

## 46.1 Request

```http request
GET /v1/featureflags/{flag_name}?includeDeleted={includeDeleted} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                      | Location | Type    | Description                                                         |
|--------------------------------|----------|---------|---------------------------------------------------------------------|
| flag_name                      | path     | string  | The name of the feature flag to retrieve.                           |
| includeDeleted                 | query    | *bool   | Optional. Set to true to return a deleted feature flag.             |
| Authorization                  | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Signed-Headers | header   | *string | The signed headers for the request, when authenticating with Sigv4. |

## 46.2 Response
On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "Name": "string",
  "Description": "string",
  "DefaultPct": float,
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "Version": int,
  "Deleted": bool
}
```

See the [FeatureFlag](#412-response) type for field descriptions.

# 47. UpdateFeatureFlag

UpdateFeatureFlag is an admin api that updates a feature flag.

## 47.1 Request

```http request
PATCH /v1/featureflags/{flag_name} HTTP/1.1
Accept: application/json
Content-Type: application/json; charset=utf-8
Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>

{
    "Description": "*string",
    "DefaultPct": "*float",
    "Deleted": "*bool"
}
```

| Parameter                      | Location | Type    | Description                                                                                                                                                 |
|--------------------------------|----------|---------|-------------------------------------------------------------------------------------------------------------------------------------------------------------|
| flag_name                      | path     | string  | The name of the feature flag to update.                                                                                                                     |
| Authorization                  | header   | string  | The authorization header for the request.                                                                                                                   |
| X-Event-Horizon-Signed-Headers | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                                                         |
| version                        | header   | string  | The version of the feature flag to update. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned |
| Description                    | body     | *string | If set, update the description of the feature flag.                                                                                                         |
| DefaultPct                     | body     | *float  | If set, update the default percentage of users that will have the feature flag enabled. Must be between 0.0 and 1.0.                                        |
| Deleted                        | body     | *bool   | If set to false, undelete the feature flag.                                                                                                                 |

## 47.2 Response
On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "Name": "string",
  "Description": "string",
  "DefaultPct": float,
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "Version": int,
  "Deleted": bool
}
```

See the [FeatureFlag](#412-response) type for field descriptions.

# 48. DeleteFeatureFlag

DeleteFeatureFlag is an admin api that soft deletes a feature flag.

## 48.1 Request

```http request
DELETE /v1/featureflags/{flag_name} HTTP/1.1
Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>
```

| Parameter                      | Location | Type    | Description                                                                                                                                                 |
|--------------------------------|----------|---------|-------------------------------------------------------------------------------------------------------------------------------------------------------------|
| flag_name                      | path     | string  | The name of the feature flag to delete.                                                                                                                     |
| Authorization                  | header   | string  | The authorization header for the request.                                                                                                                   |
| X-Event-Horizon-Signed-Headers | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                                                         |
| version                        | header   | string  | The version of the feature flag to delete. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned |

## 48.2 Response
On success a 204 NO CONTENT is returned with no body.

# 49. ListFeatureFlagOverrides

ListFeatureFlagOverrides is an admin api that lists all feature flag overrides for a given tenant.

## 49.1 Request

```http request
GET /v1/tenants/{tenant_id}/featureFlagOverrides?maxResults={maxResults}&token={token}&includeDeleted={includeDeleted} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                      | Location | Type    | Description                                                                                                     |
|--------------------------------|----------|---------|-----------------------------------------------------------------------------------------------------------------|
| tenant_id                      | path     | string  | The ID of the tenant to list feature flag overrides for.                                                        |
| maxResults                     | query    | *int    | Optional. The maximum number of feature flag overrides to return. Default is 10. Must be >=1 and <= 500.        |
| token                          | query    | *string | Optional. A token to retrieve the next page of results. If not provided, the first page of results is returned. |
| includeDeleted                 | query    | *bool   | Optional. Set to true to include deleted feature flag overrides in the results.                                 |
| Authorization                  | header   | string  | The authorization header for the request.                                                                       |
| X-Event-Horizon-Signed-Headers | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                             |

## 49.2 Response
On success a 200 OK is returned with the following JSON body:
```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "FeatureFlagOverrides": [],
  "NextToken": "*string"
}
```

| Field                | Type                                   | Description                                                                                    |
|----------------------|----------------------------------------|------------------------------------------------------------------------------------------------|
| FeatureFlagOverrides | [][FeatureFlagOverride](#432-response) | A list of feature flag overrides for the tenant.                                               |
| NextToken            | *string                                | A token to retrieve the next page of results. If there are no more results, this will be null. |

# 50. GetFeatureFlagOverride
GetFeatureFlagOverride retrieves a feature flag override for a given tenant.

## 50.1 Request

```http request
GET /v1/tenants/{tenant_id}/featureFlagOverrides/{flag_name}?includeDeleted={includeDeleted} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                      | Location | Type    | Description                                                         |
|--------------------------------|----------|---------|---------------------------------------------------------------------|
| tenant_id                      | path     | string  | The ID of the tenant to get the feature flag override for.          |
| flag_name                      | path     | string  | The name of the feature flag to get the override for.               |
| includeDeleted                 | query    | *bool   | Optional. Set to true to return a deleted feature flag override.    |
| Authorization                  | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Signed-Headers | header   | *string | The signed headers for the request, when authenticating with Sigv4. |


## 50.2 Response
On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "FlagName": "string",
  "TenantID": "string",
  "Enabled": bool,
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "Version": int,
  "Deleted": bool
}
```

See the [FeatureFlagOverride](#432-response) type for field descriptions.

# 51. UpdateFeatureFlagOverride
UpdateFeatureFlagOverride is an admin api that updates a feature flag override for a given tenant.

## 51.1 Request

```http request
PATCH /v1/tenants/{tenant_id}/featureFlagOverrides/{flag_name} HTTP/1.1
Accept: application/json
Content-Type: application/json; charset=utf-8
Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>

{
    "Enabled": "*bool",
    "Deleted": "*bool"
}
```

| Parameter                      | Location | Type    | Description                                                                                                                                                          |
|--------------------------------|----------|---------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                      | path     | string  | The ID of the tenant to update the feature flag override for.                                                                                                        |
| flag_name                      | path     | string  | The name of the feature flag to update the override for.                                                                                                             |
| Authorization                  | header   | string  | The authorization header for the request.                                                                                                                            |
| X-Event-Horizon-Signed-Headers | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                                                                  |
| version                        | header   | string  | The version of the feature flag override to update. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned |
| Enabled                        | body     | *bool   | If set, update whether the feature flag is enabled for the tenant.                                                                                                   |
| Deleted                        | body     | *bool   | If set to false, undelete the feature flag override.                                                                                                                 |

## 51.2 Response
On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "FlagName": "string",
  "TenantID": "string",
  "Enabled": bool,
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "Version": int,
  "Deleted": bool
}
```

See the [FeatureFlagOverride](#432-response) type for field descriptions.

# 52. DeleteFeatureFlagOverride
DeleteFeatureFlagOverride is an admin override that soft deletes a feature flag override for a given tenant.

## 52.1 Request

```http request
DELETE /v1/tenants/{tenant_id}/featureFlagOverrides/{flag_name} HTTP/1.1
Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>
```

| Parameter                      | Location | Type    | Description                                                                                                                                                          |
|--------------------------------|----------|---------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                      | path     | string  | The ID of the tenant to delete the feature flag override for.                                                                                                        |
| flag_name                      | path     | string  | The name of the feature flag to delete the override for.                                                                                                             |
| Authorization                  | header   | string  | The authorization header for the request.                                                                                                                            |
| X-Event-Horizon-Signed-Headers | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                                                                  |
| version                        | header   | string  | The version of the feature flag override to delete. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned |

## 52.2 Response
On success a 204 NO CONTENT is returned with no body.

# 53. Using Feature Flags

The API service will cache the data returned by GetTenantFeatureFlags for up to 5 minutes. This means that seeing changes
to feature flags may take up to 5 minutes to be reflected everywhere. 

We want to avoid situations where different API service instances or the API service and the UI service see different
values for feature flags at the same time, as that can lead to inconsistent behavior.

To prevent this, we will allow callers to pass in feature flag values via the "X-EventHorizon-FeatureFlags" header.
This allows the UI service to pass in the same feature flag values it is using into the API service, ensuring consistent
behavior.

The UI will store feature flags in a cookie, so that they are available to javascript code running in the browser. They
wil also be sent to the UI service on each request, so that the UI service can pass them into the API service as needed.

If no cookie value is present, the browser will make a call to the GetTenantFeatureFlags API to fetch them, and will then
store them in the cookie for future requests.

The UI will use 5 min cookie expiration times, to match the API service cache TTL.

> NOTE: Feature flags can be overridden by the browser. This is needed to enable distributed consistency. Thus, even
> though no customer has permission to add feature flag overrides, they can modify cookies, or set the
> X-EventHorizon-FeatureFlags header in api requests. This means that feature flags are not "secure" and should not be
> used to make authorization or authentication decisions. They are only used to enable or disable experimental features,
> or to perform A/B experiments. 

## 53.1 Adding a new feature flag

```bash
p42-ctl feature-flag add -f <flag-name> -D <description> -p <default-pct>
```

This will create a new feature flag with the given name, description and default percentage. The flag name must be
unique. The default percentage must be between 0.0 and 1.0.

## 53.2 Explicitly enabling a feature flag for a tenant

```bash
p42-ctl feature-flag override -i <tenant-id> -f <flag-name> -e
```

This will create a new feature flag override for the given tenant and flag name. If an override already exists, it will
be updated. 

## 53.3 Explicitly disabling a feature flag for a tenant

```bash
p42-ctl feature-flag override -t <tenant-id> -f <flag-name>
```

This adds an override, but doesn't pass "-e / --enable".

## 53.4 Removing an explicit override for a tenant

```bash
p42-ctl feature-flag delete-override -t <tenant-id> -f <flag>
```

This will delete the feature flag override for the given tenant and flag name. After this, the feature flag will be
enabled or disabled based on the default percentage and the tenant ID hash.

## 53.5 Deleting a feature flag

```bash
p42-ctl feature-flag delete -f <flag-name>
```

## 53.6 Undeleting a feature flag

```bash
p42-ctl feature-flag update -f <flag-name> <<EOF
{
  "Deleted": false
}
EOF
```

## 53.7 Updating a feature flag default percentage

```bash
p42-ctl feature-flag update -f <flag-name> <<EOF
{
  "DefaultPct": 0.25
}
EOF
```

You can also combine undelete / default percentage updates in a single call.

```bash
p42-ctl feature-flag update -f <flag-name> <<EOF
{
  "DefaultPct": 0.25,
  "Deleted": false
}
EOF
```

## 53.8 Getting effective feature flags for a tenant

```bash
p42-ctl feature-flag get-tenant-flags -t <tenant-id>
```

# 54. AddWorkstreamShortName

The AddWorkstreamShortName API adds a short name to a workstream.

## 54.1 Request

```http request
PUT /v1/tenants/{tenant_id}/workstreams/{workstream_id}/shortnames/{name} HTTP/1.1
Content-Type: application/json; charset=utf-8
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>
```

| Parameter                                | Location | Type    | Description                                                                                                                                               |
|------------------------------------------|----------|---------|-----------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the workstream.                                                                                                            |
| workstream_id                            | path     | string  | The ID of the workstream to add the short name to.                                                                                                        |
| name                                     | path     | string  | The short name to add to the workstream.                                                                                                                  |
| Authorization                            | header   | string  | The authorization header for the request.                                                                                                                 |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                                                                    |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                                                       |
| version                                  | header   | string  | The version of the workstream to update. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned |

## 54.2 Response

On success a 204 NO CONTENT is returned with no body.

# 55. DeleteWorkstreamShortName

The DeleteWorkstreamShortName API hard deletes a short name from a workstream.

## 55.1 Request

```http request
DELETE /v1/tenants/{tenant_id}/workstreams/{workstream_id}/shortnames/{name} HTTP/1.1
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>
```

| Parameter                                | Location | Type    | Description                                                                                                                                               |
|------------------------------------------|----------|---------|-----------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the workstream.                                                                                                            |
| workstream_id                            | path     | string  | The ID of the workstream to delete the short name from.                                                                                                   |
| name                                     | path     | string  | The short name to delete from the workstream.                                                                                                             |
| Authorization                            | header   | string  | The authorization header for the request.                                                                                                                 |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                                                                    |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                                                       |
| version                                  | header   | string  | The version of the workstream to update. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned |

## 55.2 Response
On success a 204 NO CONTENT is returned with no body.

# 56. ListWorkstreamShortNames
The ListWorkstreamShortNames API lists short names.

## 56.1 Request

```http request
GET /v1/tenants/{tenant_id}/shortnames?maxResults={maxResults}&token={token}&includeDeleted={includeDeleted}&workstreaID={workstreamID} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                                                                     |
|------------------------------------------|----------|---------|-----------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant to list short names for.                                                                   |
| maxResults                               | query    | *int    | Optional. The maximum number of short names to return. Default is 10. Must be >=1 and <= 500.                   |
| token                                    | query    | *string | Optional. A token to retrieve the next page of results. If not provided, the first page of results is returned. |
| includeDeleted                           | query    | *bool   | Optional. Set to true to include deleted short names in the results.                                            |
| workstreamID                             | query    | *string | Optional. If set, only return short names for the given workstream ID.                                          |
| Authorization                            | header   | string  | The authorization header for the request.                                                                       |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                          |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                             |

## 56.2 Response
On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "ShortNames": [],
  "NextToken": "*string"
}
```

| Field      | Type                                              | Description                                                                                    |
|------------|---------------------------------------------------|------------------------------------------------------------------------------------------------|
| ShortNames | [][WorkstreamShortName](#563-WorkstreamShortName) | A list of short names.                                                                         |
| NextToken  | *string                                           | A token to retrieve the next page of results. If there are no more results, this will be null. |

## 56.3 WorkstreamShortName

```json
{
  "TenantID": "string",
  "Name": "string",
  "WorkstreamID": "string",
  "WorkstreamVersion": int,
}
```

| Field             | Type   | Description                                                 |
|-------------------|--------|-------------------------------------------------------------|
| TenantID          | string | The tenant that owns the short name.                        |
| Name              | string | The short name.                                             |
| WorkstreamID      | string | The ID of the workstream the short name is associated with. |
| WorkstreamVersion | int    | The version of the workstream.                              |
 
# 57. MoveTask

The MoveTask API moves a task from one workstream to another.

## 57.1 Request

```http request
POST /v1/tenants/{tenant_id}/workstreams/{source_workstream_id}/tasks/{task_id}/move HTTP/1.1
Content-Type: application/json; charset=utf-8
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <task version>

{
    "DestinationWorkstreamID": "string"
}
```

| Parameter                                | Location | Type    | Description                                                                                                                                                  |
|------------------------------------------|----------|---------|--------------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the task.                                                                                                                     |
| source_workstream_id                     | path     | string  | The ID of the source workstream that currently owns the task.                                                                                                |
| task_id                                  | path     | string  | The ID of the task to move.                                                                                                                                  |
| Authorization                            | header   | string  | The authorization header for the request.                                                                                                                    |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                                                                       |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                                                          |
| If-Match                                 | header   | string  | The version of the task to move. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned.           |
| DestinationWorkstreamID                  | body     | string  | The ID of the workstream to move the task to.                                                                                                                |

## 57.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "Task": {
        "TenantID": "string",
        "WorkstreamID": "*string",  
        "TaskID": "string", 
        "Title": "string",
        "EnvironmentID": "*string",
        "Prompt": "string",
        "Parallel": bool,
        "Model": "*ModelType",
        "AssignedToTenantID": "*string",
        "AssignedToAI" : bool,  
        "RepoInfo: {},
        "State": "TaskState",
        "CreatedAt": "string",
        "UpdatedAt": "string",
        "Deleted": bool,
        "Version": int,
        "TaskNumber": int
    },
    "SourceWorkstream": {
        "WorkstreamID": "string",
        "TenantID": "string",
        "Name": "string",
        "Description": "string",
        "CreatedAt": "string",
        "UpdatedAt": "string",
        "Version": int,
        "Paused": bool,
        "Deleted": bool,
        "DefaultShortName": "string",
        "TaskCounter": int,
    },
    "DestinationWorkstream": {
        "WorkstreamID": "string",
        "TenantID": "string",
        "Name": "string",
        "Description": "string",
        "CreatedAt": "string",
        "UpdatedAt": "string",
        "Version": int,
        "Paused": bool,
        "Deleted": bool,
        "DefaultShortName": "string",
        "TaskCounter": int
    }
}
```

| Field                 | Type                        | Description                                        |
|-----------------------|-----------------------------|----------------------------------------------------|
| Task                  | [Task](#183-response)       | The updated task after the move.                   |
| SourceWorkstream      | [Workstream](#352-response) | The updated source workstream after the move.      |
| DestinationWorkstream | [Workstream](#352-response) | The updated destination workstream after the move. |

# 58. MoveShortName
The MoveShortName API moves a short name from one workstream to another.

## 58.1 Request

```http request
POST /v1/tenants/{tenant_id}/shortnames/{name}/move HTTP/1.1
Content-Type: application/json; charset=utf-8
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>

{
    "SourceWorkstreamID": "string",
    "DestinationWorkstreamID": "string",
    "SourceWorkstreamVersion": int,
    "DestinationWorkstreamVersion": int,
    "ReplacementName": "*string"
    "SetDefaultOnDestination": "bool"
}
```

| Parameter                                | Location | Type    | Description                                                                                                                                                 |
|------------------------------------------|----------|---------|-------------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the short name.                                                                                                              |
| name                                     | path     | string  | The short name to move.                                                                                                                                     |
| Authorization                            | header   | string  | The authorization header for the request.                                                                                                                   |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                                                                      |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                                                         |
| SourceWorkstreamID                       | body     | string  | The ID of the workstream to move the short name from.                                                                                                       |
| DestinationWorkstreamID                  | body     | string  | The ID of the workstream to move the short name to.                                                                                                         |
| SourceWorkstreamVersion                  | body     | int     | The version of the source workstream. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned      |
| DestinationWorkstreamVersion             | body     | int     | The version of the destination workstream. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned |
| ReplacementName                          | body     | *string | Optional. A short name to add to the source workstream to replace the moved name. If not provided, no replacement is added.                                 |
| SetDefaultOnDestination                  | body     | bool    | If true, set the moved short name as the default short name on the destination workstream.                                                                  |

## 58.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "SourceWorkstream": {
        "WorkstreamID": "string",
        "TenantID": "string",
        "Name": "string",
        "Description": "string",
        "CreatedAt": "string",
        "UpdatedAt": "string",
        "Version": int,
        "Paused": bool,
        "Deleted": bool,
        "DefaultShortName": "string",
        "TaskCounter": int,
    },
    "DestinationWorkstream": {
        "WorkstreamID": "string",
        "TenantID": "string",
        "Name": "string",
        "Description": "string",
        "CreatedAt": "string",
        "UpdatedAt": "string",
        "Version": int,
        "Paused": bool,
        "Deleted": bool,
        "DefaultShortName": "string",
        "TaskCounter": int
    }
}
```

| Field                 | Type                        | Description                                        |
|-----------------------|-----------------------------|----------------------------------------------------|
| SourceWorkstream      | [Workstream](#352-response) | The updated source workstream after the move.      |
| DestinationWorkstream | [Workstream](#352-response) | The updated destination workstream after the move. |

# 59. ListTenants

ListTenants is an admin api that lists all tenants in the system.

## 59.1 Request

```http request
GET /v1/tenants?maxResults={maxResults}&token={token} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                      | Location | Type    | Description                                                                                                     |
|--------------------------------|----------|---------|-----------------------------------------------------------------------------------------------------------------|
| maxResults                     | query    | *int    | Optional. The maximum number of tenants to return. Default is 10. Must be >=1 and <= 500.                       |
| token                          | query    | *string | Optional. A token to retrieve the next page of results. If not provided, the first page of results is returned. |
| Authorization                  | header   | string  | The authorization header for the request.                                                                       |
| X-Event-Horizon-Signed-Headers | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                             |

## 59.2 Response
On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "NextToken": "*string",
    "Tenants": []
}
```

| Field     | Type                     | Description                                                                                    |
|-----------|--------------------------|------------------------------------------------------------------------------------------------|
| NextToken | *string                  | A token to retrieve the next page of results. If there are no more results, this will be null. |
| Tenants   | [][Tenant](#32-response) | A list of tenants.                                                                             |


# 60. CreateWorkstreamTask

The CreateWorkstreamTask API creates a new task in a workstream. Workstream tasks are different form "ordinary" tasks
in a few ways:

1. "Normal" tasks are always executed by AI, and execute immediately when created. They must be fully configured to 
   be created.
 
2. Workstream tasks can be assigned to either AI or a human, or may even be unassigned.

   We only execute tasks that are assigned to AI. Human / unassigned tasks are managed manually by users.

3. Workstream tasks are ordered, and run in sequence by default. 

   All tasks in a workstream are ordered. By default, they execute sequentially in the order specified. A sequential
   AI task will automatically run when all the tasks above it in the list have completed. 

4. Editing one workstream task could cause many tasks to start executing. 

   Tasks can also be marked as parallel. This allows a group of "parallel" AI tasks stacked on top of each other
   to start simultaneously once any sequential tasks above them have completed. 

   For example, there may be a "write API specs" task assigned to a human, followed by three "implement API X" tasks
   below it. Once the human marks their task as completed, the AI tasks that depend on it can start.

5. We assume workstream tasks are not fully configured when created.

   Workstreams are collections of related tasks that will be worked on over time via some combination of AI and human
   effort. They are kind of like "mini-sprints". Getting a workstream setup correctly is similar to sprint planning or
   backlog grooming.

   It takes a lot of edits to get a sprint ready to be executed. All the tasks have to be created, they have
   to be ordered properly to manage dependencies, assigned to the right people (or AI), details have to be filled in
   etc. We want an intuitive interface so we allow tasks to be created in an incomplete state, and then edited
   iteratively until they are ready.

   We try hard to make sure that users can't make edits that have massively unintended consequences (like spinning up
   a massive number of AI tasks that aren't actually fully defined yet).

   When created, Workstreams start in a paused state, and task don't start executing until the workstream is "unpaused".

   This gives the humans planning a sprint the chance to get everything in the right state, review, then click "go"
   before AI agents start executing tasks.

6. Workstream tasks are never executed immediately upon creation.

   If a newly created workstream task would be executable, we pause the workstream automatically to prevent them from
   executing.

   Anything that looks like a mini replanning session will automatically pause the workstream until a human
   clicks the "unpause" button. This includes changes like:

   * Adding a new AI assigned task the workstream.
   * Re-ordering tasks in the sprint.
   * Assigning a task to AI that was previously assigned to a human or unassigned.
   * Re-assigning (or un-assigning) a task that was previously assigned to AI.
   * Undeleting a task that was previously deleted.
   * Manual edits to AI tasks.

   As a result, we allow UpdateWorkstreamTask to edit a bunch of fields that UpdateTask is not allowed to. We also
   make many parameters to CreateWorkstreamTask optional that are required in CreateTask.

7. Workstream tasks have "task-numbers" assigned to them. 

   This allows us to use work stream short names + the task number to refer to a task (like "API-1234"). 


## 60.1 Request

```http request
PUT /v1/tenants/{tenant_id}/workstreams/{workstream_id}/tasks/{task_id} HTTP/1.1
Content-Type: application/json; charset=utf-8
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>

{
    "Title": "string",
    "EnvironmentID": "*string",
    "Prompt": "*string",
    "Parallel": "bool",
    "Model": "*ModelType",
    "AssignedToTenantID": "*string",
    "AssignedToAI": "bool",
    "RepoInfo": {},
    "State": "*TaskState",
    "FileIDs": []
}
```

| Parameter                                | Location | Type                         | Description                                                                                             |
|------------------------------------------|----------|------------------------------|---------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string                       | The ID of the tenant that owns the workstream.                                                          |
| workstream_id                            | path     | string                       | The ID of the workstream to create the task in.                                                         |
| task_id                                  | path     | string                       | The ID of the new task to create.                                                                       |
| Authorization                            | header   | string                       | The authorization header for the request.                                                               |
| X-Event-Horizon-Delegating-Authorization | header   | *string                      | The authorization header for the delegating principal.                                                  |
| X-Event-Horizon-Signed-Headers           | header   | *string                      | The signed headers for the request, when authenticating with Sigv4.                                     |
| Title                                    | body     | string                       | The title of the task.                                                                                  |
| EnvironmentID                            | body     | *string                      | Optional. The ID of the environment the task should run in. If not provided, the workstream is not set. |
| Prompt                                   | body     | *string                      | Optional. The prompt to use for the task.                                                               |
| Parallel                                 | body     | bool                         | Optional. If set to true, the task is marked as parallel.                                               |
| Model                                    | body     | *[ModelType](#182-modeltype) | Optional. The model to use for the task.                                                                |
| AssignedToTenantID                       | body     | *string                      | Optional. The ID of the tenant the task is assigned to.                                                 |
| AssignedToAI                             | body     | bool                         | Whether the task is assigned to AI.                                                                     |
| RepoInfo                                 | body     | [RepoInfo](#185-repoinfo)    | Optional. Information about the repository associated with the task.                                    |
| State                                    | body     | [TaskState](#186-taskstate)  | Optional. The state of the task. If not specified, will default to "Pending".                           |
| FileIDs                                  | body     | []string                     | Optional. A list of file IDs to attach to the task.                                                     | 

## 60.2 Validation / Semantics

1. Model should only be set if AssignedToAI is true.
2. RepoInfo should only be set if AssignedToAI is true.
3. AssignedToAI can only be true if AssignedToTenantID is null.
4. AssignedToTenantID can only be not null if AssignedToAI is false
5. Currently, AssignedToTenantID must either be null or equal to the workstream's tenant.
   
   Eventually we will add support for organizations. When we do, for workstreams in an organization AssignedToTenantID
   can be assigned to any tenant that is a member of the organization. Currently, however, we only support single user
   tenants. Which means the states we allow are "unassigned", "assigned to the workstream's tenant", or "assigned to AI".
   We will need to change this later, but for now we should validate that AssignedToTenantID is either null or equal to
   the workstream's tenant.

6. If a new tasks is assigned to AI, the workstream will be paused automatically.
7. The new task needs a task number assigned to it.

   This should be done by incrementing the workstream's task counter and using the new value for the tasks number. 
   This also means creating a new workstream tasks always requires updating the workstream row. That implies that
   concurrent calls to CreateTask or UpdateTask can generate 409 CONFLICT errors. Clients that edit workstreams will
   need to add retry logic to handle this.

8. The new task will need `rank_generation` and `rank` fields assigned

   Those fields are used to order tasks in a workstream. We use a schema similar to Jira's LexoRank. See docs/RANKING.md
   for details on how ranking works. We always insert new workstream tasks at the bottom of the list, so the new task's
   rank should be computed accordingly.

   When implementing this in the API, you should add `RankGenration` and `Rank` as fields in the `daoTask` struct.
   These fields are not exposed via the API, but they need to be stored in the database. Make sure to configure the fields
   to be excluded from JSON serialization. 
   
   When generating new rank generations, make sure to update the min_gen and max_gen fields in the Workstream table.
   Add MinGeneration and MaxGeneration fields to the daoWorkstream struct. Make sure to configure them to be excluded from
   JSON serialization.

   Tasks can be re-ordered after creation by calling the UpdateWorkstreamTask API and settings either BeforeTaskID or
   AfterTaskID.

## 60.3 Response

On success a 201 CREATED is returned with the following JSON body:

```http request
HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8

{
    "TenantID": "string",
    "WorkstreamID": "*string",  
    "TaskID": "string", 
    "Title": "string",
    "EnvironmentID": "*string",
    "Prompt": "string",
    "Parallel": bool,
    "Model": "*ModelType",
    "AssignedToTenantID": "*string",
    "AssignedToAI" : bool,  
    "RepoInfo: {},
    "State": "TaskState",
    "CreatedAt": "string",
    "UpdatedAt": "string",
    "Deleted": bool,
    "Version": int,
    "TaskNumber": int,
    "FileIDs": []
}
```

# 61. ListWorkstreamTasks

The ListWorkstreamTasks API lists tasks in a workstream. Results are always ordered by increasing rank. To change
the order of tasks use the UpdateWorkstreamTask API and set one of the BeforeTaskID or AfterTaskID fields. 

## 61.1 Request

```http request
GET /v1/tenants/{tenant_id}/workstreams/{workstream_id}/tasks?maxResults={maxResults}&token={token}&afterTaskId={afterTaskId}&beforeTaskId={beforeTaskId}&includeDeleted={includeDeleted}&taskNumber={taskNumber} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                                                                                                                             |
|------------------------------------------|----------|---------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the workstream.                                                                                                                          |
| workstream_id                            | path     | string  | The ID of the workstream to list tasks for.                                                                                                                             |
| afterTaskId                              | query    | *string | Optional. When set, returns tasks ordered after the provided task id.                                                                                                   |
| beforeTaskId                             | query    | *string | Optional. When set, returns tasks ordered before the provided task id, in reverse rank order. Cannot be combined with afterTaskId.                                       |
| maxResults                               | query    | *int    | Optional. The maximum number of tasks to return. Default is 10. Must be >=1 and <= 500.                                                                                 |
| token                                    | query    | *string | Optional. A token to retrieve the next page of results. If not provided, the first page of results is returned.                                                         |
| includeDeleted                           | query    | *bool   | Optional. Set to true to include deleted tasks in the results.                                                                                                          |
| taskNumber                               | query    | *int    | Optional. When set, returns the task with the given task number. If no such task exists, an empty list is returned. If this parameter is set, afterTaskId is ignored. |
| Authorization                            | header   | string  | The authorization header for the request.                                                                                                                               |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                                                                                  |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                                                                     |

## 61.2 Response
On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "NextToken": "*string",
    "Tasks": []
}
```

| Field     | Type                    | Description                                                                                    |
|-----------|-------------------------|------------------------------------------------------------------------------------------------|
| NextToken | *string                 | A token to retrieve the next page of results. If there are no more results, this will be null. |
| Tasks     | [][Task](#183-response) | A list of tasks in the workstream.                                                             |

# 62. UpdateWorkstreamTask

The UpdateWorkstreamTask API updates a task in a workstream.

## 62.1 Request

```http request
PATCH /v1/tenants/{tenant_id}/workstreams/{workstream_id}/tasks/{task_id} HTTP/1.1
Content-Type: application/json; charset=utf-8
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>

{
    "Title": "*string",
    "EnvironmentID": "*string",
    "Prompt": "*string",
    "Parallel": "*bool",
    "Model": "*ModelType",
    "AssignedToTenantID": "*string",
    "AssignedToAI": "*bool",
    "RepoInfo": {},
    "State": "*TaskState",
    "BeforeTaskID": "*string",
    "AfterTaskID": "*string",
    "Deleted": "*bool",
    "NewFileIDs": [],
    "RemoveFileIDs": []
}
```

| Parameter                                | Location | Type                         | Description                                                                                                                                                                                                                                                                    |
|------------------------------------------|----------|------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string                       | The ID of the tenant that owns the workstream.                                                                                                                                                                                                                                 |
| workstream_id                            | path     | string                       | The ID of the workstream that owns the task.                                                                                                                                                                                                                                   |
| task_id                                  | path     | string                       | The ID of the task to update.                                                                                                                                                                                                                                                  |
| Authorization                            | header   | string                       | The authorization header for the request.                                                                                                                                                                                                                                      |
| X-Event-Horizon-Delegating-Authorization | header   | *string                      | The authorization header for the delegating principal.                                                                                                                                                                                                                         |
| X-Event-Horizon-Signed-Headers           | header   | *string                      | The signed headers for the request, when authenticating with Sigv4.                                                                                                                                                                                                            |
| Version                                  | header   | string                       | The expected version of the task. Used for optimistic concurrency control.                                                                                                                                                                                                     |
| Title                                    | body     | *string                      | Optional. When set, updates the task title.                                                                                                                                                                                                                                    |
| EnvironmentID                            | body     | **string                     | Optional. When set, updates the task's environment.                                                                                                                                                                                                                            |
| Prompt                                   | body     | *string                      | Optional. When                                                                                                                                                                                                                                                                 |
| Parallel                                 | body     | *bool                        | Optional. If set, updates whether the task is marked as parallel.                                                                                                                                                                                                              |
| Model                                    | body     | *[ModelType](#182-modeltype) | Optional. The new AI model of the task.                                                                                                                                                                                                                                        |
| AssignedToTenantID                       | body     | *string                      | Optional. When set, updates the tenant the task is assigned to.                                                                                                                                                                                                                |
| AssignedToAI                             | body     | *bool                        | Optional. When set, updates whether the task is assigned to AI.                                                                                                                                                                                                                |
| RepoInfo                                 | body     | *[RepoInfo](#185-repoinfo)   | Optional. When set, updates the repository information of the task.                                                                                                                                                                                                            |
| State                                    | body     | *[TaskState](#186-taskstate) | Optional. When set, updates the state of the task.                                                                                                                                                                                                                             |
| BeforeTaskID                             | body     | *string                      | Optional. If set, moves the task to be ordered before the given task. Cannot be combined with AfterTaskID.                                                                                                                                                                     |
| AfterTaskID                              | body     | *string                      | Optional. If set, moves the task to be ordered after the given task. Cannot be combined with BeforeTaskID.                                                                                                                                                                     |
| Deleted                                  | body     | *bool                        | Optional. If false, undeletes the task. To delete a task, call the [DeleteWorkstreamTask](#66-deleteworkstreamtask) api.                                                                                                                                                       |
| NewFileIDs                               | body     | []string                     | Optional. When set, update the tasks's NewFileIDs list. Entries added to this list are also added to FileIDs. Removing an entry from this list signals to the next iteration of the agent that the file has already been uploaded to the model and does not need to be resent. |
| RemoveFileIDs                            | body     | []string                     | Optional. A list of file IDs to remove from the task. Will fail if an entry is present in FileIDs, but not present in NewFileIDs.                                                                                                                                              |

## 62.2 Response
On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "TenantID": "string",
    "WorkstreamID": "*string",  
    "TaskID": "string", 
    "Title": "string",
    "EnvironmentID": "*string",
    "Prompt": "string",
    "Parallel": bool,
    "Model": "*ModelType",
    "AssignedToTenantID": "*string",
    "AssignedToAI" : bool,  
    "RepoInfo: {},
    "State": "TaskState",
    "CreatedAt": "string",
    "UpdatedAt": "string",
    "Deleted": bool,
    "Version": int,
    "TaskNumber": int
}
```

See [Task](#183-response) for field descriptions.

# 63. DeleteWorkstreamTask

The DeleteWorkstreamTask API deletes a task in a workstream. Deleted tasks can be undeleted by calling the UpdateWorkstreamTask API
and setting the Deleted field to false.

## 63.1 Request

```http request
DELETE /v1/tenants/{tenant_id}/workstreams/{workstream_id}/tasks/{task_id} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>
```

| Parameter                                | Location | Type    | Description                                                                |
|------------------------------------------|----------|---------|----------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the workstream.                             |
| workstream_id                            | path     | string  | The ID of the workstream that owns the task.                               |
| task_id                                  | path     | string  | The ID of the task to delete.                                              |
| Authorization                            | header   | string  | The authorization header for the request.                                  |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                     |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.        |
| Version                                  | header   | string  | The expected version of the task. Used for optimistic concurrency control. |

## 63.2 Response
On success a 204 NO CONTENT is returned with no body.

# 64. GetWorkstreamTask

The GetWorkstreamTask API retrieves a task in a workstream.

## 64.1 Request

```http request
GET /v1/tenants/{tenant_id}/workstreams/{workstream_id}/tasks/{task_id} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                                |
|------------------------------------------|----------|---------|----------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the workstream.                             |
| workstream_id                            | path     | string  | The ID of the workstream that owns the task.                               |
| task_id                                  | path     | string  | The ID of the task to retrieve.                                            |
| Authorization                            | header   | string  | The authorization header for the request.                                  |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                     |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.        |

## 64.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "TenantID": "string",
    "WorkstreamID": "*string",  
    "TaskID": "string", 
    "Title": "string",
    "EnvironmentID": "*string",
    "Prompt": "string",
    "Parallel": bool,
    "Model": "*ModelType",
    "AssignedToTenantID": "*string",
    "AssignedToAI" : bool,
    "RepoInfo: {},
    "State": "TaskState",
    "CreatedAt": "string",
    "UpdatedAt": "string",
    "Deleted": bool,
    "Version": int,
    "TaskNumber": int,
    "FileIDs" : [],
}

```

See [Task](#183-response) for field descriptions.

# 65. CreateRunner

The CreateRunner API creates a new "runner" associated with a tenant.

## 68.1 Request

```http request
PUT /v1/tenants/{tenant_id}/runners/{runner_id} HTTP/1.1
Content-Type: application/json; charset=utf-8
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>

{
    "Name": "string",
    "Description": "*string",
    "IsCloud" : bool,
    "RunsTasks" : bool,
    "ProxiesGithub":bool    
}
```

| Parameter                                | Location | Type    | Description                                                         |
|------------------------------------------|----------|---------|---------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the runner.                          |
| runner_id                                | path     | string  | The ID of the new runner to create.                                 |
| Authorization                            | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.              |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4. |
| Name                                     | body     | string  | The name of the runner.                                             |
| Description                              | body     | *string | Optional. The description of the runner.                            |
| IsCloud                                  | body     | bool    | Whether the runner is a cloud runner.                               |
| RunsTasks                                | body     | bool    | Whether the runner is used to execute tasks.                        |
| ProxiesGithub                            | body     | bool    | Whether the runner proxies access to github.                        |

## 65.2 Response

On success a 201 CREATED is returned with the following JSON body:

```http request
HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8

{
    "TenantID": "string",
    "RunnerID": "string",    
    "Name": "string",
    "Description": "string",
    "IsCloud": bool,
    "RunsTasks": bool,
    "ProxiesGithub": bool,
    "CreatedAt": "string",
    "UpdatedAt": "string",
    "Deleted": bool,
    "Version": int
}
```

| Field         | Type   | Description                                                         |
|---------------|--------|---------------------------------------------------------------------|
| TenantID      | string | The ID of the tenant that owns the runner.                          |
| RunnerID      | string | The ID of the runner.                                               |
| Name          | string | The name of the runner.                                             |
| Description   | string | The description of the runner.                                      |
| IsCloud       | bool   | Whether the runner is a cloud runner.                               |
| RunsTasks     | bool   | Whether the runner is used to execute tasks.                        |
| ProxiesGithub | bool   | Whether the runner proxies access to github.                        |
| CreatedAt     | string | The timestamp when the runner was created.                          |
| UpdatedAt     | string | The timestamp when the runner was last updated.                     |
| Deleted       | bool   | Whether the runner has been deleted.                                |
| Version       | int    | The version of the runner. Used for optimistic concurrency control. |

# 66. ListRunners

The ListRunners API lists runners associated with a tenant.

## 66.1 Request

```http request
GET /v1/tenants/{tenant_id}/runners?maxResults={maxResults}&token={token}&includeDeleted={includeDeleted}&runsTasks={runsTasks}&proxiesGithub={proxiesGithub} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                                                                     |
|------------------------------------------|----------|---------|-----------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the runners.                                                                     |
| maxResults                               | query    | *int    | Optional. The maximum number of runners to return. Default is 10. Must be >=1 and <= 500.                       |
| token                                    | query    | *string | Optional. A token to retrieve the next page of results. If not provided, the first page of results is returned. |
| includeDeleted                           | query    | *bool   | Optional. Set to true to include deleted runners in the results.                                                |
| runsTasks                                | query    | *bool   | Optional. When set, filters runners by whether they execute tasks.                                              |
| proxiesGithub                            | query    | *bool   | Optional. When set, filters runners by whether they proxy GitHub access.                                        |
| Authorization                            | header   | string  | The authorization header for the request.                                                                       |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                          |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                             |

## 66.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "NextToken": "*string",
    "Items": []
}
```

| Field     | Type                      | Description                                                                                    |
|-----------|---------------------------|------------------------------------------------------------------------------------------------|
| NextToken | *string                   | A token to retrieve the next page of results. If there are no more results, this will be null. |
| Items     | [][Runner](#652-response) | A list of runners associated with the tenant.                                                  |

# 67. GetRunner

The GetRunner API retrieves a runner associated with a tenant.

## 67.1 Request

```http request
GET /v1/tenants/{tenant_id}/runners/{runner_id}?includeDeleted={includeDeleted} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                         |
|------------------------------------------|----------|---------|---------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the runner.                          |
| runner_id                                | path     | string  | The ID of the runner to retrieve.                                   |
| includeDeleted                           | query    | *bool   | Optional. Set to true to return a deleted runner.                   |
| Authorization                            | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.              |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4. |

## 67.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "TenantID": "string",
    "RunnerID": "string",    
    "Name": "string",
    "Description": "string",
    "IsCloud": bool,
    "RunsTasks": bool,
    "ProxiesGithub": bool,
    "CreatedAt": "string",
    "UpdatedAt": "string",
    "Deleted": bool,
    "Version": int
}
```

See [here](662-response) for more details.

# 68. UpdateRunner

The UpdateRunner API updates a runner associated with a tenant.

## 68.1 Request

```http request
PATCH /v1/tenants/{tenant_id}/runners/{runner_id} HTTP/1.1
Content-Type: application/json; charset=utf-8
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>

{
    "Name": "*string",
    "Description": "*string",
    "IsCloud" : "*bool",
    "RunsTasks" : "*bool",
    "ProxiesGithub":*bool,
    "Deleted": "*bool"    
}
```

| Parameter                                | Location | Type    | Description                                                                  |
|------------------------------------------|----------|---------|------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the runner.                                   |
| runner_id                                | path     | string  | The ID of the runner to update.                                              |
| Authorization                            | header   | string  | The authorization header for the request.                                    |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                       |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.          |
| Version                                  | header   | string  | The expected version of the runner. Used for optimistic concurrency control. |
| Name                                     | body     | *string | Optional. When set, updates the name of the runner.                          |
| Description                              | body     | *string | Optional. When set, updates the description of the runner.                   |
| IsCloud                                  | body     | *bool   | Optional. When set, updates whether the runner is a cloud runner.            |
| RunsTasks                                | body     | *bool   | Optional. When set, updates whether the runner is used to execute tasks.     |
| ProxiesGithub                            | body     | *bool   | Optional. When set, updates whether the runner proxies access to github.     |
| Deleted                                  | body     | *bool   | Optional. Set to false to undelete a runner.                                 |

## 68.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "TenantID": "string",
    "RunnerID": "string",    
    "Name": "string",
    "Description": "string",
    "IsCloud": bool,
    "RunsTasks": bool,
    "ProxiesGithub": bool,
    "CreatedAt": "string",
    "UpdatedAt": "string",
    "Deleted": bool,
    "Version": int
}
```

See [here](#652-response) for more details.

# 69. DeleteRunner

The DeleteRunner API soft deletes a runner associated with a tenant. 

## 69.1 Request

```http request
DELETE /v1/tenants/{tenant_id}/runners/{runner_id} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>
```

| Parameter                                | Location | Type    | Description                                                                  |
|------------------------------------------|----------|---------|------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the runner.                                   |
| runner_id                                | path     | string  | The ID of the runner to delete.                                              |
| Authorization                            | header   | string  | The authorization header for the request.                                    |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                       |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.          |
| Version                                  | header   | string  | The expected version of the runner. Used for optimistic concurrency control. |

## 69.2 Response
On success a 204 NO CONTENT is returned with no body.

# 70. CreateGithubConnection

The CreateGithubConnection API creates a new GitHub connection for a tenant.

## 70.1 Request

```http request
PUT /v1/tenants/{tenant_id}/github-connections/{connection_id} HTTP/1.1
Content-Type: application/json; charset=utf-8
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>

{
    "Private": bool,
    "RunnerID" : "*string",
    "GithubUserLogin" : "*string",
    "GithubUserID" : *int,
    "Name": *string
}
```

| Parameter                                | Location | Type    | Description                                                                                                                                                                     |
|------------------------------------------|----------|---------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the GitHub connection.                                                                                                                           |
| connection_id                            | path     | string  | The ID of the new GitHub connection to create.                                                                                                                                  |
| Authorization                            | header   | string  | The authorization header for the request.                                                                                                                                       |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                                                                                          |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                                                                             |
| Private                                  | body     | bool    | Whether the connection is private.                                                                                                                                              |
| RunnerID                                 | body     | *string | Optional. The ID of the runner associated with the connection. Required when Private is true.                                                                                   |
| GithubUserLogin                          | body     | *string | The GitHub user login associated with the connection. Only valid when Private is false. For private github connection, all user information is configured on the remote runner. |
| GithubUserID                             | body     | *int    | The GitHub user ID associated with the connection. Only valid when Private is false. For private github connection, all user information is configured on the remote runner.    |
| Name                                     | body     | *string | The name of the github connection. Required when `Private` is true. Invalid if `Private` is false.                                                                              |

## 70.2 Response

On success a 201 CREATED is returned with the following JSON body:

```http request
HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8

{
    "TenantID": "string",
    "ConnectionID": "string",
    "Private": bool,
    "RunnerID" : "*string",
    "GithubUserLogin" : "*string",
    "GithubUserID" : *int,
    "OAuthToken": "*string",
    "RefreshToken": "*string",
    "TokenExpiry": "*string",
    "State" : "*string",
    "StateExpiry" : "*string",
    "CreatedAt": "string",
    "UpdatedAt": "string",
    "Name" : "*string",
    "Version": int
}
```

| Field           | Type    | Description                                                             |
|-----------------|---------|-------------------------------------------------------------------------|
| TenantID        | string  | The ID of the tenant that owns the GitHub connection.                   |
| ConnectionID    | string  | The ID of the GitHub connection.                                        |
| Private         | bool    | Whether the connection is private.                                      |
| RunnerID        | *string | The ID of the runner associated with the connection.                    |
| GithubUserLogin | *string | The GitHub user login associated with the connection.                   |
| GithubUserID    | *int    | The GitHub user ID associated with the connection.                      |
| OAuthToken      | *string | The OAuth token for the connection.                                     |
| RefreshToken    | *string | The refresh token for the connection.                                   |
| TokenExpiry     | *string | The expiry time of the OAuth token.                                     |
| State           | *string | The state parameter for OAuth flows.                                    |
| StateExpiry     | *string | The expiry time of the state parameter.                                 |
| CreatedAt       | string  | The timestamp when the connection was created.                          |
| UpdatedAt       | string  | The timestamp when the connection was last updated.                     |
| Version         | int     | The version of the connection. Used for optimistic concurrency control. |
| Name            | *string | The name of the GitHub connection.                                      |

# 71. ListGithubConnections

The ListGithubConnections API lists GitHub connections associated with a tenant.

## 71.1 Request

```http request
GET /v1/tenants/{tenant_id}/github-connections?maxResults={maxResults}&token={token}&private={private} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                                                                     |
|------------------------------------------|----------|---------|-----------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the GitHub connections.                                                          |
| maxResults                               | query    | *int    | Optional. The maximum number of connections to return. Default is 10. Must be >=1 and <= 500.                   |
| token                                    | query    | *string | Optional. A token to retrieve the next page of results. If not provided, the first page of results is returned. |
| private                                  | query    | *bool   | Optional. When set, filters connections by whether they are private.                                            |
| Authorization                            | header   | string  | The authorization header for the request.                                                                       |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                          |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                             |

## 71.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "NextToken": "*string",
    "Items": []
}
```

| Field     | Type                                | Description                                                                                    |
|-----------|-------------------------------------|------------------------------------------------------------------------------------------------|
| NextToken | *string                             | A token to retrieve the next page of results. If there are no more results, this will be null. |
| Items     | [][GithubConnection](#702-response) | A list of GitHub connections associated with the tenant.                                       |

# 72. GetGithubConnection

The GetGithubConnection API retrieves a GitHub connection associated with a tenant.

## 72.1 Request

```http request
GET /v1/tenants/{tenant_id}/github-connections/{connection_id} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                         |
|------------------------------------------|----------|---------|---------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the GitHub connection.               |
| connection_id                            | path     | string  | The ID of the GitHub connection to retrieve.                        |
| Authorization                            | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.              |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4. |

## 72.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "TenantID": "string",
    "ConnectionID": "string",
    "Private": bool,
    "RunnerID" : "*string",
    "GithubUserLogin" : "*string",
    "GithubUserID" : *int,
    "OAuthToken": "*string",
    "RefreshToken": "*string",
    "TokenExpiry": "*string",
    "State" : "*string",
    "StateExpiry" : "*string",
    "CreatedAt": "string",
    "UpdatedAt": "string",
    "Version": int
}
```

See [here](#702-response) for more details.

# 73. UpdateGithubConnection

The UpdateGithubConnection API updates a GitHub connection associated with a tenant.

## 73.1 Request

```http request
PATCH /v1/tenants/{tenant_id}/github-connections/{connection_id} HTTP/1.1
Content-Type: application/json; charset=utf-8
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>

{
    "Private": *bool,
    "RunnerID" : "*string",
    "GithubUserLogin" : "*string",
    "GithubUserID" : *int,
    "OAuthToken": "*string",
    "RefreshToken": "*string",
    "TokenExpiry": "*string",
    "State"  : "*string",
    "StateExpiry" : "*string",
    "Name" : "*string",        
}
```

| Parameter                                | Location | Type    | Description                                                                                                                     |
|------------------------------------------|----------|---------|---------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the GitHub connection.                                                                           |
| connection_id                            | path     | string  | The ID of the GitHub connection to update.                                                                                      |
| Authorization                            | header   | string  | The authorization header for the request.                                                                                       |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                                          |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                             |
| Version                                  | header   | string  | The expected version of the connection. Used for optimistic concurrency control.                                                |
| Private                                  | body     | *bool   | Optional. When set, updates whether the connection is private.                                                                  |
| RunnerID                                 | body     | *string | Optional. When set, updates the ID of the runner associated with the connection.                                                |
| GithubUserLogin                          | body     | *string | Optional. When set, updates the GitHub user login associated with the connection.                                               |
| GithubUserID                             | body     | *int    | Optional. When set, updates the GitHub user ID associated with the connection.                                                  |
| OAuthToken                               | body     | *string | Optional. When set, updates the OAuth token for the connection.                                                                 |
| RefreshToken                             | body     | *string | Optional. When set, updates the refresh token for the connection.                                                               |
| TokenExpiry                              | body     | *string | Optional. When set, updates the expiry time of the OAuth token returned by GitHub. Set to "" to clear the value.               |
| State                                    | body     | *string | Optional. When set, updates the state parameter for OAuth flows. Set to "" to clear the value.                                  |
| StateExpiry                              | body     | *string | Optional. When set, updates the expiry time of the state parameter. Set to "" to clear the value.                               |
| Name                                     | body     | *string | Optional. When set, updates the name of the github connection. Set to "" to clear the value. Not valid when `Private` is false. |

## 73.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "TenantID": "string",
    "ConnectionID": "string",
    "Private": bool,
    "RunnerID" : "*string",
    "GithubUserLogin" : "*string",
    "GithubUserID" : *int,
    "OAuthToken": "*string",
    "RefreshToken": "*string",
    "TokenExpiry": "*string",
    "State" : "*string",
    "StateExpiry" : "*string",
    "CreatedAt": "string",
    "UpdatedAt": "string",
    "Name" : "*string",
    "Version": int
}
```

See [here](#702-response) for more details.

# 74. DeleteGithubConnection

The DeleteGithubConnection API hard deletes a GitHub connection associated with a tenant.

## 74.1 Request

```http request
DELETE /v1/tenants/{tenant_id}/github-connections/{connection_id} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>
```

| Parameter                                | Location | Type    | Description                                                                      |
|------------------------------------------|----------|---------|----------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the GitHub connection.                            |
| connection_id                            | path     | string  | The ID of the GitHub connection to delete.                                       |
| Authorization                            | header   | string  | The authorization header for the request.                                        |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                           |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.              |
| Version                                  | header   | string  | The expected version of the connection. Used for optimistic concurrency control. |

## 74.2 Response

On success a 204 NO CONTENT is returned with no body.

# 75. GenerateRunnerToken

The GenerateRunnerToken API generates a new token for a runner.

## 75.1 Request

```http request
PUT /v1/tenants/{tenant_id}/runners/{runner_id}/tokens/{tokenID} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
Content-Type: application/json

{
    "TTLDays": int
}
```

| Parameter                                | Location | Type    | Description                                                                                       |
|------------------------------------------|----------|---------|---------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the runner.                                                        |
| runner_id                                | path     | string  | The ID of the runner to generate a token for.                                                     |
| tokenID                                  | path     | string  | The ID of the new token to generate. Must be a V4 UUID.                                           |
| Authorization                            | header   | string  | The authorization header for the request.                                                         |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                            |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                               |
| TTLDays                                  | body     | *int    | Optional. Token lifetime in days. Defaults to 90 when omitted. Must be between 1 and 365, inclusive. |
## 75.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "TenantID": "string",
    "RunnerID": "string",
    "TokenID": "string",
    "Token": "string",
    "CreatedAt": "string",
    "ExpiresAt": "string",
    "Revoked": bool,
    "RevokedAt": "*string",
    "Version": int,
    "SignatureHash": "string"
}
```

| Field         | Type    | Description                                                        |
|---------------|---------|--------------------------------------------------------------------|
| TokenID       | string  | The ID of the generated token.                                     |
| Token         | string  | The generated token.                                               |
| CreatedAt     | string  | The timestamp when the token was created.                          |
| ExpiresAt     | string  | The timestamp when the token expires.                              |
| Revoked       | bool    | Whether the token has been revoked.                                |
| RevokedAt     | *string | The timestamp when the token was revoked. Null if not revoked.     |
| Version       | int     | The version of the token. Used for optimistic concurrency control. |
| SignatureHash | string  | The base64 encoding of the sha256 hash of the token signature.     |

# 76. ListRunnerTokens

The ListRunnerTokens API returns a list of metadata about tokens for a runner.

## 76.1 Request

```http request
GET /v1/tenants/{tenant_id}/runners/{runner_id}/tokens?maxResults={maxResults}&nextPageToken={nextPageToken}&includeRevoked={includeRevoked} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                                              |
|------------------------------------------|----------|---------|------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the runner.                                               |
| runner_id                                | path     | string  | The ID of the runner to list tokens for.                                                 |
| maxResults                               | query    | *int    | Optional. The maximum number of tokens to return. Default is 10. Must be >=1 and <= 500. |
| nextPageToken                            | query    | *string | Optional. A token to retrieve the next page of results.                                  |
| includeRevoked                           | query    | *bool   | Optional. Whether to include revoked tokens in the results. Default is false.            |
| Authorization                            | header   | string  | The authorization header for the request.                                                |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                   |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                      |

## 76.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "NextPageToken": "*string",
    "Items": [{
        "TenantID": "string",
        "RunnerID": "string",
        "TokenID": "string",
        "CreatedAt": "string",
        "ExpiresAt": "string",
        "Revoked": bool,
        "RevokedAt": "*string",
        "Version": int,
        "SignatureHash": "string"
    }]
}
```

| Field         | Type                                              | Description                                                                                    |
|---------------|---------------------------------------------------|------------------------------------------------------------------------------------------------|
| NextPageToken | *string                                           | A token to retrieve the next page of results. If there are no more results, this will be null. |
| Items         | [][RunnerTokenMetadata](#763-RunnerTokenMetadata) | A list of runner token metadata objects.                                                       |

## 76.3 RunnerTokenMetadata

The RunnerTokenMetadata object contains metadata about a runner token.

| Field         | Type    | Description                                                                 |
|---------------|---------|-----------------------------------------------------------------------------|
| TenantID      | string  | The ID of the tenant that owns the runner.                                  |
| RunnerID      | string  | The ID of the runner the token belongs to.                                  |
| TokenID       | string  | The ID of the token.                                                        |
| CreatedAt     | string  | The timestamp when the token was created.                                   |
| ExpiresAt     | string  | The timestamp when the token expires.                                       |
| Revoked       | bool    | Whether the token has been revoked.                                         |
| RevokedAt     | *string | The timestamp when the token was revoked. Null if the token is not revoked. |
| Version       | int     | The version of the token. Used for optimistic concurrency control.          |
| SignatureHash | string  | The sha256 hash of the token.                                               |

# 77. RevokeRunnerToken

The RevokeRunnerToken API revokes a runner token.

## 77.1 Request

```http request
POST /v1/tenants/{tenant_id}/runners/{runner_id}/tokens/{token_id}/revoke HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>
``` 

| Parameter                                | Location | Type    | Description                                                                  |
|------------------------------------------|----------|---------|------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the runner.                                   |
| runner_id                                | path     | string  | The ID of the runner the token belongs to.                                   |
| token_id                                 | path     | string  | The ID of the token to revoke.                                               | 
| Authorization                            | header   | string  | The authorization header for the request.                                    |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                       |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.          |
| Version                                  | header   | string  | The expected version of the token. Used for optimistic concurrency control.  |

## 77.2 Response

On success a 204 NO CONTENT is returned with no body.

# 78. RegisterRunnerQueue

The RegisterRunnerQueue API registers a new "queue" for a runner.

When customers deploy runners in their own environments, they can deploy multiple instances for high availability and
load balancing. Each runner instance will poll the api service for batches of messages to process, invoke them, and 
post the responses back to the api service. Instances dynamically adjust the number
of concurrent pollers they use based on load. Each runner instance will use at least one poller. We use a single
queue per poller. Thus a single logical runner can have multiple queues associated with it.

PLan 42 will load balance across a runner's queues using a round robin strategy. It applies health checks to
identify which queues are healthy, and only delivers messages to healthy queues. Health checks are run by sending
"ping" messages to each queue every 30 seconds, and waiting for a response with a 5 second timeout. After 2 consecutive
failed health checks, a queue is marked as unhealthy. After 10 consecutive failed health checks, a queue is automatically
deleted.

Each queue and each API service instance have an associated ECC P-256 key pair. API service instances generate their
key pair at startup. Runners generate a key pair each time they call RegisterRunnerQueue. ECIES is used to encrypt
messages sent to a queue, and to encrypt responses sent back to the API service. This uses ECDH and SHA256 to derive
the AES Key and IV used to encrypt the message payloads.

Plan 42 enforces that the public keys supplied to RegisterRunnerQueue are unique and have not been seen by the service
before.

## 78.1 Request

```http request
PUT /v1/tenants/{tenant_id}/runners/{runner_id}/queues/{queue_id} HTTP/1.1
Content-Type: application/json; charset=utf-8
Authorization: <authorization>

{
    "PublicKey": "string"
}
```

| Parameter     | Location | Type   | Description                                     |
|---------------|----------|--------|-------------------------------------------------|
| tenant_id     | path     | string | The ID of the tenant that owns the runner.      |
| runner_id     | path     | string | The ID of the runner to register the queue for. |
| queue_id      | path     | string | The ID of the queue to register.                |
| Authorization | header   | string | The authorization header for the request.       |
| PublicKey     | body     | string | The PEM encoded public key for the queue.       |

Note that this api does not support delegation.

## 78.2 Response

On success a 201 CREATED is returned with the following JSON body:

```http request
HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8

{
    "TenantID": "string",
    "RunnerID": "string",
    "QueueID": "string",
    "PublicKey": "string",
    "CreatedAt": "string",
    "Version": int,
    "IsHealthy": bool,
    "NConsecutiveFailedHealthChecks": int,
    "NConsecutiveSuccessfulHealthChecks": int,
    "LastHealthCheckAt": "string",
    "Draining": bool
}
```

| Field                              | Type   | Description                                                   |
|------------------------------------|--------|---------------------------------------------------------------|
| TenantID                           | string | The ID of the tenant that owns the runner.                    |
| RunnerID                           | string | The ID of the runner the queue is registered for.             |
| QueueID                            | string | The ID of the registered queue.                               |
| PublicKey                          | string | The PEM encoded public key of the queue.                      |
| CreatedAt                          | string | The timestamp when the queue was registered.                  |
| Version                            | int    | The current version of the queue record.                      |
| IsHealthy                          | bool   | Whether the queue is currently healthy.                       | 
| NConsecutiveFailedHealthChecks     | int    | The number of consecutive failed health checks.               |
| NConsecutiveSuccessfulHealthChecks | int    | The number of consecutive successful health checks.           |
| LastHealthCheckAt                  | string | The timestamp when the last health check was performed.       |
| Draining                           | bool   | Whether the queue is draining (no longer receiving messages). |

# 79. GetMessagesBatch

The GetMessagesBatch API retrieves a batch of messages for a runner queue

We use "at most once" semantics for messages. Messages are implemented asynchronously because the api service
(which sends messages to runners) doesn't have line of site the them. However, messages are used in syncronous request
response scenarios (for example, when calling a github api to search for repos, or to start a task job).

We don't implement "at least once" semantics because by the time we attempt to re-drive delivery, the original
caller would have already timed out and marked it's request as failed.

So, when messages are returned from this API, the are deleted from the queue before they are returned.

## 79.1 Request

```http request
GET /v1/tenants/{tenant_id}/runners/{runner_id}/queues/{queue_id}/messages&maxWaitSeconds={maxWaitSeconds} HTTP/1.1
Content-Type: application/json; charset=utf-8
Accept: application/json
Authorization: <authorization>
```

| Parameter      | Location | Type   | Description                                                                                                                        |
|----------------|----------|--------|------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id      | path     | string | The ID of the tenant that owns the runner.                                                                                         |
| runner_id      | path     | string | The ID of the runner to fetch messages for.                                                                                        |
| queue_id       | path     | string | The ID of the queue to fetch messages from.                                                                                        |
| maxWaitSeconds | query    | *int   | The maximum time in seconds to wait for messages to arrive if the queue is empty. Defaults to 0 (no wait). Maximum is 120 seconds. |
| Authorization  | header   | string | The authorization header for the request.                                                                                          |

Note that this api does not support delegation.

## 79.2 Response
On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "Messages": [{
        "TenantID": "string",
        "RunnerID": "string",
        "QueueID": "string",
        "MessageID": "string",
        "CallerID": "string",
        "CallerPublicKey": "string",
        "CreatedAt": "string",
        "Payload": {},
    }]
}
```

| Field         | Type                                  | Description                                                         |
|---------------|---------------------------------------|---------------------------------------------------------------------|
| Messages      | [][RunnerMessage](#793-RunnerMessage) | A list of messages for the queue. At most 10 messages are returned. |

## 79.3 RunnerMessage
The RunnerMessage object contains information about a message sent to a queue.

| Field           | Type                                | Description                                                 |
|-----------------|-------------------------------------|-------------------------------------------------------------|
| TenantID        | string                              | The ID of the tenant that owns the runner.                  |
| RunnerID        | string                              | The ID of the runner the queue is registered for.           |
| QueueID         | string                              | The ID of the queue the message was sent to.                |
| MessageID       | string                              | The ID of the message.                                      |
| CallerID        | string                              | The ID of the caller that sent the message.                 |
| CallerPublicKey | string                              | The PEM encoded public key of the caller that sent message. |
| CreatedAt       | string                              | The timestamp when the message was created.                 |
| Payload         | [WrappedSecret](#794-WrappedSecret) | The encrypted message.                                      |

## 79.4 WrappedSecret

WrappedSecret is a polymorphic type that represents an encrypted secret. Currently the only supported encryption
algorithm is `ECIES.Cofactor.VariableIV.X963.SHA256.AESGCM`, though we may add more in the future.

``json
{
    "EncryptionAlgorithm": "string",
    ...
}
``

| EncryptionAlgorithm                          | Type                                           | Description                                                                                                         |
|----------------------------------------------|------------------------------------------------|---------------------------------------------------------------------------------------------------------------------|
| ECIES.Cofactor.VariableIV.X963.SHA256.AESGCM | [ecies.WrappedSecret](#795-eciesWrappedSecret) | The secret is encrypted using ECIES with an ephemeral private key and the public key of the target queue or caller. |

## 79.5 ecies.WrappedSecret

```json

{
    "EncryptionAlgorithm": "ECIES.Cofactor.VariableIV.X963.SHA256.AESGCM",
    "EncryptedData": "string",
    "EphemeralPublicKey": "string",
}
```

| Field               | Type   | Description                                                                                     |
|---------------------|--------|-------------------------------------------------------------------------------------------------|
| EncryptionAlgorithm | string | The encryption algorithm used. Currently always `ECIES.Cofactor.VariableIV.X963.SHA256.AESGCM`. |
| EncryptedData       | string | The base64 encoded ciphertext and gcm tag of the secret.                                        |
| EphemeralPublicKey  | string | The PEM encoded ephemeral public key used to encrypt the secret.                                |

# 80. WriteResponse

The WriteResponse API is used by a runner to respond to a message it received via a call to GetMessagesBatch.

## 80.1 Request

```http request
PUT /v1/tenants/{tenant_id}/runners/{runner_id}/queues/{queue_id}/messages/{message_id}/response HTTP/1.1
Content-Type: application/json; charset=utf-8
Authorization: <authorization>

{
    "CallerID": "string",
    "Payload": {}
}
```

| Parameter     | Location | Type                                | Description                                          |
|---------------|----------|-------------------------------------|------------------------------------------------------|
| tenant_id     | path     | string                              | The ID of the tenant that owns the runner.           |
| runner_id     | path     | string                              | The ID of the runner writing the response.           |
| queue_id      | path     | string                              | The ID of the queue the response is associated with. |
| message_id    | path     | string                              | The ID of the message being responded to.            |
| Authorization | header   | string                              | The authorization header for the request.            |
| CallerID      | body     | string                              | The ID of the caller that sent the original message. |
| Payload       | body     | [WrappedSecret](#794-wrappedsecret) | The encrypted response.                              |

Note that this api does not support delegation.

## 80.2 Response
On success a 204 NO CONTENT is returned with no body.

# 81. SearchTasks

The SearchTasks API searches for tasks within a tenant. Tasks can be searched by GitHub pull request ID, task ID, or both values together.

## 81.1 Request

```http request
POST /v1/tasks/search?pullRequestId={pullRequestId}&taskId={taskId}&tenantID={tenantID} HTTP/1.1
Accept: application/json
Content-Type: application/json; charset=utf-8
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>

{}
```

| Parameter                                | Location | Type    | Description                                                                                            |
|------------------------------------------|----------|---------|--------------------------------------------------------------------------------------------------------|
| pullRequestId                            | query    | *int    | The GitHub pull request ID to search for. At least one of `pullRequestId` or `taskId` is required.     |
| taskId                                   | query    | *uuid   | The task ID to search for. At least one of `pullRequestId` or `taskId` is required.                    |
| tenantID                                 | query    | *string | The tenant ID to search within. If not provided, searches across all tenants the caller has access to. |
| Authorization                            | header   | string  | The authorization header for the request.                                                              |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                 |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                    |

The request body must be valid JSON. At present the body should be an empty object ( `{}` ). Future iterations of this API may define additional fields in the body.

## 81.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "Tasks": [],
  "NextToken": "*string"
}
```

| Field     | Type                    | Description                                                                                    |
|-----------|-------------------------|------------------------------------------------------------------------------------------------|
| Tasks     | [][Task](#183-response) | The set of tasks that match the provided search criteria.                                      |
| NextToken | *string                 | A token to retrieve the next page of results. If there are no more results, this will be null. |

Requests that do not match any tasks return `Tasks: []` with `NextToken` unset. If the supplied `pullRequestId` or `taskId` are invalid or the caller lacks access to the tenant, standard error responses are returned. When searching only by `taskId`, multiple entries for the same task may be returned if the task is associated with more than one pull request.

## 82.3 AuthZ Requirements

To call SearchTasks, the caller must have either the SearchTasks or SearchTenantTasks permission.
The SearchTasks permission allows searching for tasks across all tenants, While the SearchTenantTasks permission allows
searching for tasks within a specific tenant. The SearchTasks permission implies the SearchTenantTasks permission.

# 82. GetRunnerToken

The GetRunnerToken API retrieves metadata for a runner token by its ID.

## 82.1 Request

```http request
GET /v1/tenants/{tenant_id}/runners/{runner_id}/tokens/{token_id} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                         |
|------------------------------------------|----------|---------|---------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the runner.                          |
| runner_id                                | path     | string  | The ID of the runner the token belongs to.                          |
| token_id                                 | path     | string  | The ID of the token to retrieve.                                    |
| includeDeleted                           | query    | *bool   | Whether to include deleted tokens in the lookup.                    |
| Authorization                            | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.              |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4. |

## 82.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "TenantID": "string",
    "RunnerID": "string",
    "TokenID": "string",
    "CreatedAt": "string",
    "ExpiresAt": "string",
    "Revoked": bool,
    "RevokedAt": "*string",
    "Version": int,
    "SignatureHash": "string"
}
```

See [here](#763-RunnerTokenMetadata) for more details.

# 83. UpdateTenant

UpdateTenant API updates metadata for a tenant.

## 83.1 Request

```http request
PATCH /v1/tenants/{tenant_id} HTTP/1.1
Content-Type: application/json; charset=utf-8
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>

{
    "DefaultRunnerID" : "*string",
    "DefaultGithubConnectionID": "*string"
}
```

| Parameter                                | Location | Type    | Description                                                                                                                 |
|------------------------------------------|----------|---------|-----------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant to update.                                                                                             |
| Authorization                            | header   | string  | The authorization header for the request.                                                                                   |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                                      |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                         |
| Version                                  | header   | string  | The expected version of the tenant. Used for optimistic concurrency control.                                                |
| DefaultRunnerID                          | body     | *string | Optional. When set, updates the default runner ID for the tenant. Use "" to clear the default runner.                       |
| DefaultGithubConnectionID                | body     | *string | Optional. When set, updates the default GitHub connection ID for the tenant. Use "" to clear the default github connection. |

## 83.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "TenantID": "string",
  "Type": "TenantType",
  "Version": int,
  "Deleted": boolean,
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "FullName": "*string",
  "OrgName": "*string",
  "EnterpriseName": "*string",
  "Email": "*string",
  "FirstName": "*string",
  "LastName": "*string",
  "PictureURL": "*string",
  "DefaultRunnerID" : "*string",
  "DefaultGithubConnectionID": "*string",
  "LatestEncryptionKeyVersion": "*int"
}
```

See [here](#612-response) for more details.

# 89. ListRunnerQueues

The ListRunnerQueues API returns a list of runner queues, optionally filtered by tenant, runner, health status,
drained status, and queue ID range.

## 89.1 Request

```http request
GET /v1/runner-queues?tenantID={tenantID}&runnerID={runnerID}&includeHealthy={includeHealthy}&includeDrained={includeDrained}&maxResults={maxResults}&token={token}&minQueueID={minQueueID}&maxQueueID={maxQueueID} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                                                                                                                           |
|------------------------------------------|----------|---------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenantID                                 | query    | *string | Optional. The ID of the tenant to filter queues by. Must be combined with runnerID.                                                                                   |
| runnerID                                 | query    | *string | Optional. The ID of the runner to filter queues by. Must be combined with tenantID.                                                                                   |
| includeHealthy                           | query    | *bool   | Optional. Set to filter on healthy status. When not set, queues are not filtered on the IsHealthy field. Must be combined with tenant_id and runner_id.               |
| includeDrained                           | query    | *bool   | Optional. Set to filter on drained status. When not set, queues are not filtered on the Draining field. Must be combined with tenant_id and runner_id.                |
| maxResults                               | query    | *int    | Optional. The maximum number of queues to return. Default is 10. Must be >=1 and < 500.                                                                               |
| token                                    | query    | *string | Optional. A token to retrieve the next page of results.                                                                                                               |
| minQueueID                               | query    | *string | Optional. The includsive minimum queue ID to include in the results. Useful when partioning queues between health checker insances. Must be combined with maxQueueID. |
| maxQueueID                               | query    | *string | Optional. The exclusive maximum queue ID to include in the results. Useful when partioning queues between health checker instances. Must be combined with minQueueID. |
| Authorization                            | header   | string  | The authorization header for the request.                                                                                                                             |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                                                                                |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                                                                   |


## 89.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "NextToken": "*string",
    "Items": [{
        "TenantID": "string",
        "RunnerID": "string",
        "QueueID": "string",
        "PublicKey": "string",
        "CreatedAt": "string",
        "Version": int,
        "IsHealthy": bool,
        "NConsecutiveFailedHealthChecks": int,
        "NConsecutiveSuccessfulHealthChecks": int,
        "LastHealthCheckAt": "string",
        "Draining": bool
    }]
}
```

| Field     | Type                           | Description                                                                                    |
|-----------|--------------------------------|------------------------------------------------------------------------------------------------|
| NextToken | *string                        | A token to retrieve the next page of results. If there are no more results, this will be null. |
| Items     | [][RunnerQueue](#782-response) | A list of runner queue objects.                                                                |


## 89.3 AuthZ Requirements

We define 2 separate actions related to listing runner queues:

* ListRunnerQueues
* ListAllRunnerQueues

The ListAllRunnerQueues action is an admin permission required to list queues across multiple tenants / runners,
applies globally, and can only be granted to internal plan 42 service principals. The ListRunnerQueues is a tenant
scoped permission that allows listing queues within a specific tenant.

# 90. UpdateRunnerQueue

The UpdateRunnerQueue API updates metadata for a runner queue.

## 90.1 Request

```http request
PATCH /v1/tenants/{tenant_id}/runners/{runner_id}/queues/{queue_id} HTTP/1.1
Content-Type: application/json; charset=utf-8
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>

{
    "IsHealthy": *bool,
    "NConsecutiveFailedHealthChecks": *int,
    "NConsecutiveSuccessfulHealthChecks": *int,
    "LastHealthCheckAt": "string",
    "Draining": *bool
}
```

| Parameter                                | Location | Type    | Description                                                                                                                                                                                                  |
|------------------------------------------|----------|---------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the runner.                                                                                                                                                                   |
| runner_id                                | path     | string  | The ID of the runner the queue is registered for.                                                                                                                                                            |
| queue_id                                 | path     | string  | The ID of the queue to update.                                                                                                                                                                               |
| Authorization                            | header   | string  | The authorization header for the request.                                                                                                                                                                    |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                                                                                                                       |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                                                                                                          |
| Version                                  | header   | string  | The expected version of the queue. Used for optimistic concurrency control.                                                                                                                                  |
| IsHealthy                                | body     | *bool   | Optional. When set, updates the health status of the queue. This will be updated when health checks fail. It can also be expictly set to false by a runner when it is draining a queue prior to deleting it. |
| NConsecutiveFailedHealthChecks           | body     | *int    | Optional. When set, updates the number of consecutive failed health checks for the queue.                                                                                                                    |
| NConsecutiveSuccessfulHealthChecks       | body     | *int    | Optional. When set, updates the number of consecutive successful health checks for the queue.                                                                                                                |
| LastHealthCheckAt                        | body     | *string | Optional. When set, updates the timestamp of the last health check for the queue.                                                                                                                            |
| Draining                                 | body     | *bool   | Optional. Set to true to mark the queue as draining. Must be combined with `IsHealthy = false`. Draining queues are not marked as health by the health checker.                                              |

## 90.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "TenantID": "string",
    "RunnerID": "string",
    "QueueID": "string",
    "PublicKey": "string",
    "CreatedAt": "string",
    "Version": int,
    "IsHealthy": bool,
    "NConsecutiveFailedHealthChecks": int,
    "NConsecutiveSuccessfulHealthChecks": int,
    "LastHealthCheckAt": "string",
    "Draining": bool
}
```

See [here](#782-response) for more details.

# 91. DeleteRunnerQueue

The DeleteRunnerQueue API hard deletes a runner queue.

## 91.1 Request

```http request
DELETE /v1/tenants/{tenant_id}/runners/{runner_id}/queues/{queue_id} HTTP/1.1
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>
```

| Parameter                                | Location | Type    | Description                                                                 |
|------------------------------------------|----------|---------|-----------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the runner.                                  |
| runner_id                                | path     | string  | The ID of the runner the queue is registered for.                           |
| queue_id                                 | path     | string  | The ID of the queue to delete.                                              |
| Authorization                            | header   | string  | The authorization header for the request.                                   |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                      |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.         |
| Version                                  | header   | string  | The expected version of the queue. Used for optimistic concurrency control. |

## 91.2 Response

On success a 204 NO CONTENT is returned with no body.

# 92. PingRunnerQueue

PingRunnerQueue sends a "ping" message to a runner, and then synchronously waits for a response. This is used by the
health checker to determine if a queue is healthy.

## 92.1 Request

```http request
POST /v1/tenants/{tenant_id}/runners/{runner_id}/queues/{queue_id}/ping HTTP/1.1
Content-Type: application/json; charset=utf-8
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                         |
|------------------------------------------|----------|---------|---------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the runner.                          |
| runner_id                                | path     | string  | The ID of the runner to ping.                                       |
| queue_id                                 | path     | string  | The ID of the queue to ping.                                        |
| Authorization                            | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.              |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4. |

# 93. GetRunnerQueue

GetRunnerQueue fetches metadata about a runner queue.

## 93.1 Request

```http request
GET /v1/tenants/{tenant_id}/runners/{runner_id}/queues/{queue_id} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                         |
|------------------------------------------|----------|---------|---------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the runner.                          |
| runner_id                                | path     | string  | The ID of the runner that owns the queue.                           |
| queue_id                                 | path     | string  | The ID of the queue to fetch.                                       |
| Authorization                            | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.              |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4. |

## 93.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "TenantID": "string",
    "RunnerID": "string",
    "QueueID": "string",
    "PublicKey": "string",
    "CreatedAt": "string",
    "Version": int,
    "IsHealthy": bool,
    "NConsecutiveFailedHealthChecks": int,
    "NConsecutiveSuccessfulHealthChecks": int,
    "LastHealthCheckAt": "string",
    "Draining": bool
}
```

# 94. GetTaskGithubCreds

GetTaskGithubCreds retrieves the GitHub credentials associated with a task.

## 94.1 Request

```http request
GET /v1/tenants/{tenant_id}/tasks/{task_id}/github-creds?workstreamID={workstreamID} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                         |
|------------------------------------------|----------|---------|---------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the task.                            |
| task_id                                  | path     | string  | The ID of the task to fetch GitHub credentials for.                 |
| workstreamID                             | query    | *string | Optional. The workstream that owns the task when resolving a workstream task. |
| Authorization                            | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.              |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4. |

## 94.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "GithubToken": "string
}
```

# 95. ListOrgsForGithubConnection

The ListOrgsForGithubConnection API lists all organizations associated with a GitHub connection.

## 95.1 Request

```http request
GET /v1/tenants/{tenant_id}/github-connections/{connection_id}/orgs?maxResults={maxResults}&token={token}&search={search} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                                                        |
|------------------------------------------|----------|---------|----------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the GitHub connection.                                              |
| connection_id                            | path     | string  | The ID of the GitHub connection to list organizations for.                                         |
| maxResults                               | query    | *int    | Optional. The maximum number of organizations to return. Default is 10. Must be between 1 and 100. |
| token                                    | query    | *string | Optional. A token to retrieve the next page of results.                                            |
| search                                   | query    | *string | Optional. A search string to filter organization names by.                                         |
| Authorization                            | header   | string  | The authorization header for the request.                                                          |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                             |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                |

## 95.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "NextToken": "*string",
    "Items": ["string"]
}
```

| Field     | Type     | Description                                                                                    |
|-----------|----------|------------------------------------------------------------------------------------------------|
| NextToken | *string  | A token to retrieve the next page of results. If there are no more results, this will be null. |
| Items     | []string | A list of organization names associated with the GitHub connection.                            |

# 96. SearchRepos

The SearchRepos API searches for repositories within a github org associated with a GitHub connection.

## 96.1 Request

```http request
GET /v1/tenants/{tenant_id}/github-connections/{connection_id}/orgs/{org_name}/repos?search={searchStr}&maxResults={maxResults}&token={token} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                                                   |
|------------------------------------------|----------|---------|-----------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the GitHub connection.                                         |
| connection_id                            | path     | string  | The ID of the GitHub connection to search repositories for.                                   |
| org_name                                 | path     | string  | The name of the GitHub organization to search repositories in.                                |
| searchStr                                | query    | string  | The search string to filter repositories by.                                                  |
| maxResults                               | query    | *int    | Optional. The maximum number of repositories to return. Default is 10. Must be between 1 and 100. |
| token                                    |          | query   | *string                                                                                       | Optional. A token to retrieve the next page of results.                                       |
| Authorization                            | header   | string  | The authorization header for the request.                                                     |
| X-Event-Horizon-Delegating-Authorization | header   | *string |                                                                                               | The authorization header for the delegating principal.                                        |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                           |

# 96.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "NextToken": "*string",
    "Items": ["string"]
}
```

| Field     | Type     | Description                                                                                    |
|-----------|----------|------------------------------------------------------------------------------------------------|
| NextToken | *string  | A token to retrieve the next page of results. If there are no more results, this will be null. |
| Items     | []string | A list of repository names that match the search criteria.                                     |

# 96a. ListRepoBranches

The ListRepoBranches API lists branches for a repository associated with a GitHub connection.

## 96a.1 Request

```http request
GET /v1/tenants/{tenant_id}/github-connections/{connection_id}/orgs/{org_name}/repos/{repo_name}/branches?search={searchStr}&maxResults={maxResults}&token={token} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                                                   |
|------------------------------------------|----------|---------|-----------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the GitHub connection.                                         |
| connection_id                            | path     | string  | The ID of the GitHub connection to list branches for.                                         |
| org_name                                 | path     | string  | The name of the GitHub organization that owns the repository.                                 |
| repo_name                                | path     | string  | The name of the repository to list branches for.                                              |
| searchStr                                | query    | *string | Optional. When provided, only branches containing the substring are returned.                 |
| maxResults                               | query    | *int    | Optional. The maximum number of branches to return. Default is 10. Must be between 1 and 100. |
| token                                    | query    | *string | Optional. A token to retrieve the next page of results.                                        |
| Authorization                            | header   | string  | The authorization header for the request.                                                     |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                        |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                           |

## 96a.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "NextToken": "*string",
    "Items": ["string"]
}
```

| Field     | Type     | Description                                                                                    |
|-----------|----------|------------------------------------------------------------------------------------------------|
| NextToken | *string  | A token to retrieve the next page of results. If there are no more results, this will be null. |
| Items     | []string | A list of branch names for the repository.                                                     |

# 96b. GetDefaultBranches

The GetDefaultBranches API looks up the default branch for a list of org/repo pairs using a GitHub connection.
At most 50 repos may be specified per request. For public GitHub connections the API service queries GitHub
directly. For private GitHub connections the request is forwarded to the runner.

## 96b.1 Request

```http request
POST /v1/tenants/{tenant_id}/github-connections/{connection_id}/default-branches HTTP/1.1
Content-Type: application/json; charset=utf-8
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>

{
    "Repos": ["string"]
}
```

| Parameter                                | Location | Type     | Description                                                                         |
|------------------------------------------|----------|----------|-------------------------------------------------------------------------------------|
| tenant_id                                | path     | string   | The ID of the tenant that owns the GitHub connection.                               |
| connection_id                            | path     | string   | The ID of the GitHub connection to use.                                             |
| Repos                                    | body     | []string | A list of repositories in "org/repo" format. Must contain between 1 and 50 entries. |
| Authorization                            | header   | string   | The authorization header for the request.                                           |
| X-Event-Horizon-Delegating-Authorization | header   | *string  | The authorization header for the delegating principal.                              |
| X-Event-Horizon-Signed-Headers           | header   | *string  | The signed headers for the request, when authenticating with Sigv4.                 |

## 96b.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "Items": [
        {
            "Repo": "string",
            "DefaultBranch": "string"
        }
    ]
}
```

| Field                | Type   | Description                                                  |
|----------------------|--------|--------------------------------------------------------------|
| Items                | []Item | A list of results, one per requested repo.                   |
| Items[].Repo         | string | The org/repo that was looked up.                             |
| Items[].DefaultBranch| string | The name of the repository's default branch (e.g. "main").   |

# 97. ListActiveTurns

The ListActive API lists active turns across all tenants and all tasks. It's designed to be used by the plan42 timeout job.
It's an ADMIN api, and it's use is currently restricted to internal plan42 service principals.

## 97.1 Request

```http request
GET /v1/turns?maxResults={maxResults}&token={token}&partition={partition}&{minUpdatedAt}={minUpdatedAt}&{maxUpdatedAt}={maxUpdatedAt} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                      | Location | Type    | Description                                                                                |
|--------------------------------|----------|---------|--------------------------------------------------------------------------------------------|
| maxResults                     | query    | *int    | Optional. The maximum number of turns to return. Default is 10. Must be between 1 and 500. |
| token                          | query    | *string | Optional. A token to retrieve the next page of results.                                    |
| partition                      | query    | int     | The partition to retrieve active turns from. Must be >= 0.                                 |
| minUpdatedAt                   | query    | *string | Optional. The inclusive minimum updated at timestamp to filter turns by.                   |
| maxUpdatedAt                   | query    | string  | Required. The exclusive maximum updated at timestamp to filter turns by.                   |
| Authorization                  | header   | string  | The authorization header for the request.                                                  |
| X-Event-Horizon-Signed-Headers | header   | *string | The signed headers for the request, when authenticating with Sigv4.                        |

## 97.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "NextToken": "*string",
    "Items": [{
      "TenantID": "string",
      "TaskID": "string",
      "TurnIndex": int,
      "Prompt": "string",
      "PreviousResponseID": "*string",
      "CommitInfo": {}
      "BaselineCommitHash": "*string",
      "LastCommitHash": "*string",
      "Status": "string",
      "OutputMessage": "*string", 
      "ErrorMessage": "*string"
      "CreatedAt": "string",
      "UpdatedAt": "string",
      "Version": int
      "CompletedAt": "*string"
    }]
}
```

| Field     | Type                    | Description                                                                                    |
|-----------|-------------------------|------------------------------------------------------------------------------------------------|
| NextToken | *string                 | A token to retrieve the next page of results. If there are no more results, this will be null. |
| Items     | [][Turn](#232-Response) | A list of turn metadata objects representing active turns.                                     |

# 98. WriteProviderUsage

The WriteProviderUsage API records normalized provider token usage for a single completed model iteration.
The agent calls this once after each model response within a turn (one call per iteration), not once per turn,
so that per-iteration token usage is captured even when usage counts change across iterations (for example when
context compaction reduces the input token count).

## 98.1 Request

```http request
PUT /v1/tenants/{tenant_id}/tasks/{task_id}/turns/{turn_index}/iterations/{iteration_index}/provider-usage?workstreamID={workstreamID} HTTP/1.1
Content-Type: application/json; charset=utf-8
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>

{
  "Provider": "string",
  "ProviderModelID": "string",
  "ResponseID": "*string",
  "PromptTokens": int,
  "CompletionTokens": int,
  "CachedReadInputTokens": int,
  "CacheCreationInputTokensByTTL": { "<ttl_seconds>": int, ... },
  "ReasoningTokens": int,
  "RequestStartedAt": "*string",
  "ResponseCompletedAt": "string"
}
```

| Parameter                      | Location | Type            | Description                                                                                                    |
|--------------------------------|----------|-----------------|----------------------------------------------------------------------------------------------------------------|
| tenant_id                      | path     | string          | The ID of the tenant that owns the task execution.                                                             |
| task_id                        | path     | string          | The ID of the task whose provider usage is being recorded.                                                     |
| turn_index                     | path     | int             | The turn index for the execution.                                                                              |
| iteration_index                | path     | int             | The 0-based iteration index within the turn.                                                                   |
| workstreamID                   | query    | *string         | Optional. The ID of the workstream the task belongs to.                                                        |
| Authorization                  | header   | string          | The authorization header for the request.                                                                      |
| X-Event-Horizon-Signed-Headers | header   | *string         | The signed headers for the request, when authenticating with Sigv4.                                            |
| Provider                       | body     | [Provider](#982-provider) | The provider that produced the usage record.                                                         |
| ProviderModelID                | body     | string          | The concrete provider model identifier billed for the iteration, for example `claude-opus-4-6-20260115` or `gpt-5.4`. This is the underlying provider model string, distinct from the abstract [ModelType](#182-modeltype) selected at task creation, since usage attribution and pricing are keyed on the exact model the provider billed. |
| ResponseID                     | body     | *string         | Optional. The provider's response identifier, when the provider supplies one. Stored for reconciliation against provider-side billing records and for debugging. |
| PromptTokens                   | body     | int             | The number of input tokens billed at the standard (non-cached) input rate. Must exclude any tokens reported separately via `CachedReadInputTokens` or `CacheCreationInputTokensByTTL`. For OpenAI agents this means subtracting the provider's `cached_tokens` from its `prompt_tokens` before populating this field, since OpenAI reports `cached_tokens` as a subset of `prompt_tokens`. Must be >= 0. |
| CompletionTokens               | body     | int             | The number of visible completion/output tokens billed at the standard output rate. Must exclude reasoning tokens (those are reported in `ReasoningTokens`). For OpenAI reasoning models, this means subtracting `completion_tokens_details.reasoning_tokens` from the provider's `completion_tokens` before populating this field, since OpenAI reports reasoning tokens as a subset of `completion_tokens`. Must be >= 0. |
| CachedReadInputTokens          | body     | int             | The number of input tokens served from cache reads. Must be >= 0.                                              |
| CacheCreationInputTokensByTTL  | body     | map[string]int  | Map of cache-tier TTL (in seconds, as a string key) to tokens written into that tier. A single provider response can write to more than one tier at once, so multiple keys may be non-zero. For Anthropic the keys today are `"300"` (5 minutes — the default ephemeral tier, sourced from the provider's `ephemeral_5m_input_tokens`) and `"3600"` (1 hour, sourced from `ephemeral_1h_input_tokens`). New tiers can be reported without an API change. Values must be >= 0. May be empty or omitted. |
| ReasoningTokens                | body     | int             | The number of reasoning tokens emitted by the model. Should be distinct from `CompletionTokens`. Must be >= 0. |
| RequestStartedAt               | body     | *string         | Optional. The timestamp when the provider request started, in ISO 8601 format.                                 |
| ResponseCompletedAt            | body     | string          | The timestamp when the provider response completed, in ISO 8601 format. Required.                              |

Notes:

- `PromptTokens` is uncached/base input tokens only. Cache reads and cache writes are reported in their own counters so each token class can be priced independently at billing time.
- OpenAI callers must subtract the provider's `cached_tokens` from its reported `prompt_tokens` when populating `PromptTokens`, and report the difference in `CachedReadInputTokens`. For reasoning models, OpenAI callers must also subtract `completion_tokens_details.reasoning_tokens` from `completion_tokens` when populating `CompletionTokens`, and report the difference in `ReasoningTokens`. OpenAI callers should currently leave `CacheCreationInputTokensByTTL` empty.
- Anthropic callers should populate cache read and cache creation counters from the provider response when available. Anthropic's reported input token count is already net of cache reads/creations, so `PromptTokens` maps directly from it without subtraction. Cache creations are reported in `CacheCreationInputTokensByTTL` keyed by the provider's TTL bucket in seconds. When extended thinking is enabled, populate `ReasoningTokens` from the thinking-token count and ensure `CompletionTokens` reflects only the visible output.

Error responses:

- `409 Conflict / ErrorType=WorkstreamMismatch` — A usage record already exists for the same `(tenant_id, task_id, turn_index, iteration_index)` with a different `workstream_id`. The current record is returned in the conflict body. This signals an agent or upstream bug and should be alerted rather than retried.

## 98.2 Provider

Provider is an enum that defines the model provider that produced a usage record.

| Value     |
|-----------|
| anthropic |
| openai    |

## 98.3 Response

On success a 201 CREATED is returned with the following JSON body:

```http response
HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8

{
  "TenantID": "string",
  "WorkstreamID": "*string",
  "TaskID": "string",
  "TurnIndex": int,
  "IterationIndex": int,
  "Provider": "string",
  "ProviderModelID": "string",
  "ResponseID": "*string",
  "PromptTokens": int,
  "CompletionTokens": int,
  "CachedReadInputTokens": int,
  "CacheCreationInputTokensByTTL": { "<ttl_seconds>": int, ... },
  "ReasoningTokens": int,
  "RequestStartedAt": "*string",
  "ResponseCompletedAt": "string",
  "CreatedAt": "string"
}
```

| Field                          | Type            | Description                                                                                     |
|--------------------------------|-----------------|-------------------------------------------------------------------------------------------------|
| TenantID                       | string          | The ID of the tenant that owns the usage record.                                                |
| WorkstreamID                   | *string         | The workstream ID for the task, if the task belongs to a workstream.                            |
| TaskID                         | string          | The task ID that owns the usage record.                                                         |
| TurnIndex                      | int             | The turn index for the usage record.                                                            |
| IterationIndex                 | int             | The iteration index for the usage record. 0-based.                                              |
| Provider                       | string          | The provider that produced the usage.                                                           |
| ProviderModelID                | string          | The provider-specific model ID used for the iteration.                                          |
| ResponseID                     | *string         | The provider response ID, if one was supplied.                                                  |
| PromptTokens                   | int             | The recorded standard (non-cached) input token count.                                           |
| CompletionTokens               | int             | The recorded completion/output token count.                                                     |
| CachedReadInputTokens          | int             | The recorded cached read input token count.                                                     |
| CacheCreationInputTokensByTTL  | map[string]int  | Map of cache-tier TTL in seconds (string key) to tokens written into that tier.                 |
| ReasoningTokens                | int             | The recorded reasoning token count.                                                             |
| RequestStartedAt               | *string         | The timestamp when the provider request started, if supplied.                                   |
| ResponseCompletedAt            | string          | The timestamp when the provider response completed.                                             |
| CreatedAt                      | string          | The timestamp when the usage record was created.                                                |

# 99. ListProviderUsageEvents

The ListProviderUsageEvents API lists raw provider usage event records for a tenant. This API is intended for
inspection, reconciliation, and drill-down into token usage at the task/turn/iteration level.

## 99.1 Request

```http request
GET /v1/tenants/{tenant_id}/provider-usage-events?maxResults={maxResults}&token={token}&workstreamID={workstreamID}&taskID={taskID}&provider={provider}&model={model}&startTime={startTime}&endTime={endTime} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                                                                     |
|------------------------------------------|----------|---------|-----------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant whose provider usage events should be listed.                                              |
| maxResults                               | query    | *int    | Optional. The maximum number of usage events to return. Default is 10. Must be between 1 and 500.              |
| token                                    | query    | *string | Optional. A token to retrieve the next page of results.                                                         |
| workstreamID                             | query    | *string | Optional. When provided, only usage events for the specified workstream are returned.                           |
| taskID                                   | query    | *string | Optional. When provided, only usage events for the specified task are returned.                                 |
| provider                                 | query    | *string | Optional. When provided, only usage events for the specified provider are returned.                             |
| model                                    | query    | *string | Optional. When provided, only usage events for the specified provider model ID are returned.                    |
| startTime                                | query    | *string | Optional. Inclusive lower bound on `ResponseCompletedAt`, in ISO 8601 format.                                  |
| endTime                                  | query    | *string | Optional. Exclusive upper bound on `ResponseCompletedAt`, in ISO 8601 format.                                  |
| Authorization                            | header   | string  | The authorization header for the request.                                                                       |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                          |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                             |

## 99.2 Response

On success a 200 OK is returned with the following JSON body:

```http response
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "NextToken": "*string",
  "Items": [
    {
      "TenantID": "string",
      "WorkstreamID": "*string",
      "TaskID": "string",
      "TurnIndex": int,
      "IterationIndex": int,
      "Provider": "string",
      "ProviderModelID": "string",
      "ResponseID": "*string",
      "PromptTokens": int,
      "CompletionTokens": int,
      "CachedReadInputTokens": int,
      "CacheCreationInputTokensByTTL": { "<ttl_seconds>": int, ... },
      "ReasoningTokens": int,
      "RequestStartedAt": "*string",
      "ResponseCompletedAt": "string",
      "CreatedAt": "string"
    }
  ]
}
```

| Field     | Type                   | Description                                                                                    |
|-----------|------------------------|------------------------------------------------------------------------------------------------|
| NextToken | *string                | A token to retrieve the next page of results. If there are no more results, this will be null. |
| Items     | []ProviderUsageEvent   | A list of provider usage event records for the tenant.                                         |

Each `ProviderUsageEvent` object contains the fields described in [WriteProviderUsage](#98-writeproviderusage).

# 100. GetProviderUsageSummary

The GetProviderUsageSummary API returns aggregated provider usage totals for a tenant over a requested time window.
This API is intended for dashboards, reconciliation, and future billing workflows.

## 100.1 Request

```http request
GET /v1/tenants/{tenant_id}/provider-usage-summary?groupBy={groupBy}&workstreamID={workstreamID}&taskID={taskID}&provider={provider}&model={model}&startTime={startTime}&endTime={endTime} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                                                                  |
|------------------------------------------|----------|---------|--------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant whose usage should be summarized.                                                       |
| groupBy                                  | query    | [][GroupByAttribute](#1002-groupbyattribute) | Optional. The aggregation dimensions to group the summary by. Time dimensions (`hour`, `day`, `month`) are mutually exclusive within a single request. Examples: `groupBy=day,model`, `groupBy=hour`, `groupBy=month,provider`. |
| workstreamID                             | query    | *string | Optional. When provided, only usage events for the specified workstream are included.                        |
| taskID                                   | query    | *string | Optional. When provided, only usage events for the specified task are included.                              |
| provider                                 | query    | *string | Optional. When provided, only usage events for the specified provider are included.                          |
| model                                    | query    | *string | Optional. When provided, only usage events for the specified provider model ID are included.                 |
| startTime                                | query    | *string | Optional. Inclusive lower bound on `ResponseCompletedAt`, in ISO 8601 format.                               |
| endTime                                  | query    | *string | Optional. Exclusive upper bound on `ResponseCompletedAt`, in ISO 8601 format.                               |
| Authorization                            | header   | string  | The authorization header for the request.                                                                    |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                       |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                          |

## 100.2 GroupByAttribute

GroupByAttribute is an enum that defines the dimensions a usage summary can be grouped by.

| Value      |
|------------|
| hour       |
| day        |
| month      |
| provider   |
| model      |
| task       |
| workstream |

## 100.3 Response

On success a 200 OK is returned with the following JSON body:

```http response
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "TenantID": "string",
  "StartTime": "*string",
  "EndTime": "*string",
  "GroupBy": ["string", ...],
  "Items": [
    {
      "Provider": "*string",
      "ProviderModelID": "*string",
      "TaskID": "*string",
      "WorkstreamID": "*string",
      "BucketStartTime": "*string",
      "BucketGranularity": "*string",
      "EventCount": int,
      "PromptTokens": int,
      "CompletionTokens": int,
      "CachedReadInputTokens": int,
      "CacheCreationInputTokensByTTL": { "<ttl_seconds>": int, ... },
      "ReasoningTokens": int
    }
  ]
}
```

| Field                             | Type                    | Description                                                                                                       |
|-----------------------------------|-------------------------|-------------------------------------------------------------------------------------------------------------------|
| TenantID                          | string                  | The tenant the summary applies to.                                                                                |
| StartTime                         | *string                 | The inclusive lower bound used for the summary window, if supplied.                                               |
| EndTime                           | *string                 | The exclusive upper bound used for the summary window, if supplied.                                               |
| GroupBy                           | [][GroupByAttribute](#1002-groupbyattribute) | The grouping dimensions applied to the response, echoed back in the order they appear in each bucket. |
| Items                             | [][ProviderUsageSummary](#1004-providerusagesummary) | A list of summary buckets.                                                       |

This API returns normalized token counts only; it does not return currency costs.

## 100.4 ProviderUsageSummary

ProviderUsageSummary is a single aggregated usage bucket. Grouping-key fields are only populated for the dimensions
included in the request's `GroupBy`.

| Field                         | Type            | Description                                                                                            |
|-------------------------------|-----------------|--------------------------------------------------------------------------------------------------------|
| Provider                      | *string         | The provider for the bucket when grouped by `provider`.                                                 |
| ProviderModelID               | *string         | The provider model ID for the bucket when grouped by `model`.                                           |
| TaskID                        | *string         | The task ID for the bucket when grouped by `task`.                                                      |
| WorkstreamID                  | *string         | The workstream ID for the bucket when grouped by `workstream`.                                          |
| BucketStartTime               | *string         | The bucket start time when grouped by a time dimension (`hour`, `day`, or `month`).                     |
| BucketGranularity             | *string         | The time-bucket granularity applied (`hour`, `day`, or `month`) when a time dimension is in `GroupBy`.  |
| EventCount                    | int             | The number of provider usage events included in the bucket.                                            |
| PromptTokens                  | int             | The summed standard (non-cached) input tokens for the bucket.                                          |
| CompletionTokens              | int             | The summed completion/output tokens for the bucket.                                                    |
| CachedReadInputTokens         | int             | The summed cached read input tokens for the bucket.                                                    |
| CacheCreationInputTokensByTTL | map[string]int  | Per-TTL summed cache creation input tokens for the bucket, keyed by TTL in seconds.                    |
| ReasoningTokens               | int             | The summed reasoning tokens for the bucket.                                                            |

# 101. RotateTenantEncryptionKey

The RotateTenantEncryptionKey API creates a new tenant encryption key version for a tenant. The caller supplies the
next version number in the URL path. The request fails with a 409 Conflict if the version is not exactly the latest
version + 1. The API never returns key material in the response.

## 101.1 Request

```http request
PUT /v1/tenants/{tenant_id}/encryption-keys/{version} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                         |
|------------------------------------------|----------|---------|---------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant whose key should be rotated.                   |
| version                                  | path     | int     | The version number for the new key. Must equal latest version + 1.  |
| Authorization                            | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.              |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4. |

This request does not have a body.

## 101.2 Response

On success a 201 CREATED is returned with the following JSON body:

```http response
HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8

{
  "TenantID": "string",
  "Version": int,
  "CreatedAt": "string"
}
```

| Field     | Type   | Description                                        |
|-----------|--------|----------------------------------------------------|
| TenantID  | string | The ID of the tenant that owns the key.            |
| Version   | int    | The version number of the new key.                 |
| CreatedAt | string | The timestamp when the key version was created.    |

If the provided version does not match the next expected version, a 409 Conflict is returned.

# 102. GetTenantEncryptionKey

The GetTenantEncryptionKey API retrieves metadata for a specific tenant encryption key version. Key material is never
returned in the response. The request returns 404 Not Found if the tenant does not have the requested version.

## 102.1 Request

```http request
GET /v1/tenants/{tenant_id}/encryption-keys/{version} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                         |
|------------------------------------------|----------|---------|---------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant whose key metadata should be retrieved.        |
| version                                  | path     | int     | The version number of the encryption key to fetch.                  |
| Authorization                            | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.              |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4. |

This request does not have a body.

## 102.2 Response

On success a 200 OK is returned with the following JSON body:

```http response
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "TenantID": "string",
  "Version": int,
  "CreatedAt": "string"
}
```

| Field     | Type   | Description                                        |
|-----------|--------|----------------------------------------------------|
| TenantID  | string | The ID of the tenant that owns the key.            |
| Version   | int    | The version number of the requested key.           |
| CreatedAt | string | The timestamp when the key version was created.    |

If the specified version does not exist, a 404 Not Found is returned.

# 103. GetLatestTenantEncryptionKey

The GetLatestTenantEncryptionKey API retrieves metadata for the most recent tenant encryption key version. The API
returns a 404 Not Found response if the tenant does not have any encryption keys yet. Key material is never returned.

## 103.1 Request

```http request
GET /v1/tenants/{tenant_id}/encryption-keys/latest HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                         |
|------------------------------------------|----------|---------|---------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant whose key metadata should be retrieved.        |
| Authorization                            | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.              |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4. |

This request does not have a body.

## 103.2 Response

On success a 200 OK is returned with the following JSON body:

```http response
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "TenantID": "string",
  "Version": int,
  "CreatedAt": "string"
}
```

| Field     | Type   | Description                                        |
|-----------|--------|----------------------------------------------------|
| TenantID  | string | The ID of the tenant that owns the key.            |
| Version   | int    | The version number of the latest key.              |
| CreatedAt | string | The timestamp when the key version was created.    |

If the tenant does not yet have a key, a 404 Not Found error is returned.

# 104. ListTenantEncryptionKeys

The ListTenantEncryptionKeys API returns metadata for tenant encryption key versions with pagination support. Key
material is never exposed in responses.

## 104.1 Request

```http request
GET /v1/tenants/{tenant_id}/encryption-keys?maxResults={maxResults}&token={token} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                         |
|------------------------------------------|----------|---------|---------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant whose keys should be listed.                   |
| maxResults                               | query    | *int    | Maximum number of keys to return. Optional. Range: 1-500.           |
| token                                    | query    | *string | Token for retrieving the next page. Optional.                       |
| Authorization                            | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.              |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4. |

## 104.2 Response

On success a 200 OK is returned with the following JSON body:

```http response
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "Items": [
    {
      "TenantID": "string",
      "Version": int,
      "CreatedAt": "string"
    }
  ],
  "NextToken": "*string"
}
```

| Field     | Type                  | Description                                 |
|-----------|-----------------------|---------------------------------------------|
| Items     | []TenantEncryptionKey | List of tenant encryption key metadata.     |
| NextToken | *string               | Token to retrieve the next page of results. |

Each `TenantEncryptionKey` object contains the fields described in
[RotateTenantEncryptionKey](#101-rotatetenantencryptionkey).

# 105. CreateFile

The CreateFile API creates a new file object, and returns a presigned S3 URL that can be used to upload the file
contents. Te presigned S3 URL can be used to upload the file at most once, and expires after 1 hour and 5 mins.
A new upload URL cannot be created for a file. To create a new upload URL, a new file object must be created by calling
CreateFile again with a different file_id.

The file content will be deleted 35 days after the file is created.

The file may not be larger than 30 MB. If an attempt is made to upload a file larger than that, it will be rejected by S3.
This was chose because the smallest limit between Anthropic (32 MB), Google (100 MB), and Open AI(50 MB) is 32MB. A
30MB file size ensure we can send each attachment in a single message to all providers, and have room to add extra
prompts with it should we need to.

Once the file content has been uploaded to S3, it can be attached to a task when calling CreateTask, UpdateTask, CreateTurn, CreateWorkstreamTask or UpdateWorkstreamTask.

## 105.1 Request

```http request
PUT /v1/tenants/{tenant_id}/files/{file_id} HTTP/1.1
Accept: application/json
Content-Type: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>

{
    "Name": string,
    "Size": int
}
```

| Parameter                                | Location | Type    | Description                                                                                                      |
|------------------------------------------|----------|---------|------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that will own the file.                                                                     |
| file_id                                  | path     | string  | The ID of the file to create. Must be a V4 UUID.                                                                 |
| Authorization                            | header   | string  | The authorization header for the request.                                                                        |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                           |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                              |
| Name                                     | body     | string  | The name of the file. Required.                                                                                  |
| Size                                     | body     | int     | The size of the file in bytes. Required. Must be between 1 and 31457280 (30 MB). The presigned upload URL will enforce this exact size via the Content-Length header in the SigV4 signature. |

## 105.2 Response
On success a 201 CREATED is returned with the following JSON body:

```http response
HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8

{
  "TenantID": "string",
  "FileID": "string",  
  "Name" : "string",
  "Size": int,
  "UploadURL": "string",
  "CreatedAt": "string"
}
```

| Field     | Type   | Description                                                                                                                                                                                                                                                                                                          |
|-----------|--------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| TenantID  | string | The ID of the tenant that owns the file.                                                                                                                                                                                                                                                                             |
| FileID    | string | The unique ID of the file.                                                                                                                                                                                                                                                                                           |
| Name      | string | The name of the file. File names are not required to be unique. We add support for them so that you can reference file names for attachments when communicating with the LLM.                                                                                                                                        |
| Size      | int    | The size of the file in bytes.                                                                                                                                                                                                                                                                                       |
| UploadURL | string | A presigned S3 PUT URL that can be used to upload the file content. The URL is valid for 1 hour and 5 mins. The SigV4 signature includes `Content-Length` (set to the Size specified in the request) and `If-None-Match: *`, which means the upload will be rejected if the content size does not match or if the object has already been written. The file content must be uploaded to S3 before the URL expires. |
| CreatedAt | string | The timestamp when the file object was created.                                                                                                                                                                                                                                                                      |


# 106. GetDownloadUrl

The GetDownloadUrl API returns a presigned S3 URL that can be used to download file content. A presigned S3 URL is only returned if:

1. The file content has been uploaded to S3.
2. The file object was created < 35 days ago (i.e. the S3 upload has not yet been deleted).
3. A malware scan has been completed, and the file is not flagged as malicious.
4. A moderation scan has completed, and the file is not flagged as violating content policies.
5. The file has not been rejected due to an unsupported mime type.

The returned URL is valid for 1 hour and 5 mins from the time of the request.

## 106.1 Request

```http request
GET /v1/tenants/{tenant_id}/files/{file_id}/download-url HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                         |
|------------------------------------------|----------|---------|---------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the file.                            |
| file_id                                  | path     | string  | The ID of the file to get the download URL for.                     |
| Authorization                            | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.              |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4. |

## 106.2 Response

On success a 200 OK is returned with the following JSON body:

```http response
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "TenantID": "string",
  "FileID": "string",  
  "Name": "string",
  "CreatedAt": "string"
  "DownloadURL": "string",
  "DownloadURLExpiresAt": "string",
  "ContentType" : "string",  
}
```

| Field                | Type   | Description                                                                                               |
|----------------------|--------|-----------------------------------------------------------------------------------------------------------|
| TenantID             | string | The ID of the tenant that owns the file.                                                                  |
| FileID               | string | The unique ID of the file.                                                                                |
| Name                 | string | The name of the file.                                                                                     |
| CreatedAt            | string | The timestamp when the file object was created.                                                           |
| DownloadURL          | string | A presigned S3 URL that can be used to download the file content. The URL is valid for 1 hour and 5 mins. |
| DownloadURLExpiresAt | string | The timestamp when the download URL expires.                                                              |
| ContentType          | string | The file's content type.                                                                                  |

# 107. ListFiles 

The ListFiles API is an admin api that returns metadata for files owned by a tenant, with pagination support. Files are
returned in decreasing order of creation time (i.e. newest files are returned first). This API is designed to be used by
internal tools. To get the files associated with a specific task, fetch the tasks (i.e. GetTask, GetWorkstreamTask,
ListTask, ListWorkstreamTasks, etc.) and look at the `FileIDs` array.

## 107.1 Request

```http request
GET /v1/tenants/{tenant_id}/files?maxResults={maxResults}&token={token} HTTP/1.1 
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                      | Location | Type    | Description                                                         |
|--------------------------------|----------|---------|---------------------------------------------------------------------|
| tenant_id                      | path     | string  | The ID of the tenant whose files should be listed.                  |
| maxResults                     | query    | *int    | Maximum number of files to return. Optional. Range: 1-500.          |
| token                          | query    | *string | Token for retrieving the next page. Optional.                       |
| Authorization                  | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Signed-Headers | header   | *string | The signed headers for the request, when authenticating with Sigv4. |

## 107.2 Response
On success a 200 OK is returned with the following JSON body:

```http response
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "Items": [{
     "TenantID": "string",
     "FileID": "string",  
     "Name": "string",
     "Size": int,
     "CreatedAt": "string",
     "IsMalicious": *bool,
     "MalwareScanCompletedAt": "*string",
     "ModerationScanInfo": {},
	     "ModerationScanCompletedAt": *string,
	     "UpdatedAt": "*string",
	     "ContentType" : "*string",
	     "RejectionReason": "*string",
	     "RejectedAt": "*string",
	     "Version": int
	  }],
  "NextToken": "*string"
}
```

| Field     | Type                 | Description                                 |
|-----------|----------------------|---------------------------------------------|
| Items     | [[]File](#1073-file) | List of file metadata objects.              |
| NextToken | *string              | Token to retrieve the next page of results. | 

## 107.3 File

FileMetadata objects describe metadata for a file object.

| Field                     | Type                                              | Description                                                                                                       |
|---------------------------|---------------------------------------------------|-------------------------------------------------------------------------------------------------------------------|
| TenantID                  | string                                            | The ID of the tenant that owns the file.                                                                          |
| FileID                    | string                                            | The ID of the file.                                                                                               |
| Name                      | string                                            | The name of the file.                                                                                             |
| Size                      | int                                               | The size of the file in bytes.                                                                                    |
| CreatedAt                 | string                                            | The timestamp when the file object was created.                                                                   |
| IsMalicious               | *bool                                             | Whether the file was flagged as malicious by the malware. Will be null if the malware scan has not completed yet. |
| MalwareScanCompletedAt    | *string                                           | The timestamp when the malware scan completed. Will be null if the scan has not completed yet.                    |
| ModerationScanCompletedAt | *string                                           | The timestamp when the moderation scan completed.                                                                 |
| ModerationScanInfo        | [*ModerationScanInfo](#1074-moderation-scan-info) | The response from the content moderation scan. Will be null if the scan has not completed yet.                    |
| UpdatedAt                 | * string                                          | The timestamp when the file object was last modified.                                                             |
| ContentType               | *string                                           | The file's mime type.                                                                                             |
| RejectionReason           | *string                                           | The reason the file was rejected. Will be null if the file has not been rejected.                                 |
| RejectedAt                | *string                                           | The timestamp when the file was rejected. Will be null if the file has not been rejected.                         |
| Version                   | int                                               | The version number of the file object                                                                             |

## 107.4 ModerationScanInfo

ModerationScanInfo provides metadata about the content moderation scan results for a file. It's essentially a copy
of the OpenAI moderation endpoint response.

See here for OpenAI moderation API docs: https://developers.openai.com/api/docs/guides/moderation?example=images&lang=curl

| Field   | Type                                                     | Description                                  |
|---------|----------------------------------------------------------|----------------------------------------------|
| ID      | string                                                   | The Open AI moderation response ID.          |
| Model   | string                                                   | The Open AI model used for scanning.         |
| Results | [[]ModerationScanResults](#1075-moderation-scan-results) | The list of moderation results for the file. |

## 107.5 ModerationScanResults

ModerationScanResults provides details about the content moderation scan for a file, including which categories were flagged.

| Field                     | Type                | Description                                                                                                                                                                                                   |
|---------------------------|---------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Flagged                   | bool                | Whether the file was flagged by content moderation.                                                                                                                                                           |
| Categories                | map[string]bool     | The categories that were flagged. The key is the category name, and the value is whether that category was flagged.                                                                                           |
| CategoryScores            | map[string]float    | The scores for each category. The key is the category name, and the value is the score for that category.                                                                                                     |
| CategoryAppliedInputTypes | map[string][]string | The input types that contributed to each category being flagged. The key is the category name, and the value is a list of input types (e.g. "text", "image") that contributed to that category being flagged. |

# 108. DeleteFile

The DeleteFile API is an admin api that hard deletes file content from S3, but does not remove the associated metadata
entry. This is called when a file fails a content moderation scan.

## 108.1 Request

```http request
DELETE /v1/tenants/{tenant_id}/files/{file_id} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```   

| Parameter                      | Location | Type    | Description                                                         |
|--------------------------------|----------|---------|---------------------------------------------------------------------|
| tenant_id                      | path     | string  | The ID of the tenant that owns the file.                            |
| file_id                        | path     | string  | The ID of the file to delete.                                       |
| Authorization                  | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Signed-Headers | header   | *string | The signed headers for the request, when authenticating with Sigv4. |

## 108.2 Response

On success a 204 No Content is returned with an empty body. 

```http response
HTTP/1.1 204 No Content
```

# 109. GetFile

The GetFile API returns metadata about a File object.

## 109.1 Request

```http request
GET /v1/tenants/{tenant_id}/files/{file_id} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                         |
|------------------------------------------|----------|---------|---------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the file.                            |
| file_id                                  | path     | string  | The ID of the file to get metadata for.                             |
| Authorization                            | header   | string  | The authorization header for the request.                           |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.              |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4. |

## 109.2 Response

On success a 200 OK is returned with the following JSON body:

```http response
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "TenantID": "string",
  "FileID": "string",  
  "Name": "string",
  "Size": int,
  "CreatedAt": "string",
  "IsMalicious": *bool,
  "MalwareScanCompletedAt": "*string",
  "ModerationScanInfo": {},
	  "ModerationScanCompletedAt": *string,
	  "UpdatedAt": "*string",
	  "ContentType" : "*string",
	  "RejectionReason": "*string",
	  "RejectedAt": "*string",
	  "Version": int
	}
```

See [FileMetadata](#1073-file) for more info on the response.

# 110. UpdateFile

The UpdateFile API is an admin api that allows updating file metadata. It's used by internal services to update
metadata about a file as various scans complete.

## 110.1 Request

```http request
PATCH /v1/tenants/{tenant_id}/files/{file_id} HTTP/1
Accept: application/json
Content-Type: application/json
Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
If-Match: <version>

{
	  "IsMalicious": *bool,
	  "ModerationScanInfo": *ModerationScanInfo,
	  "ContentType" : *string,
	  "RejectionReason": *string,
	}
```

| Parameter                      | Location | Type                | Description                                                                                                                                          |
|--------------------------------|----------|---------------------|------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                      | path     | string              | The ID of the tenant that owns the file.                                                                                                             |
| file_id                        | path     | string              | The ID of the file to update.                                                                                                                        |
| Authorization                  | header   | string              | The authorization header for the request.                                                                                                            |
| X-Event-Horizon-Signed-Headers | header   | *string             | The signed headers for the request, when authenticating with Sigv4.                                                                                  |
| version                        | header   | string              | The version of the file to update. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned. |
| IsMalicious                    | body     | *bool               | Optiopnal. When set marks the file as malicious or non-malicious. Cannot be modified once set.                                                       |
| ModerationScanInfo             | body     | *ModerationScanInfo | Optional. Sets the results of a moderations can. Cannot be modified once set.                                                                        |
| ContentType                    | body     | *string             | Optional. Sets the file's mime type. This is set by a lambda, using libmagic, after the file's malware scan has completed.                           |
| RejectionReason                | body     | *string             | Optional. Sets the reason the file was rejected. This is set by a lambda when the detected mime type is not supported. Cannot be modified once set. |

## 110.2 Response

On success a 200 OK is returned with the following JSON body:

```http response
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "TenantID": "string",
  "FileID": "string",  
  "Name": "string",
  "Size": int,
  "CreatedAt": "string",
  "IsMalicious": *bool,
  "MalwareScanCompletedAt": "*string",
  "ModerationScanInfo": {},
  "ModerationScanCompletedAt": *string,
  "UpdatedAt": "*string",
  "ContentType" : "*string",
  "Version": int
}
```   

See [FileMetadata](#1073-file) for more info on the response.
