# RoozyLabs Prism — Canonical Authorization & RBAC Model (v2.2.0)

## 1. Executive Overview

RoozyLabs Prism implements a **hierarchical, multi-tenant, least-privilege, fail-closed** authorization architecture. Every access decision in the system evaluates the principal, action, target resource, and the tenant scope boundaries:

```text
Can(principal, action, resource, organization, workspace, project, context) -> ALLOW | DENY
```

---

## 2. Structural Hierarchy & Evaluation Chain

### 2.1 Structural Hierarchy
```text
User / Identity (Authentication Root)
      │
      ├── Organization Membership (`organization_members`)
      │       └── Organization Role (`roles`, `role_permissions` via `role_id`)
      │
      └── Workspace Membership (`workspace_members`)
              └── Workspace Role (`admin`, `developer`, `operator`, `viewer`)
                      │
                      └── Projects (`projects`)
                              │
                              ├── Agents (`agents`: org_id, workspace_id, project_id)
                              ├── MCP Servers (`mcp_servers`: org_id, workspace_id)
                              ├── Tools (`tools`: org_id, workspace_id)
                              ├── Resources (`resources`: org_id, workspace_id)
                              └── Gateway Keys (`gateway_api_keys`: org_id, workspace_id, project_id)
```

### 2.2 Canonical Evaluation Chain
```text
Request
  ↓
Authentication (JWT / Gateway Key)
  ↓
Organization Membership (organization_members)
  ↓
Organization Permissions (role_permissions via role_id)
  ↓
Workspace Access Gate
  ↓
Workspace Membership (workspace_members)
  OR explicit org-level "workspace:admin" permission
  ↓
Resource Permission (<resource>:<action>)
```

> [!IMPORTANT]
> **Org Owner ≠ automatic member of every workspace.**
> Workspace membership is mandatory for workspace-scoped resources. Organization administrative access across all workspaces is explicitly modeled via the `workspace:admin` permission granted through `role_permissions`.

---

## 3. Principal Taxonomy

The authorization engine distinguishes 5 distinct principal types:

| Principal Type | Identifier Format | Description | Authorization Scope |
| :--- | :--- | :--- | :--- |
| **`human_user`** | UUID (`user.id`) | Authenticated human engineer or administrator operating via Web Dashboard / CLI. | Scoped to active `organization_id` & `workspace_id` memberships. |
| **`service_principal`** | `sp_<uuid>` | Machine-to-machine service integration for CI/CD, FinOps cron, or backend orchestrators. | Explicitly bound to organization & workspace with scoped permissions. |
| **`agent`** | UUID (`agent.id`) | Autonomous AI Agent executing downstream LLM tool calls and MCP integrations. | Inherits parent workspace scope; restricted by agent policy sandbox. |
| **`gateway_api_key`** | `prism_sk_<hash>` | Scoped Bearer API credential used for proxy inference (`/v1/chat/completions`). | Bound to specific `org_id`, `workspace_id`, `project_id`, and allowed model list. |
| **`platform_admin`** | Platform Root | Internal RoozyLabs infrastructure operators managing multi-tenant cluster health. | Platform-level only; completely separated from tenant data access. |

---

## 4. Separation of Identity vs Authoritative Membership

1. **User Table (`"user"`)**:
   - Represents identity, authentication credentials, email verification, and non-authoritative profile metadata (`name`, `email`, `avatar_url`, `primary_role`).
   - `user.primary_role` and `user.org_id` are **non-authoritative metadata** used only as UX display hints.
2. **Organization Membership (`organization_members`)**:
   - **Sole authoritative source of truth** for tenant-level access.
   - Requires explicit active record matching `(org_id, user_id)` with an assigned `role_id` foreign key.
   - Legacy `role` string column is retained as deprecated read-only for one release cycle before removal.
3. **Workspace Membership (`workspace_members`)**:
   - Explicit membership relation for fine-grained multi-workspace environments.
   - Adding a user to a workspace strictly verifies the user is an active member of the workspace's parent organization.

---

## 5. Canonical Role Model & Permissions Taxonomy

### 5.1. System Roles
- **`owner`**: Full administrative, billing, policy, member management, and resource ownership control over the organization (includes `workspace:admin`).
- **`admin`**: Team management, credential management, model routing, and gateway control (cannot delete organization or remove owners).
- **`developer`**: CRUD for API Keys, Prompts, MCP Servers, Tools, Playground, and Logs.
- **`finops_manager`**: Invoices, payment methods, plan tiers, budget limits, and FinOps analytics.
- **`auditor`**: Read-only cryptographic inspection of audit trails, compliance policies, and request logs.
- **`viewer`**: Read-only monitoring access across telemetry dashboards.

### 5.2. Canonical Permission Taxonomy (`<resource>:<action>`)

```text
# Organization Management
organization:read
organization:update
organization:delete

# Team & Role Management
member:read
member:invite
member:update
member:remove
role:read
role:create
role:update
role:delete

# Workspace & Projects
workspace:read
workspace:create
workspace:update
workspace:delete
workspace:admin
project:read
project:create
project:update
project:delete

# AI Infrastructure & Gateway
api_key:read
api_key:create
api_key:rotate
api_key:revoke

credential:read
credential:create
credential:update
credential:delete

model:read
model:create
model:update
model:delete

# Agents & MCP Tools
agent:read
agent:create
agent:update
agent:delete
agent:execute

mcp:read
mcp:create
mcp:update
mcp:delete
mcp:execute

tool:read
tool:create
tool:update
tool:delete
tool:execute

resource:read
resource:create
resource:update
resource:delete
resource:query

# Governance, Budgets & FinOps
governance:read
governance:create
governance:update
governance:delete
governance:evaluate

budget:read
budget:create
budget:update
budget:delete

quota:read
quota:update

billing:read
billing:manage

finops:read
finops:manage_budget

# Operational & Audit Trail
playground:execute
logs:read
audit:read
audit:export
audit:verify
```

---

## 6. Critical Security Invariants

1. **Fail-Closed Rule**: Any database error, missing role, missing permission, missing tenant context, or unverified checker yields an immediate `DENY` (`401 Unauthorized` or `403 Forbidden`).
2. **Atomic Last-Owner Protection**: An organization must always possess at least one active `owner`. Transactions attempting to delete, demote, or remove the final owner use row-level locking (`SELECT ... FOR UPDATE`) to prevent race conditions.
3. **Zero Tenant Bypass**: Complete elimination of special tenant shortcuts (`org_default`) and hardcoded user strings (`user_admin`). Missing tenant headers return explicit errors.
4. **Mandatory Workspace RBAC**: Accessing workspace-scoped resources requires explicit workspace membership or the explicit `workspace:admin` permission.
5. **Organizational Chain Integrity**: Adding a workspace member strictly verifies that the user belongs to the workspace's parent organization.
6. **Gateway Key Scope Narrowing**: Gateway API keys cannot exceed their provisioning scope (Organization, Workspace, Project, Allowed Models, Spend Caps).
