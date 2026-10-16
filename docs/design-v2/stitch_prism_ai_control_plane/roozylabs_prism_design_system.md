# RoozyLabs Prism — Design System

> **AI Infrastructure Control Plane**
>
> A high-density, dark-first design system for operating AI gateways, models, credentials, agents, routing, governance, and infrastructure.

---

## 1. Design Direction

### Core idea

**Prism should feel like infrastructure software, not an analytics dashboard.**

The interface should communicate:

- Control
- Reliability
- Intelligence
- Observability
- Precision
- Security
- Speed

Visual references:

- Cloud infrastructure consoles
- Developer tools
- Modern observability platforms
- AI infrastructure products
- High-end technical SaaS

Avoid:

- Excessive gradients
- Huge marketing-style typography inside the console
- Excessive rounded cards
- Excessive shadows
- Decorative illustrations
- Dashboard "gamification"
- Every metric becoming a colored KPI card

### Visual principle

> **Dense information, low visual noise.**

The user should be able to scan:

**System → Provider → Credential → Model → Route → Agent → Cost → Outcome**

without feeling overwhelmed.

---

# 2. Brand Personality

| Attribute | Direction |
|---|---|
| Technical | High |
| Premium | High |
| Friendly | Medium |
| Playful | Low |
| Enterprise | High |
| Developer-focused | Very High |
| Visual density | High |
| Decoration | Low |
| Motion | Subtle |
| Contrast | High |

### Prism should feel

> "The operating system for AI infrastructure."

Not:

> "Another AI dashboard."

---

# 3. Color System

## 3.1 Dark Theme — Primary

Dark mode is the canonical Prism experience.

### Background

| Token | Value | Usage |
|---|---|---|
| `prism-bg-base` | `#08090A` | Application background |
| `prism-bg-subtle` | `#0B0D10` | Secondary sections |
| `prism-bg-card` | `#0F1115` | Cards |
| `prism-bg-elevated` | `#14171D` | Dropdowns / popovers |
| `prism-bg-hover` | `#181C23` | Hover |
| `prism-bg-active` | `#1C2028` | Active state |

The existing repository already uses `#08090A` and `#0F1115`; keep those as the foundation. 

---

## 3.2 Borders

Borders should be subtle.

| Token | Value |
|---|---|
| `border-subtle` | `#1B1F27` |
| `border-default` | `#242832` |
| `border-strong` | `#343A46` |
| `border-focus` | `#8B5CF6` |

Do not use bright white borders.

---

# 4. Brand Accent

## Prism Violet

Primary signature:

`#8B5CF6`

Use for:

- Primary actions
- Active navigation
- Selected states
- Focus states
- Routing intelligence
- Prism Auto
- Important links
- Brand identity

Do not make every element violet.

### Supporting accents

| Semantic | Color |
|---|---|
| Info / Cyan | `#06B6D4` |
| Success / Emerald | `#10B981` |
| Warning / Amber | `#F59E0B` |
| Error / Red | `#EF4444` |
| Neutral | `#64748B` |

The existing theme already uses these semantic colors. 

---

# 5. AI Intelligence Colors

Prism needs one additional visual layer for AI-specific concepts.

| Concept | Color |
|---|---|
| Routing | Violet |
| Model | Cyan |
| Credential | Emerald |
| Agent | Blue |
| Tool | Amber |
| Resource | Teal |
| MCP | Purple |
| Governance | Red |
| Cost | Yellow |
| Audit | Slate |

This lets users visually understand infrastructure domains without relying entirely on icons.

---

# 6. Typography

## Primary UI Font

Use:

**Inter**

Fallback:

```text
-apple-system,
BlinkMacSystemFont,
"Segoe UI",
Roboto,
sans-serif
```

The current repository uses the system stack. 

---

## Technical Font

Use:

**JetBrains Mono**

For:

- API keys
- Model IDs
- Provider IDs
- Request IDs
- Agent IDs
- Latency
- Token counts
- Cost
- HTTP methods
- Endpoint paths
- Code
- Logs
- Infrastructure metrics

Example:

```text
prism-auto
prism_sk_••••••••91f2
gpt-5
req_01K8F7...
128 ms
$0.00421
```

---

# 7. Typography Scale

| Token | Size | Weight |
|---|---:|---:|
| `display` | 32px | 700 |
| `page-title` | 24px | 700 |
| `section-title` | 18px | 600 |
| `card-title` | 15px | 600 |
| `body` | 14px | 400 |
| `body-medium` | 14px | 500 |
| `caption` | 12px | 400 |
| `micro` | 11px | 500 |
| `metric` | 24px | 700 |
| `metric-large` | 32px | 700 |

Avoid extremely large headings inside the application.

---

# 8. Spacing System

Use a 4px base grid.

```text
4
8
12
16
20
24
32
40
48
64
```

### Recommended application spacing

| Context | Spacing |
|---|---:|
| Icon → label | 8px |
| Form fields | 16px |
| Card internal | 20px |
| Card → card | 16px |
| Section → section | 32px |
| Page padding | 24px |
| Large page section | 40px |

---

# 9. Radius

Prism should be moderately rounded.

| Component | Radius |
|---|---:|
| Input | 6px |
| Button | 6px |
| Tag | 6px |
| Small card | 8px |
| Card | 10px |
| Modal | 12px |
| Large container | 12px |
| Avatar | 50% |

Avoid 20–24px "consumer SaaS" cards.

---

# 10. Layout System

## Global Application

```text
┌───────────────────────────────────────────────────────────────┐
│ Sidebar │ Topbar                                               │
│         ├──────────────────────────────────────────────────────┤
│         │                                                      │
│         │ Page Header                                         │
│         │                                                      │
│         │ Main Content                                        │
│         │                                                      │
│         │                                                      │
│         │                                                      │
└─────────┴──────────────────────────────────────────────────────┘
```

### Sidebar

Desktop:

**248px**

Collapsed:

**72px**

The existing Prism application already uses a 250px sidebar. 

Keep this.

---

# 11. Sidebar Design

Sidebar hierarchy:

```text
PRISM

Overview
  Dashboard
  AI Sandbox

AI INFRASTRUCTURE
  Providers
  Credentials
  Models
  Routing

GATEWAYS
  Gateway Keys
  Tool Gateway
  Resource Gateway
  MCP Gateway
  Agent Gateway

GOVERNANCE
  Policies
  Governance & RBAC
  Audit Trail

OPERATIONS
  Request Logs
  Budgets
  Playground

SYSTEM
  Settings
```

The existing sidebar contains these functional areas already; the improvement should primarily be **grouping and hierarchy**, rather than adding more navigation items. 

### Sidebar rule

Maximum:

**5 navigation groups**

Do not create a 20-item flat menu.

---

# 12. Sidebar Active State

Active navigation:

```text
┌──────────────────────────────┐
│ ◇  Dashboard                 │
└──────────────────────────────┘
```

Use:

- Violet-tinted background
- Violet icon
- White text
- 2–3px left indicator

Do not use a huge filled purple pill.

---

# 13. Topbar

Topbar height:

**56–64px**

Structure:

```text
[☰]  Dashboard

                         ● Operational
                         v2.1.0
                         ◐
                         Avatar
```

### System status

Always visible.

Examples:

```text
● Operational
● Degraded
● Connecting
● Incident
```

Use semantic status colors.

The existing implementation already exposes real-time SSE system status; retain this as a first-class infrastructure indicator. 

---

# 14. Page Header

Every major page should follow:

```text
Dashboard

Monitor your AI infrastructure, traffic, cost and system health.

[Workspace ▼]                         [Time range ▼]
```

For resource pages:

```text
Credentials

Manage provider credentials, health and rotation policies.

                                      [+ Add Credential]
```

### Rule

Every page needs:

1. Title
2. One-line description
3. Primary action
4. Optional filters

---

# 15. Dashboard Design

The Dashboard should NOT become a wall of KPI cards.

Recommended:

```text
┌─────────────────────────────────────────────────────────────┐
│ AI Infrastructure Overview                     Operational ● │
│                                                             │
│  Requests       Tokens        Cost          Success Rate    │
│  1.24M          48.2M         $182.42       99.94%         │
└─────────────────────────────────────────────────────────────┘

┌──────────────────────────────┐ ┌───────────────────────────┐
│ Request Traffic              │ │ Gateway Health             │
│                              │ │                             │
│        ╱╲                    │ │ OpenAI       ● Healthy     │
│   ╱╲  ╱  ╲                   │ │ Anthropic    ● Healthy     │
│ ╱  ╲╱    ╲                  │ │ Gemini       ● Degraded    │
└──────────────────────────────┘ └───────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ Recent AI Activity                                           │
│                                                             │
│  19:42  prism-auto → claude-sonnet      184ms    $0.0032   │
│  19:41  agent:dev → gpt-5                92ms    $0.0018   │
│  19:41  agent:qa → gemini                 211ms   $0.0009   │
└─────────────────────────────────────────────────────────────┘
```

---

# 16. Metric Cards

Metric cards should be restrained.

```text
REQUESTS

1,284,291
+12.4%

vs previous period
```

Use:

- Label
- Large metric
- Delta
- Small contextual information

Avoid:

- giant icons
- rainbow gradients
- decorative illustrations

---

# 17. Cards

Default card:

```text
┌──────────────────────────────────────────────┐
│ Provider Health                     View all │
│                                              │
│ OpenAI      ● Healthy              99.98%    │
│ Anthropic   ● Healthy              99.94%    │
│ Gemini      ● Degraded             97.21%    │
└──────────────────────────────────────────────┘
```

Card header:

- 14–15px
- semibold
- optional action on right

---

# 18. Tables

Tables are extremely important for Prism.

Use dense tables.

Example:

```text
MODEL
─────────────────────────────────────────────────────────────
Model              Provider       Latency     Cost       Status

gpt-5              OpenAI         182ms       $0.0032    ●
claude-sonnet      Anthropic      211ms       $0.0041    ●
gemini-2.5-pro     Google         164ms       $0.0028    ●
```

### Table rules

- Row height: 48–56px
- Header: 12px uppercase or semibold
- Monospace for IDs and metrics
- Hover row
- Sticky header for long tables
- Pagination
- Column visibility when necessary

---

# 19. Status System

Never communicate infrastructure status through color alone.

Use:

```text
● Healthy
● Degraded
● Cooldown
● Exhausted
● Disabled
```

This maps directly to Prism's credential health state machine. 

### Credential health

```text
Healthy      90–100
Degraded     60–89
Cooldown     temporary
Exhausted    0 / quota unavailable
Disabled     manually disabled
```

Use status dot + text.

---

# 20. Credential Health Visualization

Credential pages should emphasize operational health.

```text
Credential

OpenAI Production #03

● Healthy

Health Score
92 / 100

Success Rate       99.8%
Remaining Quota    74%
Cooldowns          2
Requests           124,921
```

Health score:

- Large numeric value
- Progress indicator
- Supporting metrics
- Recent health events

Do not make the health score a giant circular gauge.

---

# 21. Provider Cards

Provider cards:

```text
┌─────────────────────────────────────────────┐
│ OpenAI                              ● Active │
│                                             │
│ 12 models                                   │
│ 8 credentials                               │
│                                             │
│ 99.94% availability     182ms avg latency   │
│                                             │
│ [Manage]                                    │
└─────────────────────────────────────────────┘
```

Provider logo:

24–28px.

Never oversized.

---

# 22. Model Cards

Model identity should be technical.

```text
┌─────────────────────────────────────────────┐
│ gpt-5                               ● Active │
│ openai/gpt-5                                │
│                                             │
│ Context       400K                          │
│ Input         $1.25 / 1M                    │
│ Output        $10.00 / 1M                   │
│                                             │
│ Routing score                     92.4      │
└─────────────────────────────────────────────┘
```

---

# 23. Prism Auto

`prism-auto` is a signature feature and deserves unique treatment.

### Visual identity

Use:

**Violet + subtle animated prism/light effect**

But keep it restrained.

Example:

```text
PRISM AUTO

Intelligent model routing

Quality       35%
Cost          25%
Speed         20%
Health        20%

                    Score
                    92.4
```

---

# 24. Routing Visualization

Routing should visually communicate:

```text
REQUEST
   │
   ▼
CLASSIFICATION
   │
   ├── Quality
   ├── Cost
   ├── Speed
   └── Health
   │
   ▼
CANDIDATE MODELS
   │
   ▼
SELECTED MODEL
   │
   ▼
EXECUTION
```

Use a horizontal pipeline on desktop.

Each step is a compact node.

---

# 25. Routing Playground

The Playground should feel like a developer tool.

```text
┌──────────────────────────────────────────────────────────────┐
│ Routing Playground                                           │
├────────────────────────────┬─────────────────────────────────┤
│ Request                    │ Routing Decision                 │
│                            │                                 │
│ model: prism-auto          │ Classification                   │
│ temperature: 0.2           │ → coding                         │
│                            │                                 │
│ prompt:                    │ Candidates                      │
│ > Fix authentication...    │                                 │
│                            │ gpt-5             91.2           │
│                            │ claude-sonnet      89.4           │
│ [Run Simulation]           │ gemini            76.8           │
└────────────────────────────┴─────────────────────────────────┘
```

The interface should feel closer to an IDE than a marketing page.

---

# 26. Agent Gateway

Agents should be treated as infrastructure entities.

Agent card:

```text
DEV AGENT

● Online

Agent ID
dev-agent

Allowed Models
gpt-5
claude-sonnet

Tools
GitHub
Filesystem

Budget
$120 / month

Requests
28,421
```

---

# 27. Tool Gateway

Tools need a compact operational interface.

```text
Tool
github_search

Type
REST

Status
● Healthy

Requests
18,294

Success
99.7%

Fallback
enabled
```

Tool status should be immediately visible.

---

# 28. MCP Gateway

MCP should have:

- Registry
- Servers
- Capabilities
- Health
- Transport
- Tools
- Resources

Example:

```text
MCP SERVER

Linear MCP

● Verified

Transport
HTTP / SSE

Tools
24

Resources
8

Health
99.8%
```

---

# 29. Audit Trail

Audit Trail should look like a security console.

```text
19:42:31

agent:dev
→ request
→ gpt-5
→ tool: github.search
→ $0.0032
→ success

Hash
a83f9d...71bc
```

Use:

- Monospace
- Timeline
- Hash truncation
- Expandable details
- Verification badge

Prism's audit system is cryptographically signed, so the UI should visually reinforce that trust model. 

---

# 30. Logs

Logs should use a terminal-inspired interface.

```text
19:42:31  POST  /v1/chat/completions
19:42:31  200   model=gpt-5 latency=182ms
19:42:32  POST  /v1/chat/completions
19:42:32  429   credential=openai-prod-02
19:42:32  INFO  rotating credential
19:42:32  200   credential=openai-prod-03
```

Color only:

- Errors
- Warnings
- Important statuses

Do not color every log line.

---

# 31. Budget / FinOps

FinOps should emphasize decision making.

Top section:

```text
MONTHLY SPEND

$4,821.32

72% of budget

██████████████░░░░░

Forecast
$6,214

Budget
$6,700
```

Then:

```text
Spend by Provider
Spend by Model
Burn Rate
Forecast
Optimization Opportunities
```

---

# 32. Forms

Forms should be compact.

```text
Provider Name
[ OpenAI                              ]

API Endpoint
[ https://api.openai.com/v1           ]

API Key
[ ••••••••••••••••••••••• ] [Reveal]

Status
[● Active]

                         [Cancel] [Save]
```

### Form principles

- Labels above fields
- No floating labels
- Clear validation
- One primary action
- Dangerous actions separated

---

# 33. Buttons

### Primary

Violet background.

```text
[ + Add Provider ]
```

### Secondary

Dark/elevated background.

```text
[ Configure ]
```

### Tertiary

Text button.

```text
View details →
```

### Destructive

Only for destructive actions.

```text
[ Disable Credential ]
```

Never use red for normal actions.

---

# 34. Inputs

Default:

```text
height: 36–40px
radius: 6px
```

Focus:

```text
border: violet
box-shadow: 0 0 0 2px rgba(139,92,246,.15)
```

Inputs should remain compact.

---

# 35. Tags

Use tags primarily for state.

```text
● Active
● Healthy
● Degraded
● Beta
● Verified
```

Avoid tags as decorative metadata.

---

# 36. Empty States

Never use a huge illustration.

Example:

```text
No credentials yet

Connect your first AI provider credential to start routing requests.

[ + Add Credential ]
```

Optional subtle icon.

---

# 37. Loading States

Prefer skeletons over spinners.

For tables:

```text
████████████
████████
██████████████
```

For metrics:

```text
██████
██████████
```

Use spinners only for short interactions.

---

# 38. Error States

Example:

```text
Unable to load provider health

The gateway could not retrieve provider status.

[ Retry ]
```

Do not show raw stack traces by default.

---

# 39. Notifications

Use notifications for:

- Credential rotation
- Provider failure
- Budget threshold
- Policy denial
- Agent creation
- MCP registration
- Deployment

Example:

```text
Credential rotated

OpenAI Production #02 reached its rate limit.
Prism automatically switched to #03.
```

This reinforces Prism's automation.

---

# 40. Motion

Motion should communicate system activity.

Use:

- 150–200ms transitions
- subtle hover
- skeleton shimmer
- status pulse
- route execution animation
- streaming indicators

Avoid:

- bouncing cards
- excessive parallax
- large page transitions
- decorative animations

### Special motion

Prism Auto may use a very subtle animated violet/cyan light effect.

---

# 41. Icons

Use:

**Ant Design Icons**

The repository already uses `@ant-design/icons`. 

Rules:

- 16px standard
- 18px navigation
- 20px primary action
- 24px empty-state icon

Icons should reinforce meaning, not decorate every heading.

---

# 42. Data Visualization

Charts should be operational.

Good:

- Traffic over time
- Latency
- Error rate
- Cost
- Token consumption
- Credential health
- Routing score
- Provider availability

Avoid:

- Pie chart overload
- 3D charts
- decorative radial charts
- excessive gradients

### Chart principle

> If the chart does not help make an operational decision, remove it.

---

# 43. Light Theme

Light mode remains supported but is secondary.

Background:

```text
#F7F8FA
```

Surface:

```text
#FFFFFF
```

Border:

```text
#E5E7EB
```

Text:

```text
#111827
```

Primary:

```text
#7C3AED
```

Light mode should feel like:

**developer SaaS / cloud console**

not:

**generic Ant Design admin template**.

The current repository already supports both modes through `ThemeContext`; keep the architecture rather than creating a separate visual system. 

---

# 44. Responsive Behavior

### Desktop

Primary target.

```text
≥ 1280px
```

### Laptop

```text
1024–1279px
```

Collapse sidebar automatically if necessary.

### Tablet

```text
768–1023px
```

Use drawer navigation.

### Mobile

```text
< 768px
```

Prioritize:

- Dashboard
- Logs
- Credentials
- Models
- Agents

Complex tables should become horizontally scrollable or card-based.

---

# 45. Dashboard Density

Target:

**60–70% information / 30–40% whitespace**

This is important.

The screenshot reference has a more spacious SaaS layout. Prism should borrow the **composition**, but increase information density because Prism is infrastructure software.

---

# 46. Visual Hierarchy

Every screen should have:

```text
LEVEL 1
Page title

LEVEL 2
Important metric / primary state

LEVEL 3
Cards / tables

LEVEL 4
Metadata

LEVEL 5
Technical details
```

Use size, weight and spacing before using color.

---

# 47. Design Tokens

Recommended implementation structure:

```text
theme/
├── prismTheme.ts
├── tokens.ts
├── colors.ts
├── typography.ts
├── spacing.ts
├── radius.ts
└── components.ts
```

Example:

```ts
export const prismTokens = {
  colors: {
    bgBase: '#08090A',
    bgCard: '#0F1115',
    bgElevated: '#14171D',

    borderSubtle: '#1B1F27',
    borderDefault: '#242832',
    borderStrong: '#343A46',

    primary: '#8B5CF6',
    info: '#06B6D4',
    success: '#10B981',
    warning: '#F59E0B',
    error: '#EF4444',
  },

  radius: {
    sm: 6,
    md: 8,
    lg: 10,
    xl: 12,
  },

  spacing: {
    xs: 4,
    sm: 8,
    md: 12,
    lg: 16,
    xl: 24,
    '2xl': 32,
    '3xl': 40,
  },
};
```

---

# 48. Component Architecture

Build Prism UI around reusable primitives.

```text
Prism UI
│
├── Layout
│   ├── Sidebar
│   ├── Topbar
│   ├── PageHeader
│   └── PageContainer
│
├── Data Display
│   ├── MetricCard
│   ├── StatusBadge
│   ├── HealthScore
│   ├── DataTable
│   └── ActivityFeed
│
├── Infrastructure
│   ├── ProviderCard
│   ├── CredentialCard
│   ├── ModelCard
│   ├── AgentCard
│   ├── ToolCard
│   └── MCPServerCard
│
├── Routing
│   ├── RoutingScore
│   ├── RoutingPipeline
│   ├── CandidateList
│   └── DecisionPanel
│
├── Observability
│   ├── LogViewer
│   ├── RequestTimeline
│   ├── HealthIndicator
│   └── AuditEvent
│
└── Forms
    ├── ProviderForm
    ├── CredentialForm
    ├── ModelForm
    ├── AgentForm
    └── PolicyForm
```

---

# 49. Page Template

All major Prism pages should follow one consistent template.

```text
┌─────────────────────────────────────────────────────────────┐
│ Page Title                                      [Primary CTA]│
│ Description                                                 │
├─────────────────────────────────────────────────────────────┤
│ Filters / Tabs / Search                                     │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ Main Content                                                │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

This should become the default page skeleton.

---

# 50. Dashboard / Landing Page Relationship

The **landing page** and **dashboard** should share the same design language but not the same density.

### Landing

More:

- whitespace
- storytelling
- gradients
- product visuals
- large typography

### Dashboard

More:

- data
- tables
- status
- controls
- operational information

Both should clearly belong to Prism.

---

# 51. Prism Visual Signature

The Prism brand should be recognizable through five things:

### 1. Dark infrastructure canvas

`#08090A`

### 2. Violet intelligence

`#8B5CF6`

### 3. Technical typography

JetBrains Mono

### 4. Compact operational UI

Dense tables, metrics and status indicators.

### 5. Prism light

A restrained violet → blue → cyan spectrum used only for:

- logo
- Prism Auto
- selected hero visuals
- important AI intelligence moments

---

# 52. Screenshot Reference Adaptation

The supplied reference screenshot has several patterns worth adopting:

### Keep

- Left navigation
- Large page canvas
- Clear endpoint/product areas
- Compact cards
- Strong section hierarchy
- Query/playground interaction
- Recent activity
- Generous page-level structure

### Change for Prism

Instead of:

```text
Light + soft + colorful SaaS
```

Prism becomes:

```text
Dark + technical + precise + operational
```

Instead of multiple colorful endpoint cards, Prism should prioritize:

```text
Infrastructure Health
Routing
Providers
Credentials
Agents
Requests
Cost
```

---

# 53. Final Visual Formula

The Prism UI should roughly follow:

```text
70% Neutral Surface
20% Typography / Data
8% Semantic Colors
2% Brand Accent
```

The violet accent should feel valuable because it is **rare**.

---

# 54. Design Principle

The most important Prism UI rule:

> **Show the system state first. Show the configuration second. Show the implementation details third.**

For example:

Bad:

```text
OpenAI Credential
API Key
Created At
Provider
```

Better:

```text
OpenAI Production #03

● Healthy       Health 94

99.8% success
74% quota remaining
182ms latency

API key
Provider
Created at
```

The user should understand the operational state before reading configuration metadata.

---

# 55. North Star

Every Prism screen should answer one of these questions immediately:

**What is happening?**

**Is the system healthy?**

**What is costing money?**

**Where is traffic going?**

**Which credential is being used?**

**Why did Prism choose this model?**

**Which agent is doing this?**

**What failed?**

**What action should I take?**

If a UI element does not help answer one of those questions, it should probably be removed.

---

## Final Design Direction

**RoozyLabs Prism**

> **Control AI. Route intelligently. Operate reliably.**

Visual language:

**Dark-first + technical + high-density + premium + calm + intelligent.**

The screenshot should be treated as the **layout inspiration**, while the existing Prism dark theme, routing system, credential health, agent gateway, audit trail and FinOps capabilities become the actual visual language of the product.