# Prism CLI (`prism`) Reference Manual

The `prism` command-line interface lets developers manage API keys, inspect available models, monitor agent identity policies, evaluate routing simulation, and check system health directly from the terminal.

---

## Installation & Setup

```bash
pnpm add -g @roozylabs/prism-cli
```

Verify installation:

```bash
prism --version
```

---

## Authentication (`prism login`)

Save your Gateway URL and Gateway API key locally in `~/.prism/config.json`:

```bash
prism login --key prism_sk_123456789 --url http://localhost:8080
```

---

## Commands Reference

### `prism health`
Performs a health check against the Gateway database and Redis cache.

```bash
prism health
```

Example Output:
```text
=== Prism Gateway Health Status ===
Status   : ok
Database : ok
Redis    : ok
Version  : 2.1.0
Timestamp: 2026-08-26T11:25:00Z
===================================
```

### `prism model list`
Displays a clean, formatted table of all active models and their providers.

```bash
prism model list
```

### `prism agent list / create / inspect`
Manage Agent identities and policies:

```bash
# List all registered agents
prism agent list

# Create a new agent
prism agent create --name devops-agent --description "CI/CD Deployment Helper"

# Inspect agent details
prism agent inspect <agent_id>
```

### `prism credential list / reset-cooldown`
Monitor and manage upstream provider credentials:

```bash
# List upstream credentials and health scores
prism credential list

# Reset cooldown status for a degraded credential
prism credential reset-cooldown <credential_id>
```

### `prism routing simulate`
Simulate `prism-auto` dynamic routing decisions:

```bash
prism routing simulate --model prism-auto --prompt "Optimize postgres query with index" --policy quality
```

### `prism gateway status`
Displays high-level gateway metrics and operational readiness.

```bash
prism gateway status
```

