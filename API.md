# 1. Authentication

We support authentication using either JWT tokens or Sigv4 Auth using AWS IAM credentials. For JWT, we support the
following types of tokens:

1. Web UI Tokens
2. Auth Provider Tokens (i.e. Google Identity Tokens)
3. Service Account Tokens
4. Agent Tokens

## 1.1 Web UI Tokens

Web UI Tokens are JWT tokens signed by Event Horizon that authenticate users of the web ui. When users login,
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

Service Account Tokens are JWT tokens signed by Event Horizon that authenticate automation scripts that interact with
the API. Service Account Tokens are typically long-lived (up to 366 days). 

If you automation has access to AWS IAM credentials, consider using Sigv4 Auth instead.

## 1.4 Agent Tokens

Agent Tokens are JWT tokens signed by Event Horizon that authenticate agents that run tasks on behalf of users.
They are used to update turn status and to upload turn logs. 

## 1.5 Sigv4 Auth

Sigv4 Auth uses AWS IAM Role credentials to sign requests to the API using Sigv4. For automation scripts that have access to
AWS, this is the preferred method of authentication, as it does not require explicit secret management or rotation.

This is also the mechanism Event Horizon uses internally, for example, to authenticate between the web ui and the API.

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
| sts:GetCallerIdentity | Used for Sigv4 authentication. The token should be the base64 encoding of a a valid signed http request to sts:GetCallerIdentity. See https://github.com/debugging-sucks/sigv4util for details. |
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

   When a user logs in to Event Horizon for the first time, a new tenant is created for them. Resources created under
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
  "DefaultGithubConnectionID": "*string"
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
  "DefaultGithubConnectionID": "*string"  
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

| Value          | Description                                                                                                                                                                                                                                            |
|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| User           | A human user.                                                                                                                                                                                                                                          |
| IAMRole        | An AWS IAM Role authenticating via Sigv4.                                                                                                                                                                                                              |
| Service        | An named alias for an IAM Role. This is used to enable policies to refer to event horizon services without exposing our role arns to customers (which would make it impossible to ever change them). Valid Services names are 'WebUI' and 'AdminRole'. |
| ServiceAccount | A service account.                                                                                                                                                                                                                                     |
| Agent          | An executing agent invocation.                                                                                                                                                                                                                         |
| Runner         | A runner instance.                                                                                                                                                                                                                                     |

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
| RegisterRunnerInstance    |
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

## 8.5 AgentAccess

```json
{
    "Name": "AgentAccess",
    "Effect": "Allow",
    "Tenant": "tenant_id",
    "Principal": {
        "Type": "Agent"
    },
    "Actions": ["UpdateTurn", "UpdateTask", "GetTask", "GetTurn", "UploadTurnLogs"],
    "Constraints": [
        "$request.Tenant == $policy.Tenant",
        "$request.Tenant == $Principal.Tenant",
        "$request.TaskID == $Principal.TaskID",
        "$request.TurnID == $Principal.TurnID"
    ]
}
```

## 8.6 GetCurrentUser

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

## 8.7 GetCurrentUser

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

# 9. Default Organization Tenant Policies

This section defines the default policies that are created when an organization tenant is created.

## 9.1 EnableWebUIDelegation

This enables the Web UI to perform delegated actions on an Organization by members of the organization that authenticate via Web UI Tokens.

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
    "Organization" : "$policy.Tenant",
    "TokenTypes": ["WebUIToken"]
  }
}
```

## 9.2 OwnerAccess

This policy allows owners of an organization to perform any action on the organization.

```json
{
  "Name": "OwnerAccess",
  "Effect": "Allow",
  "Tenant": "tenant_id",
  "Principal": {
    "Type": "User",
    "Tenant": "*", 
    "Organization" : "$policy.Tenant",
    "OrganizationRole": "Owner"
  },
  "Actions": ["*"]
}
```

## 9.3 MemberAccess

This policy allows members of an organization to perform non-admin actions on the organization.

> TODO: Define the actions members are allowed to perform. Right now the list is empty, which means no actions are allowed.

```json
{
  "Name": "MemberAccess",
  "Effect": "Allow",
  "Tenant": "tenant_id",
  "Principal": {
    "Type": "User", 
    "Tenant": "*",
    "Organization" : "$policy.Tenant",
    "OrganizationRole": "Member"
  },
  "Actions": [
     ...
  ]
}
```

## 9.4 TaskAccess

```json
{
    "Name": "AgentAccess",
    "Effect": "Allow",
    "Tenant": "tenant_id",
    "Principal": {
        "Type": "Agent"
    },
    "Actions": ["UpdateTurn", "UpdateTask", "GetTask", "GetTurn", "UploadTurnLogs"],
    "Constraints": [
        "$request.Tenant == $policy.Tenant",
        "$request.Tenant == $principal.Tenant",
        "$request.TaskID == $principal.TaskID",
        "$request.TurnID == $principal.TurnID"
    ]
}
```

# 10. Default Enterprise Tenant Policies

This section defines the default policies that are created when an enterprise tenant is created.

## 10.1 EnableWebUIDelegation

This enables the Web UI to perform delegated actions on an enterprise by members of the enterprise that authenticate via Web UI Tokens.

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
    "Enterprise" : "$policy.Tenant",
    "TokenTypes": ["WebUIToken"]
  }
}
```

## 10.2 OwnerAccess

This policy allows owners of an enterprise to perform any action on the enterprise.

```json
{
  "Name": "OwnerAccess",
  "Effect": "Allow",
  "Tenant": "tenant_id",
  "Principal": {
    "Type": "User",
    "Enterprise" : "$policy.Tenant",
    "EnterpriseRole": "Owner"
  },
  "Actions": ["*"]
}
```

## 10.3 MemberAccess

This policy allows members of an enterprise to perform non-admin actions on the enterprise.

> TODO: Define the actions members are allowed to perform. Right now the list is empty, which means no actions are allowed.

```json
{
  "Name": "MemberAccess",
  "Effect": "Allow",
  "Tenant": "tenant_id",
  "Principal": {
    "Type": "User",
    "Enterprise" : "$policy.Tenant",
    "EnterpriseRole": "Member"
  },
  "Actions": [
  ]
}
```

## 10.4 AgentAccess

```json
{
    "Name": "AgentAccess",
    "Effect": "Allow",
    "Tenant": "tenant_id",
    "Principal": {
        "Type": "Agent"
    },
    "Actions": ["UpdateTurn", "UpdateTask", "GetTask", "GetTurn", "UploadTurnLogs"],
    "Constraints": [
        "$request.Tenant == $policy.Tenant",
        "$request.Tenant == $principal.Tenant",
        "$request.TaskID == $principal.TaskID",
        "$request.TurnID == $principal.TurnID"
    ]
}
```

# 11. Default Runner Policies

When a new runner is created, we create a policy that enables runner tokens for that runner to:

1. Register new runner instances
2. Get batches of messages
3. Write runner responses.

```json
{
  "Name": "Runner-<runner_id>",
  "Effect": "Allow",
  "Tenant": "tenant_id",
  "Principal": {
    "Type": "Runner",
    "Tenant": "$policy.Tenant",
    "RunnerID": "<runner_id>",
  },
  "Actions" : [
    "RegisterRunnerInstance",
    "GetMessagesBatch",
    "WriteResponse"
  ],
  "Constraints": [
    "$request.Tenant == $policy.Tenant",
    "$request.RunnerID == $policy.Principal.RunnerID"
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
    "RunnerID" : "string",
    "GithubConnectionID" : "string"
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
| DockerImage                              | body     | *string                 | The Docker image to use for the environment. Optional. Defaults to the latest event horizon agent wrapper image.                                                                                                      |
| AllowedHosts                             | body     | []string                | A list of outbound hostnames the environment is allowed to connect to. Only TLS connections to hosts with public trusted certs or internal event-horizon oss mirrors are allowed.  At most 50 hosts can be specified. |
| EnvVars                                  | body     | [][EnvVar](#132-envvar) | A list of environment variables to set in the environment. At most 50 env vars may be specified.                                                                                                                      |
| RunnerID                                 | body     | string                  | The ID of the runner to use for the environment. Must be the id of a runner or the value "default".                                                                                                                   |
| GithubConnectionID                       | body     | string                  | The ID of the GitHub connection to use for checking out code when running tasks in this environment.Must be the ID of a Github Connection or the value "default".                                                     |

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
}
```

| Parameter                                | Location | Type                                  | Description                                                                     |
|------------------------------------------|----------|---------------------------------------|---------------------------------------------------------------------------------|
| tenant_id                                | path     | string                                | The ID of the tenant to create the task for.                                    |
| task_id                                  | path     | string                                | The ID of the task to create. This must be a v4 UUID.                           |
| Authorization                            | header   | string                                | The authorization header for the request.                                       |
| X-Event-Horizon-Delegating-Authorization | header   | *string                               | The authorization header for the delegating principal.                          |
| X-Event-Horizon-Signed-Headers           | header   | *string                               | The signed headers for the request, when authenticating with Sigv4.             |
| Title                                    | body     | string                                | The title of the task.                                                          |
| EnvironmentID                            | body     | string                                | The ID of the environment to execute the task in.                               |
| Prompt                                   | body     | string                                | The prompt to use for the task.                                                 |
| Model                                    | body     | [ModelType](#182-modeltype)           | The model to use for the task. Required if the task is not assigned to a human. |
| RepoInfo                                 | body     | map[string][*RepoInfo](#185-repoinfo) | A map of "org/repo" to repo info.                                               |

## 18.2 ModelType

ModelType is an enum that defines the type of model to use for the task. 

| Value           |
|-----------------|
| Codex Mini      |
| O3              |
| O3 Pro          |
| Claude 4 Opus   |
| Claude 4 Sonnet |

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
  "State": "TaskState",
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "Deleted": bool,
  "Version": int,
  "TaskNumber": int
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
| State              | [TaskState](#186-taskstate)             | The current state of the task.                                                                                                                                        |
| CreatedAt          | string                                  | The timestamp when the task was created, in ISO 8601 format.                                                                                                          |
| UpdatedAt          | string                                  | The timestamp when the task was last updated, in ISO 8601 format.                                                                                                     |
| Deleted            | bool                                    | Whether the task has been deleted.                                                                                                                                    |
| Version            | int                                     | The version of the task. This is incremented every time the task is updated.                                                                                          |
| TaskNumber         | *int                                    | The number of the task within the workstream. This is a sequential number assigned when the task is created. Is nil if the task is not part of a workstream.          |

## 18.5 RepoInfo

RepoInfo is an object that contains information about a repository used in a task's environment.


```json
{
   "PRLink": "*string",
   "PRID": "*string",
   "PRNumber": *int,
   "FeatureBranch": "string",
   "TargetBranch": "string"
}
```

| Field         | Type    | Description                                                                                                     |
|---------------|---------|-----------------------------------------------------------------------------------------------------------------|
| PRLink        | *string | The link to the pull request for the feature branch. Will be null if no pr has been generated.                  |
| PRID          | *string | The ID of the pull request for the feature branch. Will be null if no pr has been generated.                    |
| PRNumber      | *int    | The number of the pull request for the feature branch. Will be null if no pr has been generated.                |
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
  "State": "TaskState",
  "CreatedAt": "string",
  "UpdatedAt": "string",
  "Deleted": bool,
  "Version": int
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
    "Deleted": *bool
}
```

| Parameter                                | Location | Type                   | Description                                                                                                                                          |
|------------------------------------------|----------|------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string                 | The ID of the tenant to update the task for.                                                                                                         |
| task_id                                  | path     | string                 | The ID of the task to update.                                                                                                                        |
| Authorization                            | header   | string                 | The authorization header for the request.                                                                                                            |
| X-Event-Horizon-Delegating-Authorization | header   | *string                | The authorization header for the delegating principal.                                                                                               |
| X-Event-Horizon-Signed-Headers           | header   | *string                | The signed headers for the request, when authenticating with Sigv4.                                                                                  |
| version                                  | header   | string                 | The version of the task to update. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned. |
| Title                                    | body     | *string                | If set, update the task's title.                                                                                                                     |
| Prompt                                   | body     | *string                | If set, update the task's prompt.                                                                                                                    |
| Model                                    | body     | *ModelType             | If set, update the task's model type.                                                                                                                |
| RepoInfo                                 | body     | map[string][*RepoInfo] | If set, update the task's repository info. This tracks branch names and PR links for each repo used in the environment.                              |
| Deleted                                  | body     | *bool                  | If set to false, undelete the task. May not be set to true. Use DeleteTask instead.                                                                  |

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
  "TaskNumber": int
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
    "Prompt": "string"    
}
```

| Parameter                                | Location | Type    | Description                                                                                                                                                                                                                   |
|------------------------------------------|----------|---------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant to create the turn for.                                                                                                                                                                                  |
| task_id                                  | path     | string  | The ID of the task to create the turn for.                                                                                                                                                                                    |
| turnIndex                                | path     | int     | The index of the turn to create. This must be the next index in the sequence (i.e. latest turn index + 1).                                                                                                                    |
| Authorization                            | header   | string  | The authorization header for the request.                                                                                                                                                                                     |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                                                                                                                                        |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                                                                                                                           |
| taskVersion                              | header   | string  | The version of the task to create the turn for. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned. Adding a turn to a task will increment it's version number. |
| Prompt                                   | body     | string  | The prompt to use for the turn.                                                                                                                                                                                               |

## 23.2 Response

On success a 201 CREATED is returned with the following JSON body:

```http request
HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8

{
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
}
```

| Field              | Type                                     | Description                                                                                                                                                             |
|--------------------|------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| TenantID           | string                                   | The ID of the tenant that owns the turn.                                                                                                                                |
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
GET /v1/tenants/{tenant_id}/tasks/{task_id}/turns?maxResults={maxResults}&token={token}&includeDeleted={includeDeleted} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                                                                     |
|------------------------------------------|----------|---------|-----------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant to list turns for.                                                                         |
| task_id                                  | path     | string  | The ID of the task to list turns for.                                                                           |
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
GET /v1/tenants/{tenant_id}/tasks/{task_id}/turns/{turnIndex}?includeDeleted={includeDeleted} HTTP/1.1
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
  "CompletedAt": "*string"S
}
```

See [CreateTurn](#232-response) for details on the response fields.

# 26. GetLastTurn

GetLastTurn retrieves the last turn for a task. This is useful for quickly getting the most recent turn without having
to list all turns.

## 26.1 Request

```http request
GET /v1/tenants/{tenant_id}/tasks/{task_id}/turns/last?includeDeleted={includeDeleted} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                         |
|------------------------------------------|----------|---------|---------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant to get the last turn for.                      |
| task_id                                  | path     | string  | The ID of the task to get the last turn for.                        |
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

StreamTurns logs streams for a turn using Server-Sent Events (SSE). 

## 28.1 Request

```http request
GET /v1/tenants/{tenant_id}/tasks/{task_id}/turns/{turnIndex}/logs?includeDeleted={includeDeleted} HTTP/1.1
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
GET /v1/tenants/{tenant_id}/tasks/{task_id}/turns/{turnIndex}/logs/last?includeDeleted={includeDeleted} HTTP/1.1
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

| Parameter                                | Location | Type    | Description                                                                                                     |
|------------------------------------------|----------|---------|-----------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant to list workstreams for.                                                                   |
| maxResults                               | query    | *int    | Optional. The maximum number of workstreams to return. Default is 10. Must be >=1 and <= 500.                   |
| token                                    | query    | *string | Optional. A token to retrieve the next page of results. If not provided, the first page of results is returned. |
| includeDeleted                           | query    | *bool   | Optional. Set to true to include deleted workstreams in the results.                                            |
| shortName                                | query    | *string | Optional. Searches for the workstream with the provides short name.                                             |
| Authorization                            | header   | string  | The authorization header for the request.                                                                       |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                          |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                             |

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
eh-ctl feature-flag add -f <flag-name> -D <description> -p <default-pct>
```

This will create a new feature flag with the given name, description and default percentage. The flag name must be
unique. The default percentage must be between 0.0 and 1.0.

## 53.2 Explicitly enabling a feature flag for a tenant

```bash
eh-ctl feature-flag override -i <tenant-id> -f <flag-name> -e
```

This will create a new feature flag override for the given tenant and flag name. If an override already exists, it will
be updated. 

## 53.3 Explicitly disabling a feature flag for a tenant

```bash
eh-ctl feature-flag override -t <tenant-id> -f <flag-name>
```

This adds an override, but doesn't pass "-e / --enable".

## 53.4 Removing an explicit override for a tenant

```bash
eh-ctl feature-flag delete-override -t <tenant-id> -f <flag>
```

This will delete the feature flag override for the given tenant and flag name. After this, the feature flag will be
enabled or disabled based on the default percentage and the tenant ID hash.

## 53.5 Deleting a feature flag

```bash
eh-ctl feature-flag delete -f <flag-name>
```

## 53.6 Undeleting a feature flag

```bash
eh-ctl feature-flag update -f <flag-name> <<EOF
{
  "Deleted": false
}
EOF
```

## 53.7 Updating a feature flag default percentage

```bash
eh-ctl feature-flag update -f <flag-name> <<EOF
{
  "DefaultPct": 0.25
}
EOF
```

You can also combine undelete / default percentage updates in a single call.

```bash
eh-ctl feature-flag update -f <flag-name> <<EOF
{
  "DefaultPct": 0.25,
  "Deleted": false
}
EOF
```

## 53.8 Getting effective feature flags for a tenant

```bash
eh-ctl feature-flag get-tenant-flags -t <tenant-id>
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
  "Name": "string",
  "WorkstreamID": "string",
  "WorkstreamVersion": int,
}
```

| Field             | Type   | Description                                                 |
|-------------------|--------|-------------------------------------------------------------|
| Name              | string | The short name.                                             |
| WorkstreamID      | string | The ID of the workstream the short name is associated with. |
| WorkstreamVersion | int    | The version of the workstream.                              |
 
# 57. MoveTask

The MoveTask API moves a task from one workstream to another.

## 57.1 Request

```http request
POST /v1/tenants/{tenant_id}/tasks/{task_id}/move HTTP/1.1
Content-Type: application/json; charset=utf-8
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>

{
    "DestinationWorkstreamID": "string"
    "TaskVersion": int,
    "SourceWorkstreamVersion": int,
    "DestinationWorkstreamVersion": int
}
```

| Parameter                                | Location | Type    | Description                                                                                                                                                  |
|------------------------------------------|----------|---------|--------------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the task.                                                                                                                     |
| task_id                                  | path     | string  | The ID of the task to move.                                                                                                                                  |
| Authorization                            | header   | string  | The authorization header for the request.                                                                                                                    |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                                                                       |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                                                          |
| DestinationWorkstreamID                  | body     | string  | The ID of the workstream to move the task to.                                                                                                                |
| TaskVersion                              | body     | int     | The version of the task to move. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned.           |
| SourceWorkstreamVersion                  | body     | int     | The version of the source workstream. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned.      |
| DestinationWorkstreamVersion             | body     | int     | The version of the destination workstream. This is used for optimistic concurrency control. If the version does not match, a 409 Conflict error is returned. |

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
    "State": "*TaskState"
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
    "TaskNumber": int
}
```

# 61. ListWorkstreamTasks

The ListWorkstreamTasks API lists tasks in a workstream. Results are always ordered by increasing rank. To change
the order of tasks use the UpdateWorkstreamTask API and set one of the BeforeTaskID or AfterTaskID fields.

## 61.1 Request

```http request
GET /v1/tenants/{tenant_id}/workstreams/{workstream_id}/tasks?maxResults={maxResults}&token={token}&includeDeleted={includeDeleted} HTTP/1.1
Accept: application/json
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>
```

| Parameter                                | Location | Type    | Description                                                                                                     |
|------------------------------------------|----------|---------|-----------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the workstream.                                                                  |
| workstream_id                            | path     | string  | The ID of the workstream to list tasks for.                                                                     |
| maxResults                               | query    | *int    | Optional. The maximum number of tasks to return. Default is 10. Must be >=1 and <= 500.                         |
| token                                    | query    | *string | Optional. A token to retrieve the next page of results. If not provided, the first page of results is returned. |
| includeDeleted                           | query    | *bool   | Optional. Set to true to include deleted tasks in the results.                                                  |
| Authorization                            | header   | string  | The authorization header for the request.                                                                       |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                          |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                             |

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
    "Deleted": "*bool"
}
```

| Parameter                                | Location | Type                         | Description                                                                                                              |
|------------------------------------------|----------|------------------------------|--------------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string                       | The ID of the tenant that owns the workstream.                                                                           |
| workstream_id                            | path     | string                       | The ID of the workstream that owns the task.                                                                             |
| task_id                                  | path     | string                       | The ID of the task to update.                                                                                            |
| Authorization                            | header   | string                       | The authorization header for the request.                                                                                |
| X-Event-Horizon-Delegating-Authorization | header   | *string                      | The authorization header for the delegating principal.                                                                   |
| X-Event-Horizon-Signed-Headers           | header   | *string                      | The signed headers for the request, when authenticating with Sigv4.                                                      |
| Version                                  | header   | string                       | The expected version of the task. Used for optimistic concurrency control.                                               |
| Title                                    | body     | *string                      | Optional. When set, updates the task title.                                                                              |
| EnvironmentID                            | body     | **string                     | Optional. When set, updates the task's environment.                                                                      |
| Prompt                                   | body     | *string                      | Optional. When                                                                                                           |
| Parallel                                 | body     | *bool                        | Optional. If set, updates whether the task is marked as parallel.                                                        |
| Model                                    | body     | *[ModelType](#182-modeltype) | Optional. The new AI model of the task.                                                                                  |
| AssignedToTenantID                       | body     | *string                      | Optional. When set, updates the tenant the task is assigned to.                                                          |
| AssignedToAI                             | body     | *bool                        | Optional. When set, updates whether the task is assigned to AI.                                                          |
| RepoInfo                                 | body     | *[RepoInfo](#185-repoinfo)   | Optional. When set, updates the repository information of the task.                                                      |
| State                                    | body     | *[TaskState](#186-taskstate) | Optional. When set, updates the state of the task.                                                                       |
| BeforeTaskID                             | body     | *string                      | Optional. If set, moves the task to be ordered before the given task. Cannot be combined with AfterTaskID.               |
| AfterTaskID                              | body     | *string                      | Optional. If set, moves the task to be ordered after the given task. Cannot be combined with BeforeTaskID.               |
| Deleted                                  | body     | *bool                        | Optional. If false, undeletes the task. To delete a task, call the [DeleteWorkstreamTask](#66-deleteworkstreamtask) api. |

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
    "TaskNumber": int
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
    "GithubUserID" : *int
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

# 71. ListGithubConnections

The ListGithubConnections API lists GitHub connections associated with a tenant.

## 71.1 Request

```http request
GET /v1/tenants/{tenant_id}/github-connections?maxResults={maxResults}&token={token} HTTP/1.1
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
    "State"  : "*string",
    "StateExpiry" : "*string"        
}
```

| Parameter                                | Location | Type    | Description                                                                                       |
|------------------------------------------|----------|---------|---------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant that owns the GitHub connection.                                             |
| connection_id                            | path     | string  | The ID of the GitHub connection to update.                                                        |
| Authorization                            | header   | string  | The authorization header for the request.                                                         |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                            |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                               |
| Version                                  | header   | string  | The expected version of the connection. Used for optimistic concurrency control.                  |
| Private                                  | body     | *bool   | Optional. When set, updates whether the connection is private.                                    |
| RunnerID                                 | body     | *string | Optional. When set, updates the ID of the runner associated with the connection.                  |
| GithubUserLogin                          | body     | *string | Optional. When set, updates the GitHub user login associated with the connection.                 |
| GithubUserID                             | body     | *int    | Optional. When set, updates the GitHub user ID associated with the connection.                    |
| OAuthToken                               | body     | *string | Optional. When set, updates the OAuth token for the connection.                                   |
| RefreshToken                             | body     | *string | Optional. When set, updates the refresh token for the connection.                                 |
| State                                    | body     | *string | Optional. When set, updates the state parameter for OAuth flows. Set to "" to clear the value.    |
| StateExpiry                              | body     | *string | Optional. When set, updates the expiry time of the state parameter. Set to "" to clear the value. |

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

# 78. RegisterRunnerInstance

The RegisterRunnerInstance API registers a new runner instance. 

When customers deploy runners in their own environments, they can deploy multiple instances to
provide high availability and load balancing. These instances all correspond to a single logical runner. Each time
the runner software starts up, it will generate a random instance ID and register its self. Instances fetch messages by
calling GetMessageBatch. Each call to GetMessageBatch also heartbeats that instances. If an instance
goes 60s without heart beating, it will be marked as unhealthy and traffic will no longer be routed to it.  

New instances are available for traffic immediately upon registration. 

Instances that become unhealthy must heart beat successfully 10 times (150 seconds) before traffic will be routed to
them. Unlike a load balancer, we do not implement a mode where we route traffic to all instances when they are all unhealthy.

We enforce that the PublicKey provided is unique and has not been seen by Plan 42 before. Any attempt to re-use
an existing PublicKey will result in a 404 BAD REQUEST error.

## 78.1 Request

```http request
PUT /v1/tenants/{tenant_id}/runners/{runner_id}/instances/{instance_id} HTTP/1.1
Content-Type: application/json; charset=utf-8
Authorization: <authorization>

{
    "PublicKey": "string"
}
```

| Parameter     | Location | Type   | Description                                        |
|---------------|----------|--------|----------------------------------------------------|
| tenant_id     | path     | string | The ID of the tenant that owns the runner.         |
| runner_id     | path     | string | The ID of the runner to register the instance for. |
| instance_id   | path     | string | The ID of the instance to register.                |
| Authorization | header   | string | The authorization header for the request.          |
| PublicKey     | body     | string | The PEM encoded public key of the instance.        |

Note that this api does not support delegation.

## 78.2 Response

On success a 201 CREATED is returned with the following JSON body:

```http request
HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8

{
    "TenantID": "string",
    "RunnerID": "string",
    "InstanceID": "string",
    "PublicKey": "string",
    "RegisteredAt": "string",
    "LastHeartBeatAt": "string",
    "IsHealthy": bool
}
```

| Field           | Type   | Description                                          |
|-----------------|--------|------------------------------------------------------|
| TenantID        | string | The ID of the tenant that owns the runner.           |
| RunnerID        | string | The ID of the runner the instance is registered for. |
| InstanceID      | string | The ID of the registered instance.                   |
| PublicKey       | string | The PEM encoded public key of the instance.          |
| RegisteredAt    | string | The timestamp when the instance was registered.      |
| LastHeartBeatAt | string | The timestamp when the instance last heart beated.   |
| IsHealthy       | bool   | Whether the instance is currently healthy.           | 

# 79. GetMessagesBatch

The GetMessagesBatch API retrieves a batch of messages for a runner instance.

We use "at most once" semantics for messages. Messages are implemented asynchronously because the api service
(which sends messages to runners) doesn't have line of site the them. However, messages are used in syncronous request
response scenarios (for example, when calling a github api to search for repos, or to start a task job).

We don't implement "at least once" semantics because by the time we attempt to re-drive delivery, the original
caller would have already timed out and marked it's request as failed.

So, when messages are returned from this API, the are deleted from the instance's queue before they are returned.

## 79.1 Request

```http request
POST /v1/tenants/{tenant_id}/runners/{runner_id}/instances/{instance_id}/messages/batch HTTP/1.1
Content-Type: application/json; charset=utf-8
Accept: application/json
Authorization: <authorization>
```

| Parameter     | Location | Type   | Description                                          |
|---------------|----------|--------|------------------------------------------------------|
| tenant_id     | path     | string | The ID of the tenant that owns the runner.           |
| runner_id     | path     | string | The ID of the runner the instance is registered for. |
| instance_id   | path     | string | The ID of the instance to retrieve messages for.     |
| Authorization | header   | string | The authorization header for the request.            |

Note that this api does not support delegation.

## 79.2 Response
On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
    "Messages": [{
        "CallerID": "string",
        "MessageID": "string",
        "MessageType": "string",
        "CreatedAt": "string",
        "CallerPublicKey": "string",
        "Payload": "string",
    }]
}
```

| Field         | Type                                  | Description                                                                   |
|---------------|---------------------------------------|-------------------------------------------------------------------------------|
| Messages      | [][RunnerMessage](#793-RunnerMessage) | A list of messages for the runner instance. At most 10 messages are returned. |

## 79.3 RunnerMessage
The RunnerMessage object contains information about a message sent to a runner instance.

| Field           | Type   | Description                                                                                                                                                                |
|-----------------|--------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| CallerID        | string | The ID of the caller that sent the message.                                                                                                                                |
| MessageID       | string | The ID of the message.                                                                                                                                                     |
| MessageType     | string | The type of the message.                                                                                                                                                   |
| CreatedAt       | string | The timestamp when the message was created.                                                                                                                                |
| CallerPublicKey | string | The PEM encoded public key of the caller that sent message.                                                                                                                |
| Payload         | string | The base64 encoded payload of the message. The payload is encrypted using ECIES, with a key dervied from the private key of the caller and the public key of the instance. |

# 80. WriteResponse

The WriteResponse API is used by a runner instance to respond to a message it received via a call to GetMessagesBatch.

## 80.1 Request

```http request
PUT /v1/tenants/{tenant_id}/runners/{runner_id}/instances/{instance_id}/messages/{message_id}/response HTTP/1.1
Content-Type: application/json; charset=utf-8
Authorization: <authorization>

{
    "CallerID": "string",
    "Payload": "string"
}
```

| Parameter     | Location | Type   | Description                                                                                                                                                                 |
|---------------|----------|--------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| tenant_id     | path     | string | The ID of the tenant that owns the runner.                                                                                                                                  |
| runner_id     | path     | string | The ID of the runner the instance is registered for.                                                                                                                        |
| instance_id   | path     | string | The ID of the instance responding to the message.                                                                                                                           |
| message_id    | path     | string | The ID of the message being responded to.                                                                                                                                   |
| Authorization | header   | string | The authorization header for the request.                                                                                                                                   |
| CallerID      | body     | string | The ID of the caller that sent the original message.                                                                                                                        |
| Payload       | body     | string | The base64 encoded payload of the response. The payload is encrypted using ECIES, with a key dervied from the private key of the instance and the public key of the caller. |

Note that this api does not support delegation.

## 80.2 Response
On success a 204 NO CONTENT is returned with no body.

# 81. SearchTasks

The SearchTasks API searches for tasks within a tenant. Currently the only supported search criterion is a GitHub pull request ID.

## 81.1 Request

```http request
POST /v1/tenants/{tenant_id}/tasks/search?pullRequestId={pullRequestId} HTTP/1.1
Accept: application/json
Content-Type: application/json; charset=utf-8
Authorization: <authorization>
X-Event-Horizon-Delegating-Authorization: <authorization>
X-Event-Horizon-Signed-Headers: <signed headers>

{}
```

| Parameter                                | Location | Type    | Description                                                                                                           |
|------------------------------------------|----------|---------|-----------------------------------------------------------------------------------------------------------------------|
| tenant_id                                | path     | string  | The ID of the tenant to search.                                                                                       |
| pullRequestId                            | query    | *int    | The GitHub pull request ID to search for. Required when searching by pull request.                                    |
| Authorization                            | header   | string  | The authorization header for the request.                                                                             |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                                |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                   |

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

Requests that do not match any tasks return `Tasks: []` with `NextToken` unset. If the supplied `pullRequestId` is invalid or the caller lacks access to the tenant, standard error responses are returned.

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
