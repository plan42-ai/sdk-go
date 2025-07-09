CREATE SCHEMA IF NOT EXISTS event_horizon;

CREATE ROLE api_service WITH LOGIN;
AWS IAM GRANT api_service TO 'arn:aws:iam::802872447332:role/AWSReservedSSO_AdministratorAccess_8625b8d8962d2f79';
GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA event_horizon TO api_service;

CREATE TABLE IF NOT EXISTS event_horizon.Tenants (
    tenant_id UUID PRIMARY KEY,
    type TEXT NOT NULL CHECK (type IN ('User', 'Organization', 'Enterprise')),
    version INT NOT NULL DEFAULT 1,
    deleted BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS  event_horizon.Users (
    tenant_id UUID PRIMARY KEY,
    fullname TEXT NOT NULL,
    email TEXT NOT NULL,
    firstname TEXT NOT NULL,
    lastname TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS event_horizon.Organizations (
    tenant_id UUID PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    enterprise_tenant_id UUID
);

CREATE INDEX ASYNC IF NOT EXISTS idex_organizations_enterprise_tenant_id ON event_horizon.Organizations (enterprise_tenant_id, name, tenant_id);

CREATE TABLE IF NOT EXISTS event_horizon.Enterprises (
     tenant_id UUID PRIMARY KEY,
     name TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS event_horizon.GooglePrincipals (
    tenant_id UUID NOT NULL,
    principalid UUID NOT NULL UNIQUE ,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    iss TEXT NOT NULL,
    aud TEXT NOT NULL,
    sub TEXT NOT NULL,
    primary key (tenant_id, principalid),
    unique(sub, aud, iss)
);

CREATE INDEX ASYNC IF NOT EXISTS idex_google_principals_tenant_id ON event_horizon.GooglePrincipals (tenant_id, created_at, principalid);

CREATE TABLE IF NOT EXISTS event_horizon.Members (
    parent_tenant_id UUID NOT NULL,
    child_tenant_id UUID NOT NULL,
    member_role TEXT NOT NULL CHECK (member_role IN ('Owner', 'Member')),
    primary key (parent_tenant_id, child_tenant_id)
);

CREATE TABLE IF NOT EXISTS event_horizon.Policies (
    tenant_id TEXT,
    policy_id UUID NOT NULL,
    version int NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    name TEXT NOT NULL,
    effect TEXT NOT NULL CHECK (effect IN('Allow', 'Deny')),

    principal_type TEXT NOT NULL CHECK (principal_type IN('User', 'IAMRole', 'Service', 'ServiceAccount')),
    principal_name TEXT,
    principal_role_arn TEXT,
    principal_tenant_id TEXT,
    principal_token_types_bitvector bigint NOT NULL,
    principal_provider TEXT,
    principal_organization_tenant_id TEXT,
    principal_organization_role TEXT CHECK (principal_organization_role IN('Owner', 'Member')),
    principal_enterprise_tenant_id TEXT,
    principal_enterprise_role TEXT CHECK (principal_enterprise_role IN('Owner', 'Member')),

    /* DSQL doesn't support bit(n), bit varying, bit varying(n), or bigint[], so just use a bigint for our bitvector.
     * This is ok until we have more than 64 actions. Then we may need to change this, or add actions_bitvector_2, etc.
     */
    actions_bitvector bigint NOT NULL,

    delegator_type TEXT NOT NULL CHECK (delegator_type IN('User', 'IAMRole', 'Service', 'ServiceAccount')),
    delegator_name TEXT,
    delegator_role_arn TEXT,
    delegator_tenant_id TEXT,
    delegator_token_types_bitvector bigint NOT NULL,
    delegator_provider TEXT,
    delegator_organization_tenant_id TEXT,
    delegator_organization_role TEXT CHECK (delegator_organization_role IN('Owner', 'Member')),
    delegator_enterprise_tenant_id TEXT,
    delegator_enterprise_role TEXT CHECK (delegator_enterprise_role IN('Owner', 'Member')),

    /* Same thing about bit(n), bit varying, bit varying(n), or bigint[]. */
    delegated_actions_bitvector bigint NOT NULL,
    primary key (tenant_id, policy_id),
    /* enforce policy names are unique per tenant */
    UNIQUE(tenant_id, name),
    /* Create an index to use when implementing ListTenantPolicies */
    UNIQUE(tenant_id, created_at, policy_id)
);