# 🚀 Paperclip Agent Integration Guide for RoozyLabs Prism

This guide explains how to configure **Paperclip** agents (**Roozy**, **Eleana**, **Developer Agents**) to connect seamlessly to **RoozyLabs Prism** using the **`prism-auto`** Smart Router and avoid common workspace path execution errors on Windows.

---

## 1. Paperclip Adapter Configuration

In your Paperclip Console (`http://localhost:3100 > Agents > [Agent Name] > Configuration > Adapter`):

| Setting | Value | Description |
| :--- | :--- | :--- |
| **Adapter Type** | `Prism RoozyLabs` (`prism_roozylabs`) | Standard OpenAI API protocol with Prism extension headers |
| **Gateway URL** | `https://api.prism.roozylabs.com` | Live Prism Model Gateway API Proxy |
| **Gateway API Key** | `prism_sk_...` | Your Prism Gateway API Key |
| **Model** | **`prism-auto`** | Dynamic Smart Router model |
| **Custom Headers** | `X-Prism-Agent-ID: roozy-ceo` | Agent identity tracking header |

---

## 2. Resolving Missing Disposition & Workspace Path Errors

### Problem:
An agent attempted `find /workspace/prism -maxdepth 4 ...`, resulting in:
`Paperclip could not resolve this issue's missing disposition automatically.`

### Root Cause:
The LLM model inferred a generic Linux container path (`/workspace/prism`) because the system prompt or project configuration did not explicitly declare the local workspace root path for Windows (`C:\me\projects\ai-gateway`).

### Solution:

#### Option A: Update Paperclip Project Configuration
In Paperclip (`Projects > Prism > Configuration`):
- Set **Local Folder**: `C:\me\projects\ai-gateway`
- Add to **Agent Instructions / Prompt**:
  ```text
  System Operating System: Windows
  Workspace Root Directory: C:\me\projects\ai-gateway
  When running shell/bash tool calls, use relative paths from the workspace root (e.g. `git ls-files` or `dir`) instead of absolute hardcoded Linux paths like `/workspace/prism`.
  ```

#### Option B: Use Cross-Platform Tool Commands
Instruct Paperclip agents to use `git ls-files` or standard Node/Go CLI tools instead of Linux-only `find /workspace/prism`:

```bash
# Recommended cross-platform file listing for Paperclip agents:
git ls-files
```

---

## 3. Resolving Tool Call XML Output in Stateless Adapters

### Problem:
An agent outputs raw `<tool_call>` XML tags like:
```xml
<tool_call>
<function=paperclip>
<parameter=action>list_agents</parameter>
</function>
</tool_call>
```
resulting in `Missing disposition recovery blocked`.

### Cause:
In Phase 1 stateless LLM adapters (`prism-roozylabs`), direct tool execution is not supported. When the LLM outputs raw `<tool_call>` XML tags, no tool is executed and no issue disposition is updated.

### Solution:
Tambahkan instruksi **Stateless Execution Rule** pada Agent Instructions / Prompt di Paperclip:

```text
Stateless Execution Rule:
- Tool call XML tags (such as <tool_call> or <function>) cannot be executed by this stateless adapter.
- Do NOT output <tool_call> or <function> XML tags in text.
- Write your complete response, updates, and findings directly as clean Markdown text.
```

Dengan aturan ini:
- Model LLM (seperti `hy3-free`) tidak akan lagi memancarkan tag XML `<tool_call>`.
- `has_tool_calls` menjadi `false`.
- `finish_reason` tetap `stop`, dan status eksekusi Paperclip run akan berhasil penuh (`"stopReason": "completed"`)! 🎉

---

## 4. Resolving "Run completed. Agent did not post a summary comment" Notice

### Problem:
Paperclip Board displays:
`Run completed. Agent did not post a summary comment this run (transcript withheld — see run log).`

### Cause in Paperclip (`heartbeat-run-summary.ts`):
When a stateless adapter (like `prism-roozylabs`) returns a run summary, Paperclip inspects `resultJson.summary`. Paperclip withholds the summary transcript from the board comment if:
1. **Length > 1,200 characters** (`MAX_FALLBACK_COMMENT_CHARS`).
2. **Starts with Narration Openers** (e.g. `Let me...`, `I'll...`, `First,...`, `Checking...`, `Looking at...`).

### Recommended Solutions:

#### Solution A: Built-in Adapter Fix (Applied in `prism-roozylabs`)
The `prism-roozylabs` adapter automatically strips leading narration openers (like *"Let me..."*, *"First,..."*) and caps the summary at 1,000 characters before sending it to Paperclip.

#### Solution B: Add Prompt Rule for Concise Output Format
Add this rule to your Agent System Prompt / Skill instructions:
```text
Formatting Rule for Task Completion:
- Always format your final response as a concise summary under 1,000 characters.
- Do NOT start your final response with narration openers like "Let me...", "First,...", "I'll...", or "Checking...".
- Start directly with key result bullets (e.g. "### Summary of Work Completed:").
```

#### Solution C: Explicit Comment API Call
Instruct Paperclip agents to invoke Paperclip's comment API before ending the run:
```bash
POST /api/issues/:issue_id/comments
Content-Type: application/json

{ "body": "### Run Summary\n- Task completed successfully.\n- Updated PRD and skills." }
```

---

## 5. Verification & Observability in Prism

Once configured:
1. Every task execution triggered by Paperclip agents will hit `https://api.prism.roozylabs.com/v1/chat/completions` with `"model": "prism-auto"`.
2. **Smart Router Decision**: Prism automatically classifies task intent, pre-filters ready credentials, and routes to the optimal provider (OpenAI, Anthropic, Google Gemini).
3. **Observability**:
   - Inspect active agent executions under **`https://app.prism.roozylabs.com/agents`**.
   - Inspect cryptographic SHA-256 execution logs under **`https://app.prism.roozylabs.com/audit-trail`**.
   - Monitor real-time streaming decisions on **`https://app.prism.roozylabs.com/logs`**.
