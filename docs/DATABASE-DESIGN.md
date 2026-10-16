# Database Design — AI Gateway

| Metadata | Detail |
| :--- | :--- |
| **Version** | 1.0 |
| **Status** | Draft |
| **Date** | 18 August 2026 |
| **Database** | PostgreSQL 15 |
| **Migrations** | golang-migrate |

---

## 1. ER Diagram

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│                              AI Gateway Database                            │
└─────────────────────────────────────────────────────────────────────────────┘

    ┌──────────────────────────────────────────────────────────────────────┐
    │                        Better Auth Tables                            │
    ├──────────────────────────────────────────────────────────────────────┤
    │                                                                      │
    │  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐           │
    │  │    user       │    │   session    │    │   account    │           │
    │  ├──────────────┤    ├──────────────┤    ├──────────────┤           │
    │  │ id            │←───│ user_id      │    │ user_id      │           │
    │  │ name          │    │ id           │    │ id           │           │
    │  │ email         │    │ token        │    │ accountId    │           │
    │  │ emailVerified │    │ expiresAt    │    │ providerId   │           │
    │  │ image         │    │ ipAddress    │    └──────────────┘           │
    │  │ createdAt     │    │ userAgent    │                              │
    │  │ updatedAt     │    │ createdAt    │    ┌──────────────┐           │
    │  └──────┬───────┘    │ updatedAt    │    │ verification │           │
    │         │            └──────────────┘    ├──────────────┤           │
    │         │                                │ id           │           │
    │         │                                │ identifier   │           │
    │         │                                │ value        │           │
    │         │                                │ expiresAt    │           │
    │         │                                └──────────────┘           │
    └─────────┼────────────────────────────────────────────────────────────┘
              │
    ┌─────────┼────────────────────────────────────────────────────────────┐
    │         │                                                            │
    │  ┌──────▼───────┐    ┌──────────────────┐    ┌──────────────────┐   │
    │  │gateway_api_  │    │    providers      │    │  routing_rules   │   │
    │  │    keys      │    ├──────────────────┤    ├──────────────────┤   │
    │  ├──────────────┤    │ id               │    │ id               │   │
    │  │ id           │    │ user_id          │    │ user_id          │   │
    │  │ user_id      │    │ name             │    │ model_pattern    │   │
    │  │ name         │    │ slug             │    │ provider_id      │   │
    │  │ key_hash     │    │ base_url         │    │ priority         │   │
    │  │ key_prefix   │    │ type             │    │ enabled          │   │
    │  │ enabled      │    │ enabled          │    │ created_at       │   │
    │  │ rate_limit   │    │ created_at       │    │ updated_at       │   │
    │  │ allowed_     │    │ updated_at       │    └──────────────────┘   │
    │  │   models     │    └────────┬─────────┘                           │
    │  │ expires_at   │             │                                     │
    │  │ last_used_at │             │                                     │
    │  │ request_     │             │                                     │
    │  │   count      │             │                                     │
    │  │ created_at   │             │                                     │
    │  │ updated_at   │             │                                     │
    │  └──────────────┘             │                                     │
    │                               │                                     │
    │                 ┌─────────────┴─────────────┐                       │
    │                 │                           │                       │
    │                 ↓                           ↓                       │
    │       ┌──────────────────┐       ┌──────────────────┐              │
    │       │   credentials    │       │     models        │              │
    │       ├──────────────────┤       ├──────────────────┤              │
    │       │ id               │       │ id               │              │
    │       │ provider_id      │       │ provider_id      │              │
    │       │ name             │       │ name             │              │
    │       │ encrypted_key    │       │ slug             │              │
    │       │ key_prefix       │       │ display_name     │              │
    │       │ priority         │       │ enabled          │              │
    │       │ enabled          │       │ created_at       │              │
    │       │ status           │       │ updated_at       │              │
    │       │ last_used_at     │       └──────────────────┘              │
    │       │ request_count    │                                         │
    │       │ error_count      │                                         │
    │       │ last_error       │                                         │
    │       │ last_error_at    │                                         │
    │       │ created_at       │                                         │
    │       │ updated_at       │                                         │
    │       └──────────────────┘                                         │
    │                                                                    │
    │       ┌──────────────────────────────────────┐                     │
    │       │          request_logs                 │                     │
    │       ├──────────────────────────────────────┤                     │
    │       │ id                                   │                     │
    │       │ gateway_api_key_id                   │                     │
    │       │ provider_id                          │                     │
    │       │ credential_id                        │                     │
    │       │ model                                │                     │
    │       │ status_code                          │                     │
    │       │ latency_ms                           │                     │
    │       │ input_tokens                         │                     │
    │       │ output_tokens                        │                     │
    │       │ total_tokens                         │                     │
    │       │ error_message                        │                     │
    │       │ retry_count                          │                     │
    │       │ created_at                           │                     │
    │       └──────────────────────────────────────┘                     │
    │                                                                    │
    └────────────────────────────────────────────────────────────────────┘
```

---

## 2. Table Definitions

### 2.1 `user` (Better Auth)

User accounts managed by Better Auth. Auto-created by the library.

| Column | Type | Constraints | Description |
| :--- | :--- | :--- | :--- |
| `id` | `TEXT` | `PRIMARY KEY` | Unique identifier (Better Auth format) |
| `name` | `TEXT` | `NOT NULL` | Display name |
| `email` | `TEXT` | `UNIQUE NOT NULL` | User email |
| `emailVerified` | `BOOLEAN` | `DEFAULT false` | Email verification status |
| `image` | `TEXT` | | Profile image URL |
| `createdAt` | `TIMESTAMPTZ` | `DEFAULT NOW()` | Creation timestamp |
| `updatedAt` | `TIMESTAMPTZ` | `DEFAULT NOW()` | Last update timestamp |

### 2.2 `session` (Better Auth)

User sessions managed by Better Auth. Auto-created by the library.

| Column | Type | Constraints | Description |
| :--- | :--- | :--- | :--- |
| `id` | `TEXT` | `PRIMARY KEY` | Session identifier |
| `expiresAt` | `TIMESTAMPTZ` | `NOT NULL` | Session expiration |
| `token` | `TEXT` | `UNIQUE NOT NULL` | Session token |
| `ipAddress` | `TEXT` | | IP address |
| `userAgent` | `TEXT` | | User agent string |
| `userId` | `TEXT` | `NOT NULL`, `REFERENCES user(id) ON DELETE CASCADE` | User reference |
| `createdAt` | `TIMESTAMPTZ` | `DEFAULT NOW()` | Creation timestamp |
| `updatedAt` | `TIMESTAMPTZ` | `DEFAULT NOW()` | Last update timestamp |

### 2.3 `account` (Better Auth)

Authentication provider accounts. Auto-created by the library.

| Column | Type | Constraints | Description |
| :--- | :--- | :--- | :--- |
| `id` | `TEXT` | `PRIMARY KEY` | Account identifier |
| `accountId` | `TEXT` | `NOT NULL` | Provider account ID |
| `providerId` | `TEXT` | `NOT NULL` | Provider ID (e.g., "credential") |
| `userId` | `TEXT` | `NOT NULL`, `REFERENCES user(id) ON DELETE CASCADE` | User reference |
| `password` | `TEXT` | | Bcrypt password hash (for email/password) |
| `createdAt` | `TIMESTAMPTZ` | `DEFAULT NOW()` | Creation timestamp |
| `updatedAt` | `TIMESTAMPTZ` | `DEFAULT NOW()` | Last update timestamp |

### 2.4 `verification` (Better Auth)

Email verification and password reset tokens. Auto-created by the library.

| Column | Type | Constraints | Description |
| :--- | :--- | :--- | :--- |
| `id` | `TEXT` | `PRIMARY KEY` | Verification identifier |
| `identifier` | `TEXT` | `NOT NULL` | Email or other identifier |
| `value` | `TEXT` | `NOT NULL` | Verification token |
| `expiresAt` | `TIMESTAMPTZ` | `NOT NULL` | Token expiration |
| `createdAt` | `TIMESTAMPTZ` | `DEFAULT NOW()` | Creation timestamp |
| `updatedAt` | `TIMESTAMPTZ` | `DEFAULT NOW()` | Last update timestamp |

### 2.5 `providers`

AI provider configurations (OpenAI, Anthropic, Google).

| Column | Type | Constraints | Description |
| :--- | :--- | :--- | :--- |
| `id` | `UUID` | `PRIMARY KEY`, `DEFAULT gen_random_uuid()` | Unique identifier |
| `user_id` | `UUID` | `NOT NULL`, `REFERENCES users(id) ON DELETE CASCADE` | Owner |
| `name` | `VARCHAR(255)` | `NOT NULL` | Display name (e.g., "Anthropic Production") |
| `slug` | `VARCHAR(100)` | `UNIQUE NOT NULL` | URL-friendly identifier (e.g., "anthropic") |
| `base_url` | `VARCHAR(500)` | `NOT NULL` | Provider API base URL |
| `type` | `VARCHAR(50)` | `NOT NULL` | Provider type: `openai`, `anthropic`, `google`, `openrouter` |
| `enabled` | `BOOLEAN` | `DEFAULT true` | Whether provider is active |
| `created_at` | `TIMESTAMPTZ` | `DEFAULT NOW()` | Creation timestamp |
| `updated_at` | `TIMESTAMPTZ` | `DEFAULT NOW()` | Last update timestamp |

### 2.3 `credentials`

API keys for providers (encrypted at rest).

| Column | Type | Constraints | Description |
| :--- | :--- | :--- | :--- |
| `id` | `UUID` | `PRIMARY KEY`, `DEFAULT gen_random_uuid()` | Unique identifier |
| `provider_id` | `UUID` | `NOT NULL`, `REFERENCES providers(id) ON DELETE CASCADE` | Parent provider |
| `name` | `VARCHAR(255)` | `NOT NULL` | Display name (e.g., "Production #1") |
| `encrypted_key` | `TEXT` | `NOT NULL` | AES-256-GCM encrypted API key |
| `key_prefix` | `VARCHAR(20)` | `NOT NULL` | First/last chars for display (e.g., "sk-ant-...1234") |
| `priority` | `INTEGER` | `DEFAULT 1` | Rotation priority (lower = higher priority) |
| `enabled` | `BOOLEAN` | `DEFAULT true` | Whether credential is active |
| `status` | `VARCHAR(20)` | `DEFAULT 'active'` | Current status: `active`, `rate_limited`, `invalid`, `disabled` |
| `last_used_at` | `TIMESTAMPTZ` | | Last time credential was used |
| `request_count` | `BIGINT` | `DEFAULT 0` | Total requests using this credential |
| `error_count` | `BIGINT` | `DEFAULT 0` | Total errors for this credential |
| `last_error` | `TEXT` | | Last error message |
| `last_error_at` | `TIMESTAMPTZ` | | When last error occurred |
| `created_at` | `TIMESTAMPTZ` | `DEFAULT NOW()` | Creation timestamp |
| `updated_at` | `TIMESTAMPTZ` | `DEFAULT NOW()` | Last update timestamp |

### 2.4 `models`

Available AI models mapped to providers.

| Column | Type | Constraints | Description |
| :--- | :--- | :--- | :--- |
| `id` | `UUID` | `PRIMARY KEY`, `DEFAULT gen_random_uuid()` | Unique identifier |
| `provider_id` | `UUID` | `NOT NULL`, `REFERENCES providers(id) ON DELETE CASCADE` | Parent provider |
| `name` | `VARCHAR(255)` | `NOT NULL` | Model name (e.g., "claude-sonnet-4-20250514") |
| `slug` | `VARCHAR(255)` | `UNIQUE NOT NULL` | URL-friendly identifier |
| `display_name` | `VARCHAR(255)` | `NOT NULL` | Human-readable name (e.g., "Claude Sonnet 4") |
| `enabled` | `BOOLEAN` | `DEFAULT true` | Whether model is available |
| `created_at` | `TIMESTAMPTZ` | `DEFAULT NOW()` | Creation timestamp |
| `updated_at` | `TIMESTAMPTZ` | `DEFAULT NOW()` | Last update timestamp |

### 2.5 `gateway_api_keys`

API keys for client authentication.

| Column | Type | Constraints | Description |
| :--- | :--- | :--- | :--- |
| `id` | `UUID` | `PRIMARY KEY`, `DEFAULT gen_random_uuid()` | Unique identifier |
| `user_id` | `UUID` | `NOT NULL`, `REFERENCES users(id) ON DELETE CASCADE` | Owner |
| `name` | `VARCHAR(255)` | `NOT NULL` | Display name (e.g., "OpenCode Key") |
| `key_hash` | `VARCHAR(255)` | `UNIQUE NOT NULL` | SHA-256 hash of the key |
| `key_prefix` | `VARCHAR(20)` | `NOT NULL` | First chars for display (e.g., "gw_sk_8sdf...") |
| `enabled` | `BOOLEAN` | `DEFAULT true` | Whether key is active |
| `rate_limit` | `INTEGER` | `DEFAULT 100` | Requests per minute limit |
| `allowed_models` | `TEXT[]` | | Array of allowed model slugs (NULL = all) |
| `expires_at` | `TIMESTAMPTZ` | | Optional expiration |
| `last_used_at` | `TIMESTAMPTZ` | | Last time key was used |
| `request_count` | `BIGINT` | `DEFAULT 0` | Total requests with this key |
| `created_at` | `TIMESTAMPTZ` | `DEFAULT NOW()` | Creation timestamp |
| `updated_at` | `TIMESTAMPTZ` | `DEFAULT NOW()` | Last update timestamp |

### 2.6 `routing_rules`

Custom routing rules for model-to-provider mapping.

| Column | Type | Constraints | Description |
| :--- | :--- | :--- | :--- |
| `id` | `UUID` | `PRIMARY KEY`, `DEFAULT gen_random_uuid()` | Unique identifier |
| `user_id` | `UUID` | `NOT NULL`, `REFERENCES users(id) ON DELETE CASCADE` | Owner |
| `model_pattern` | `VARCHAR(255)` | `NOT NULL` | Model pattern (e.g., "claude-*") |
| `provider_id` | `UUID` | `REFERENCES providers(id) ON DELETE SET NULL` | Target provider |
| `priority` | `INTEGER` | `DEFAULT 1` | Rule priority (lower = higher) |
| `enabled` | `BOOLEAN` | `DEFAULT true` | Whether rule is active |
| `created_at` | `TIMESTAMPTZ` | `DEFAULT NOW()` | Creation timestamp |
| `updated_at` | `TIMESTAMPTZ` | `DEFAULT NOW()` | Last update timestamp |

### 2.7 `request_logs`

Request logging for observability.

| Column | Type | Constraints | Description |
| :--- | :--- | :--- | :--- |
| `id` | `UUID` | `PRIMARY KEY`, `DEFAULT gen_random_uuid()` | Unique identifier |
| `gateway_api_key_id` | `UUID` | `REFERENCES gateway_api_keys(id) ON DELETE SET NULL` | API key used |
| `provider_id` | `UUID` | `REFERENCES providers(id) ON DELETE SET NULL` | Provider used |
| `credential_id` | `UUID` | `REFERENCES credentials(id) ON DELETE SET NULL` | Credential used |
| `model` | `VARCHAR(255)` | `NOT NULL` | Model name requested |
| `status_code` | `INTEGER` | `NOT NULL` | HTTP status code |
| `latency_ms` | `INTEGER` | `NOT NULL` | Response latency in milliseconds |
| `input_tokens` | `INTEGER` | `DEFAULT 0` | Input token count |
| `output_tokens` | `INTEGER` | `DEFAULT 0` | Output token count |
| `total_tokens` | `INTEGER` | `DEFAULT 0` | Total token count |
| `error_message` | `TEXT` | | Error message if failed |
| `retry_count` | `INTEGER` | `DEFAULT 0` | Number of retries |
| `created_at` | `TIMESTAMPTZ` | `DEFAULT NOW()` | Request timestamp |

---

## 3. Indexes

### Performance Indexes

```sql
-- Better Auth Tables (already created in migration 001)
-- CREATE INDEX idx_session_token ON session(token);
-- CREATE INDEX idx_session_user_id ON session("userId");
-- CREATE INDEX idx_session_expires_at ON session("expiresAt");
-- CREATE INDEX idx_account_user_id ON account("userId");
-- CREATE INDEX idx_account_provider_id ON account("providerId");
-- CREATE INDEX idx_verification_identifier ON verification(identifier);

-- Providers
CREATE INDEX idx_providers_user_id ON providers(user_id);
CREATE INDEX idx_providers_slug ON providers(slug);
CREATE INDEX idx_providers_type ON providers(type);

-- Credentials
CREATE INDEX idx_credentials_provider_id ON credentials(provider_id);
CREATE INDEX idx_credentials_status ON credentials(status);
CREATE INDEX idx_credentials_enabled ON credentials(enabled);
CREATE INDEX idx_credentials_priority ON credentials(priority);
CREATE INDEX idx_credentials_provider_status ON credentials(provider_id, status);
CREATE INDEX idx_credentials_provider_enabled ON credentials(provider_id, enabled);

-- Models
CREATE INDEX idx_models_provider_id ON models(provider_id);
CREATE INDEX idx_models_slug ON models(slug);

-- Gateway API Keys
CREATE INDEX idx_gateway_api_keys_user_id ON gateway_api_keys(user_id);
CREATE INDEX idx_gateway_api_keys_key_hash ON gateway_api_keys(key_hash);
CREATE INDEX idx_gateway_api_keys_enabled ON gateway_api_keys(enabled);

-- Routing Rules
CREATE INDEX idx_routing_rules_user_id ON routing_rules(user_id);
CREATE INDEX idx_routing_rules_model_pattern ON routing_rules(model_pattern);
CREATE INDEX idx_routing_rules_priority ON routing_rules(priority);

-- Request Logs (time-series queries)
CREATE INDEX idx_request_logs_created_at ON request_logs(created_at);
CREATE INDEX idx_request_logs_gateway_api_key_id ON request_logs(gateway_api_key_id);
CREATE INDEX idx_request_logs_provider_id ON request_logs(provider_id);
CREATE INDEX idx_request_logs_credential_id ON request_logs(credential_id);
CREATE INDEX idx_request_logs_model ON request_logs(model);
CREATE INDEX idx_request_logs_status_code ON request_logs(status_code);

-- Composite indexes for common queries
CREATE INDEX idx_request_logs_date_provider ON request_logs(created_at, provider_id);
CREATE INDEX idx_request_logs_date_model ON request_logs(created_at, model);
```

---

## 4. Migration Files

### Migration 001: Create Better Auth Tables

```sql
-- 001_create_better_auth_tables.up.sql

-- User table (Better Auth schema)
CREATE TABLE "user" (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT UNIQUE NOT NULL,
    "emailVerified" BOOLEAN DEFAULT false,
    image TEXT,
    "createdAt" TIMESTAMPTZ DEFAULT NOW(),
    "updatedAt" TIMESTAMPTZ DEFAULT NOW()
);

-- Session table (Better Auth schema)
CREATE TABLE session (
    id TEXT PRIMARY KEY,
    "expiresAt" TIMESTAMPTZ NOT NULL,
    token TEXT UNIQUE NOT NULL,
    "ipAddress" TEXT,
    "userAgent" TEXT,
    "userId" TEXT NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    "createdAt" TIMESTAMPTZ DEFAULT NOW(),
    "updatedAt" TIMESTAMPTZ DEFAULT NOW()
);

-- Account table (Better Auth schema)
CREATE TABLE account (
    id TEXT PRIMARY KEY,
    "accountId" TEXT NOT NULL,
    "providerId" TEXT NOT NULL,
    "userId" TEXT NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    password TEXT,
    "createdAt" TIMESTAMPTZ DEFAULT NOW(),
    "updatedAt" TIMESTAMPTZ DEFAULT NOW()
);

-- Verification table (Better Auth schema)
CREATE TABLE verification (
    id TEXT PRIMARY KEY,
    identifier TEXT NOT NULL,
    value TEXT NOT NULL,
    "expiresAt" TIMESTAMPTZ NOT NULL,
    "createdAt" TIMESTAMPTZ DEFAULT NOW(),
    "updatedAt" TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes for Better Auth tables
CREATE INDEX idx_session_token ON session(token);
CREATE INDEX idx_session_user_id ON session("userId");
CREATE INDEX idx_session_expires_at ON session("expiresAt");
CREATE INDEX idx_account_user_id ON account("userId");
CREATE INDEX idx_account_provider_id ON account("providerId");
CREATE INDEX idx_verification_identifier ON verification(identifier);
```

```sql
-- 001_create_better_auth_tables.down.sql
DROP TABLE IF EXISTS verification;
DROP TABLE IF EXISTS account;
DROP TABLE IF EXISTS session;
DROP TABLE IF EXISTS "user";
```

### Migration 002: Create Providers Table

```sql
-- 002_create_providers.up.sql
CREATE TABLE providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    base_url VARCHAR(500) NOT NULL,
    type VARCHAR(50) NOT NULL CHECK (type IN ('openai', 'anthropic', 'google', 'openrouter')),
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_providers_user_id ON providers(user_id);
CREATE INDEX idx_providers_slug ON providers(slug);
CREATE INDEX idx_providers_type ON providers(type);
```

```sql
-- 002_create_providers.down.sql
DROP TABLE IF EXISTS providers;
```

### Migration 003: Create Credentials Table

```sql
-- 003_create_credentials.up.sql
CREATE TABLE credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    encrypted_key TEXT NOT NULL,
    key_prefix VARCHAR(20) NOT NULL,
    priority INTEGER DEFAULT 1,
    enabled BOOLEAN DEFAULT true,
    status VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'rate_limited', 'invalid', 'disabled')),
    last_used_at TIMESTAMPTZ,
    request_count BIGINT DEFAULT 0,
    error_count BIGINT DEFAULT 0,
    last_error TEXT,
    last_error_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_credentials_provider_id ON credentials(provider_id);
CREATE INDEX idx_credentials_status ON credentials(status);
CREATE INDEX idx_credentials_enabled ON credentials(enabled);
CREATE INDEX idx_credentials_priority ON credentials(priority);
CREATE INDEX idx_credentials_provider_status ON credentials(provider_id, status);
CREATE INDEX idx_credentials_provider_enabled ON credentials(provider_id, enabled);
```

```sql
-- 003_create_credentials.down.sql
DROP TABLE IF EXISTS credentials;
```

### Migration 004: Create Models Table

```sql
-- 004_create_models.up.sql
CREATE TABLE models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_models_provider_id ON models(provider_id);
CREATE INDEX idx_models_slug ON models(slug);
```

```sql
-- 004_create_models.down.sql
DROP TABLE IF EXISTS models;
```

### Migration 005: Create Gateway API Keys Table

```sql
-- 005_create_gateway_api_keys.up.sql
CREATE TABLE gateway_api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) UNIQUE NOT NULL,
    key_prefix VARCHAR(20) NOT NULL,
    enabled BOOLEAN DEFAULT true,
    rate_limit INTEGER DEFAULT 100,
    allowed_models TEXT[],
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    request_count BIGINT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_gateway_api_keys_user_id ON gateway_api_keys(user_id);
CREATE INDEX idx_gateway_api_keys_key_hash ON gateway_api_keys(key_hash);
CREATE INDEX idx_gateway_api_keys_enabled ON gateway_api_keys(enabled);
```

```sql
-- 005_create_gateway_api_keys.down.sql
DROP TABLE IF EXISTS gateway_api_keys;
```

### Migration 006: Create Routing Rules Table

```sql
-- 006_create_routing_rules.up.sql
CREATE TABLE routing_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    model_pattern VARCHAR(255) NOT NULL,
    provider_id UUID REFERENCES providers(id) ON DELETE SET NULL,
    priority INTEGER DEFAULT 1,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_routing_rules_user_id ON routing_rules(user_id);
CREATE INDEX idx_routing_rules_model_pattern ON routing_rules(model_pattern);
CREATE INDEX idx_routing_rules_priority ON routing_rules(priority);
```

```sql
-- 006_create_routing_rules.down.sql
DROP TABLE IF EXISTS routing_rules;
```

### Migration 007: Create Request Logs Table

```sql
-- 007_create_request_logs.up.sql
CREATE TABLE request_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    gateway_api_key_id UUID REFERENCES gateway_api_keys(id) ON DELETE SET NULL,
    provider_id UUID REFERENCES providers(id) ON DELETE SET NULL,
    credential_id UUID REFERENCES credentials(id) ON DELETE SET NULL,
    model VARCHAR(255) NOT NULL,
    status_code INTEGER NOT NULL,
    latency_ms INTEGER NOT NULL,
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    total_tokens INTEGER DEFAULT 0,
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Time-series indexes
CREATE INDEX idx_request_logs_created_at ON request_logs(created_at);
CREATE INDEX idx_request_logs_gateway_api_key_id ON request_logs(gateway_api_key_id);
CREATE INDEX idx_request_logs_provider_id ON request_logs(provider_id);
CREATE INDEX idx_request_logs_credential_id ON request_logs(credential_id);
CREATE INDEX idx_request_logs_model ON request_logs(model);
CREATE INDEX idx_request_logs_status_code ON request_logs(status_code);

-- Composite indexes for common dashboard queries
CREATE INDEX idx_request_logs_date_provider ON request_logs(created_at, provider_id);
CREATE INDEX idx_request_logs_date_model ON request_logs(created_at, model);
```

```sql
-- 007_create_request_logs.down.sql
DROP TABLE IF EXISTS request_logs;
```

---

## 5. Encryption Approach

### Credential Encryption (AES-256-GCM)

```go
// Pseudocode for encryption/decryption

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "io"
)

// Encrypt encrypts plaintext using AES-256-GCM
func Encrypt(plaintext string, key []byte) (string, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return "", err
    }

    aesGCM, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }

    nonce := make([]byte, aesGCM.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", err
    }

    ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts AES-256-GCM encrypted text
func Decrypt(encrypted string, key []byte) (string, error) {
    ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
    if err != nil {
        return "", err
    }

    block, err := aes.NewCipher(key)
    if err != nil {
        return "", err
    }

    aesGCM, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }

    nonceSize := aesGCM.NonceSize()
    if len(ciphertext) < nonceSize {
        return "", errors.New("ciphertext too short")
    }

    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return "", err
    }

    return string(plaintext), nil
}
```

### Gateway API Key Hashing (SHA-256)

```go
import (
    "crypto/sha256"
    "encoding/hex"
)

// HashAPIKey creates a SHA-256 hash of the API key
func HashAPIKey(key string) string {
    hash := sha256.Sum256([]byte(key))
    return hex.EncodeToString(hash[:])
}

// GenerateAPIKey creates a new random API key
func GenerateAPIKey() (string, string, string) {
    // Generate random bytes
    b := make([]byte, 32)
    rand.Read(b)

    // Format: gw_sk_<base64>
    key := "gw_sk_" + base64.RawURLEncoding.EncodeToString(b)
    prefix := key[:12] + "..."
    hash := HashAPIKey(key)

    return key, prefix, hash
}
```

---

## 6. Seed Data

### Development Seed

```sql
-- 008_seed_dev_data.up.sql

-- Default admin user (Better Auth format)
-- Password will be set via Better Auth on first login
INSERT INTO "user" (id, name, email, "emailVerified", created_at, updated_at) VALUES
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'Admin', 'admin@aigateway.dev', true, NOW(), NOW());

-- Default providers
INSERT INTO providers (id, user_id, name, slug, base_url, type) VALUES
    ('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'OpenAI', 'openai', 'https://api.openai.com', 'openai'),
    ('c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'Anthropic', 'anthropic', 'https://api.anthropic.com', 'anthropic'),
    ('d0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'Google', 'google', 'https://generativelanguage.googleapis.com', 'google');

-- Default models
INSERT INTO models (provider_id, name, slug, display_name) VALUES
    ('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'gpt-4o', 'gpt-4o', 'GPT-4o'),
    ('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'gpt-4o-mini', 'gpt-4o-mini', 'GPT-4o Mini'),
    ('c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'claude-sonnet-4-20250514', 'claude-sonnet', 'Claude Sonnet 4'),
    ('c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'claude-3-5-haiku-20241022', 'claude-haiku', 'Claude 3.5 Haiku'),
    ('d0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'gemini-1.5-pro', 'gemini-pro', 'Gemini 1.5 Pro'),
    ('d0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'gemini-1.5-flash', 'gemini-flash', 'Gemini 1.5 Flash');
```

```sql
-- 008_seed_dev_data.down.sql
DELETE FROM models;
DELETE FROM providers;
DELETE FROM "user" WHERE email = 'admin@aigateway.dev';
```

### Better Auth Account Seed

```sql
-- 009_seed_admin_account.up.sql

-- Create account for admin user (email/password)
-- Password hash is generated by Better Auth (bcrypt)
INSERT INTO account (id, "accountId", "providerId", "userId", password, created_at, updated_at) VALUES
    ('e0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'admin@aigateway.dev', 'credential', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', '$2a$10$YourBcryptHashHere', NOW(), NOW());
```

```sql
-- 009_seed_admin_account.down.sql
DELETE FROM account WHERE "userId" = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
```

---

## 7. Database Configuration

### PostgreSQL Settings

```sql
-- Recommended PostgreSQL configuration for AI Gateway

-- Connection pooling
max_connections = 100

-- Memory
shared_buffers = 1GB          -- 25% of 4GB VPS
effective_cache_size = 3GB    -- 75% of 4GB VPS
work_mem = 16MB
maintenance_work_mem = 256MB

-- Write performance
wal_buffers = 16MB
checkpoint_completion_target = 0.9
max_wal_size = 1GB

-- Query performance
random_page_cost = 1.1        -- SSD storage
effective_io_concurrency = 200

-- Logging
log_min_duration_statement = 1000  -- Log queries > 1s
log_checkpoints = on
log_connections = on
log_disconnections = on
```

---

## 8. Table Size Estimates (V1)

| Table | Estimated Rows (1 year) | Size |
| :--- | :--- | :--- |
| `user` | 10 | ~1 MB |
| `session` | 100 | ~1 MB |
| `account` | 10 | ~1 MB |
| `verification` | 100 | ~1 MB |
| `providers` | 500 | ~1 MB |
| `credentials` | 2,000 | ~5 MB |
| `models` | 5,000 | ~5 MB |
| `gateway_api_keys` | 1,000 | ~2 MB |
| `routing_rules` | 500 | ~1 MB |
| `request_logs` | 10,000,000 | ~10 GB |

**Recommendation:** Partition `request_logs` by month for better performance after 6 months.

---

## 8. Schema Conventions

`internal/database/migrate_test.go` checks these on a database migrated from empty, so a migration that breaks them fails CI.

- **Owner:** resources are owned by a user. `user_id` is `text`, like `"user".id`, and references it `ON DELETE CASCADE`. `NULL` means a system default every user can read but not change. Audit and log tables keep the raw id without a foreign key, so their history outlives the user.
- **Organization:** the column that points at `organizations(id)` is always `org_id`, never `organization_id`. It is context and a tenant boundary, not the owner.
- **Foreign keys:** every foreign key has an index that starts with its column.
- **Indexes:** no two indexes cover the same columns. `request_logs` is the most written table, so add an index there only for a query that needs it.
- **Views:** none. A view freezes its column list when it is created and drifts from the table.
- **Primary keys:** new tables use `uuid` with `gen_random_uuid()`. The existing exceptions keep their types, because changing a primary key type would touch every foreign key that points at it:
  - `text`: the Better Auth tables (`"user"`, `session`, `account`, `verification`) and `oauth_states`;
  - `varchar`: the tenancy tables (`organizations`, `workspaces`, `projects`, `organization_members`), plus `settings` and `request_payloads`;
  - `bigint`: the append-only logs `tool_invocations` and `mcp_server_invocations`.
