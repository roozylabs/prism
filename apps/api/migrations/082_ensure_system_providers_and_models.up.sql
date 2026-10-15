-- Migration 082: Ensure Global System AI Providers and Base Models are Seeded
-- Guarantees system-level providers (OpenAI, Anthropic, Google, OpenCode Zen, Groq, DeepSeek) are always available across all tenants

-- 1. Ensure system providers exist with fixed system user_id
INSERT INTO providers (id, user_id, name, slug, base_url, type, enabled, routing_strategy, created_at, updated_at)
VALUES
    ('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'OpenAI', 'openai', 'https://api.openai.com', 'openai', true, 'round_robin', NOW(), NOW()),
    ('c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'Anthropic', 'anthropic', 'https://api.anthropic.com', 'anthropic', true, 'round_robin', NOW(), NOW()),
    ('d0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'Google Gemini', 'google', 'https://generativelanguage.googleapis.com', 'google', true, 'round_robin', NOW(), NOW()),
    ('e0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'OpenCode Zen', 'opencode-zen', 'https://opencode.ai/zen', 'opencode', true, 'round_robin', NOW(), NOW()),
    ('a2eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'Groq Fast Inference', 'groq', 'https://api.groq.com/openai/v1', 'groq', true, 'round_robin', NOW(), NOW()),
    ('a1eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'DeepSeek AI', 'deepseek', 'https://api.deepseek.com', 'deepseek', true, 'round_robin', NOW(), NOW())
ON CONFLICT (slug) DO UPDATE SET
    enabled = true,
    updated_at = NOW();

-- 2. Ensure base system models are bound to system providers
INSERT INTO models (provider_id, name, slug, display_name, enabled, supports_tools, supports_vision, created_at, updated_at)
VALUES
    ('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'gpt-4o', 'gpt-4o', 'GPT-4o', true, true, true, NOW(), NOW()),
    ('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'gpt-4o-mini', 'gpt-4o-mini', 'GPT-4o Mini', true, true, true, NOW(), NOW()),
    ('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'o3-mini', 'o3-mini', 'o3-mini Reasoning', true, true, false, NOW(), NOW()),
    ('c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'claude-3-7-sonnet-20250219', 'claude-3-7-sonnet', 'Claude 3.7 Sonnet', true, true, true, NOW(), NOW()),
    ('c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'claude-3-5-haiku-20241022', 'claude-3-5-haiku', 'Claude 3.5 Haiku', true, true, true, NOW(), NOW()),
    ('d0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'gemini-2.0-flash', 'gemini-2.0-flash', 'Gemini 2.0 Flash', true, true, true, NOW(), NOW()),
    ('d0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'gemini-1.5-pro', 'gemini-1.5-pro', 'Gemini 1.5 Pro', true, true, true, NOW(), NOW()),
    ('e0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'deepseek-v4-pro', 'deepseek-v4-pro', 'DeepSeek V4 Pro', true, true, false, NOW(), NOW()),
    ('e0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'kimi-k3', 'kimi-k3', 'Kimi K3', true, true, false, NOW(), NOW()),
    ('a2eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'llama-3.3-70b-versatile', 'llama-3.3-70b-versatile', 'Llama 3.3 70B (Groq)', true, true, false, NOW(), NOW()),
    ('a1eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'deepseek-chat', 'deepseek-chat', 'DeepSeek V3 (Chat)', true, true, false, NOW(), NOW()),
    ('a1eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'deepseek-reasoner', 'deepseek-reasoner', 'DeepSeek R1 (Reasoner)', true, true, false, NOW(), NOW())
ON CONFLICT (provider_id, slug) DO UPDATE SET
    enabled = true,
    updated_at = NOW();
