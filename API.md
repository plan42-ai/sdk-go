# 1. Authentication

We support authentication using either JWT tokens or Sigv4 Auth using AWS IAM credentials. For JWT, we support the
following types of tokens:

1. Web UI Tokens
2. Auth Provider Tokens (i.e. Google Identity Tokens)
3. Service Account Tokens

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
2. To create Web UI tokens via [GenerateWebUIToken](#10-generatewebuitoken).

## 1.3 Service Account Tokens

Service Account Tokens are JWT tokens signed by Event Horizon that authenticate automation scripts that interact with
the API. Service Account Tokens are typically long-lived (up to 366 days). 

If you automation has access to AWS IAM credentials, consider using Sigv4 Auth instead.

## 1.4 Sigv4 Auth

Sigv4 Auth uses AWS IAM Role credentials to sign requests to the API using Sigv4. For automation scripts that have access to
AWS, this is the preferred method of authentication, as it does not require explicit secret management or rotation.

This is also the mechanism Event Horizon uses internally, for example, to authenticate between the web ui and the API.

## 1.5 Delegation

We support "delegated authentication". When ever a service (like the Web UI) performs an action on behalf of a user,
it will supply both its own authentication information, and that of the user it is acting on behalf of (the delegating principal).
Both the credentials of the calling principal and that of the delegating principal will be verified.

When authorizing the request, the api will verify the following:

1. That the "delegating principal" has permission to perform the requested operation.
2. That the "calling principal" has "PerformDelegatedAction" permission for the delegating principal and the requested
   action.

See [Authorization](#11-authorization) for more details on how policies are defined and evaluated.

Both "Web UI" and "Auth Provider" tokens are only usable in delegated contexts. They cannot be used to invoke the api
directly.

## 1.6 Authentication Headers

The following HTTP headers are used for authentication:

Authentication: <type> <token>
X-Event-Horizon-Delegating-Authorization: <type> <token>
X-Event-Horizon-Signed-Headers: <signed headers>


| Header                                 | Description                                                                                                                                                                    |
|----------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Authentication                         | The authorization header for the request. See [Authorization Types](#17-authorization-types) for the list of valid <type values.                                               |
| X-Event-Horizon-Delegating-Authorization | The authorization header for the delegating principal. This is only used when the request is delegated. It is optional, but if provided, must be a valid authorization header. |
| X-Event-Horizon-Signed-Headers           | The signed headers for the request, when authenticating with Sigv4. This is only used when the request is signed using Sigv4. It is optional, but if provided, must be valid.  |

## 1.7 Authorization Types

| Value                 | Description                                                                                                                                                                                     |
|-----------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| WebUIToken            | Uses for Web UI tokens. The token should be the base64 encoding of the Web UI Token json.                                                                                                       |
| AuthProviderToken     | Used for Auth Provider tokens, such as Google Identity Tokens. The token should be the base64 encoding of the Auth Provider Token json.                                                         |
| ServiceAccountToken   | Used for Service Account tokens. The token should be the base64 encoding of the Service Account Token json.                                                                                     |
| sts:GetCallerIdentity | Used for Sigv4 authentication. The token should be the base64 encoding of a a valid signed http request to sts:GetCallerIdentity. See https://github.com/debugging-sucks/sigv4util for details. |

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
  "TenantId": "string",
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
}
```

| Field          | Type                         | Description                                                                                                 |
|----------------|------------------------------|-------------------------------------------------------------------------------------------------------------|
| TenantId       | string                       | The ID of the tenant that was created. This is a v4 UUID.                                                   |
| Type           | [TenantType](#33-tenanttype) | The type of tenant that was created. Valid values are "user", "organization", and "enterprise".             |
| Version        | int                          | The version of the tenant object. Will be 1 on create. This is incremented each time the tenant is updated. |
| Deleted        | boolean                      | Whether the tenant is deleted. This is false on create.                                                     |
| CreatedAt      | string                       | The timestamp when the tenant was created, in ISO 8601 format.                                              |
| UpdatedAt      | string                       | The timestamp when the tenant was last updated, in ISO 8601 format.                                         |
| FullName       | *string                      | For user tenants: the user's full name.                                                                     |
| OrgName        | *string                      | For organization tenants: the organization name.                                                            |
| EnterpriseName | *string                      | For enterprise tenants: the enterprise name.                                                                |
| Email          | *string                      | For user tenants: the user's email address.                                                                 |
| FirstName      | *string                      | For user tenants: the user's first name.                                                                    |
| LastName       | *string                      | For user tenants: the user's last name.                                                                     |
| PictureURL     | *string                      | The URL of the picture for the tenant. Optional.                                                            |

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
  "TenantId": "string",
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
| [CreateTenant](#32-response)        | For details on response fields. |
| [Error Handling](#2-error-handling) | For details on error responses. |

# 5. ListTenantPrincipals

# 6. GetTenantPrincipal

# 7. DeleteTenantPrincipal

# 8. UpdateTenantBinding

# 9. GetTenantForPrincipal

# 10. GenerateWebUIToken

# 11. Authorization

The API implements authorization using a policy-based model. For the MVP, the set of policies used is fixed, and
we do not define any policy management apis. This will be changed in the future as we add support for service accounts.
We have 2 sets of default policies:

1. Global policies that apply to all tenants. See [Default Global Policies](#12-default-global-policies) for details.
2. Default tenant policies that are created when a new tenant is created. The specific policies created depend on the
   type of tenant.

   - For user tenants, see [Default User Tenant Policies](#13-default-user-tenant-policies).
   - For organization tenants, see [Default Organization Tenant Policies](#14-default-organization-tenant-policies).
   - For enterprise tenants, see [Default Enterprise Tenant Policies](#15-default-enterprise-tenant-policies).

## 11.1 Policy Schema

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
  "UpdatedAt": "string",
}
```

| Field              | Type                               | Description                                                                                                                                                                                                                                                                |
|--------------------|------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| PolicyID           | string                             | The unique identifier for the policy. This is a v4 UUID.                                                                                                                                                                                                                   |
| Name               | string                             | The name of the policy. This must be unique within the tenant.                                                                                                                                                                                                             |
| Effect             | [EffectType](#112-effecttype)      | The effect of the policy. This can be "Allow" or "Deny".                                                                                                                                                                                                                   |
| Tenant             | *string                            | The TennantID that the policy applies to. If this is null, the policy applies to contexts that do not specify a tenant (such as CreateTenant). If this is "*", the policy applies to all tenants. If this is a specific tenant ID, the policy applies only to that tenant. |
| Principal          | [Principal](#113-policyprincipal)  | The principal that the policy applies to.                                                                                                                                                                                                                                  |
| Actions            | [][Action](#115-action)            | The actions the policy allows or denies.                                                                                                                                                                                                                                   |
| DelegatedActions   | [][Action](#115-action)            | Only valid when action is `PerformDelegatedAction`. It identifies the section of actions that can be delegated.                                                                                                                                                            |
| DelegatedPrincipal | [*Principal](#113-policyprincipal) | Only valid when action is `PerformDelegatedAction`. It identifies the principal for which delegation is enabled                                                                                                                                                            |
| Constraints        | [][Expression](#116-expressions)   | A list of constraints expressions that must be satisfied for the policy to apply. They are dynamic and are evaluated at policy evaluation time.                                                                                                                            |
| CreatedAt          | string                             | The timestamp when the policy was created, in ISO 8601 format.                                                                                                                                                                                                             |
| UpdatedAt          | string                             | The timestamp when the policy was last updated, in ISO 8601 format.                                                                                                                                                                                                        |

## 11.2 EffectType

EffectType is an enum that defines whether a policy allows or denies access to a resource.

| Value |
|-------|
| Allow |
| Deny  |

## 11.3 PolicyPrincipal

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

| Field            | Type                                | Description                                                                                                                                                                                                                                                               |
|------------------|-------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Type             | [PrincipalType](#114-principaltype) | The type of principal.                                                                                                                                                                                                                                                    |
| Name             | *string                             | The name of the principal. Only used for `Service` and `ServiceAccount` principals.                                                                                                                                                                                       |
| RoleArn          | *string                             | The ARN of the IAM role. Only used for `IAMRole` principals.                                                                                                                                                                                                              |
| Tenant           | *string                             | The TenantID of the principal. Only used for `User` and `ServiceAccount` principals. May also be an [Expression](#116-expressions or the value `*`.                                                                                                                       |
| TokenTypes       | [][TokenType](#117-tokentype)       | When specified, restricts the policy to only apply to principals that authenticated using one of the specified token types.                                                                                                                                               |
| Provider         | *string                             | The name of the authentication provider for the principal. Only valid for `AuthProviderToken` token types. Currently only "Google" is supported.                                                                                                                          |
| Organization     | *string                             | The TenantID of the organization that the principal is a member of. When set restricts the policy to only apply to principals that are members of the provided org. Only valid for `User` and `ServiceAccount` principals. May also be an [Expression](#116-expressions). |
| OrganizationRole | [*MemberRole](#118-memberrole)      | The role of the principal in the organization. Only valid for `User` principals. Valid values are "Owner" and "Member".                                                                                                                                                   |
| Enterprise       | *string                             | The TenantID of the enterprise that the principal is a member of. When set restricts the policy to only apply to principals that are members of the provided enterprise. Only valid for `User` and `ServiceAccount` principals.                                           |
| EnterpriseRole   | [*MemberRole](#118-memberrole)      | The role of the principal in the enterprise. Only valid for `User` principals. Valid values are "Owner" and "Member".                                                                                                                                                     |

## 11.4 PrincipalType

PrincipalType is an enum that defines the type of principal that a policy applies to.

| Value          | Description                                                                                                                                                                                                                                            |
|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| User           | A human user.                                                                                                                                                                                                                                          |
| IAMRole        | An AWS IAM Role authenticating via Sigv4.                                                                                                                                                                                                              |
| Service        | An named alias for an IAM Role. This is used to enable policies to refer to event horizon services without exposing our role arns to customers (which would make it impossible to ever change them). Valid Services names are 'WebUI' and 'AdminRole'. |
| ServiceAccount | A service account.                                                                                                                                                                                                                                     |

## 11.5 Action

Action is an enum that defines the actions that a policy can allow or deny.

| Value                  |
|------------------------|
| PerformDelegatedAction |
| CreateTenant           |
| GetTenant              |
| GenerateWebUIToken     |
| ListTenants            |

## 11.6 Expressions

We support evaluating expressions in policies. Eventually we should define a full expression grammar here. For MVP we
only need to support the following expressions:

| Expression           | Description                                                                                 |
|----------------------|---------------------------------------------------------------------------------------------|
| $request.<FieldName> | A field from the request object for an api call.                                            |
| $policy.<FieldName>  | A field from the policy object being evaluated.                                             |
| 'StringLiteral'      | A string literal.                                                                           |
| expr == expr         | An expression that evaluates to true if the left-hand side is equal to the right-hand side. |

## 11.7 TokenType

TokenType is an enum that defines the type of token that a principal used to authenticate.

| Value               | Description                                                                                          |
|---------------------|------------------------------------------------------------------------------------------------------|
| WebUIToken          | A token issued by the web ui.                                                                        |
| AuthProviderToken   | A token issued by an external identity provider, such as Google Identity Tokens.                     |
| ServiceAccountToken | A token issued by a service account. This is used for automation scripts that interact with the API. |

## 11.8 MemberRole

MemberRole is an enum that defines the role of a user in an organization or enterprise.

| Value  | 
|--------|
| Owner  |
| Member |

## 11.9 DSQL Schema

```postgresql

```

# 12. Default Global Policies

The policies below are defined globally (on either the null tenant or the "*" tenant) and apply to all tenants.

## 12.1 Enable Account Creation From UI

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
  "Constraints" : ["$request.TenantType == 'User'"] 
}
```

## 12.2 Enable Account Creation Via the Admin Role

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
  "Constraints" : ["$request.TenantType == 'User'"] 
}
```

## 12.3 Enable Admin Access

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

# 13. Default User Tenant Policies

# 13.1 EnableWebUIDelegation

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

## 13.2 EnableAdminDelegation

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

## 13.3 GenerateWebUIToken

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

## 13.4 UserAccess

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

# 14. Default Organization Tenant Policies

This section defines the default policies that are created when an organization tenant is created.

## 14.1 EnableWebUIDelegation

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

## 14.2 OwnerAccess

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

## 14.3 MemberAccess

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

# 15. Default Enterprise Tenant Policies

This section defines the default policies that are created when an enterprise tenant is created.

## 15.1 EnableWebUIDelegation

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

## 15.2 OwnerAccess

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

## 15.3 MemberAccess

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
  ],
}
```

# 16. ListPolicies

The ListPolicies API is used to list all policies for a tenant. 

# 16.1 Request

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
| maxResults                               | query    | *int    | The maximum number of policies to return. Optional. Default is 500. Must be >=1 and <= 500.                                                                                                    |
| token                                    | query    | *string | A token to retrieve the next page of results. Optional. If not provided, the first page of results is returned.                                                                                |
| Authorization                            | header   | string  | The authorization header for the request.                                                                                                                                                      |
| X-Event-Horizon-Delegating-Authorization | header   | *string | The authorization header for the delegating principal.                                                                                                                                         |
| X-Event-Horizon-Signed-Headers           | header   | *string | The signed headers for the request, when authenticating with Sigv4.                                                                                                                            |

# 16.2 Response

On success a 200 OK is returned with the following JSON body:

```http request
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "Policies": [],
  "NextToken": "*string"
}
```

| Field     | Type                    | Description                                                                                    |
|-----------|-------------------------|------------------------------------------------------------------------------------------------|
| Policies  | [][Policy](#111-policy) | A list of policies for the tenant. See [Policy](#111-policy) for details on the policy object. |
| NextToken | *string                 | A token to retrieve the next page of results. If there are no more results, this will be null. |