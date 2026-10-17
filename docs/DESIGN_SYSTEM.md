# RoozyLabs Prism — Design System
**Version:** 1.0.0 (superseded)  
> Superseded on 2026-10-17 by the RoozyLabs Prism design system artifact and `.agents/skills/antislop/DESIGN.md`: the shipped UI is square-cornered, light by default with a dark theme, uses no spectrum gradient, and since the accent decision its dashboard primary is the foreground colour with violet reserved for the brand mark and the landing page. Kept for history.  
**Core Concept:** *Prism = one interface, many intelligences.*  
**Philosophy:** A single resilient core that refracts and routes requests across diverse AI providers and foundation models. Visual identity embraces beams, spectrums, refraction, layers, and nodes—executed with strict restraint and precision.

---

## 1. Brand Personality

Prism is designed for high-throughput, mission-critical AI routing and infrastructure. It intentionally avoids saturated rainbow gradients and generic "AI startup template" tropes.

* **Technical** — Precise, reliable, and predictable.
* **Minimal** — High signal-to-noise ratio; clarity over decorative clutter.
* **Modular** — Composable primitives built for complex multi-provider topologies.
* **Developer-First** — Optimized for speed, keyboard navigation, observability, and code ergonomics.
* **Infrastructure-Grade** — Feels like mission control, not a consumer chatbot.

> **Visual Direction:** Linear × Vercel × Raycast × modern infrastructure tooling, accented with restrained Prism refraction moments.

---

## 2. Brand Concept & Metaphor

```
                 ┌── OpenAI
                 │
                 ├── Anthropic
Application ── Prism ── Gemini
                 │
                 ├── Mistral
                 │
                 └── Local Models
```

Prism functions as an **intelligent abstraction layer**. Developers and client services never need to manage fragmented SDKs or provider quirks.

* **Developer Experience:** Send a standard request to `Prism`.
* **Prism Execution:** Dynamic routing, automatic fallbacks, credential resolution, telemetry collection, and response normalization.

---

## 3. Color System

Prism employs a **dark-first** palette tailored for developer tooling, telemetry screens, and high-density dashboards.

### 3.1 Core Palette
| Token | Hex | Role / Usage |
| :--- | :--- | :--- |
| `prism-black` | `#08090A` | App base background, canvas |
| `prism-surface` | `#0F1115` | Default card background, sidebars |
| `prism-surface-2` | `#151820` | Elevated containers, table headers, modals |
| `prism-border` | `#242832` | Standard border lines, dividers |
| `prism-white` | `#F5F7FA` | Primary typography, active icons |
| `prism-muted` | `#8B93A1` | Secondary text, inactive states, labels |

### 3.2 Spectrum Accent
Spectrum colors are **never used as dominant background surfaces**. They serve strictly as brand signatures, routing visualizers, and state highlights.

* `prism-violet` — `#8B5CF6`
* `prism-blue` — `#3B82F6`
* `prism-cyan` — `#06B6D4`
* `prism-green` — `#22C55E`
* `prism-amber` — `#F59E0B`

```
Visual Signature:
[ Violet (#8B5CF6) ] ──────► [ Blue (#3B82F6) ] ──────► [ Cyan (#06B6D4) ]
```

### 3.3 Semantic Colors
* **Success:** `#22C55E` (Operational, Healthy, 200 OK)
* **Warning:** `#F59E0B` (Degraded, Rate Limit Approaching, Retry)
* **Error:** `#EF4444` (Failed, 5xx, Outage, Key Revoked)
* **Info:** `#3B82F6` (Active Routing, Informational Badges)

---

## 4. Typography

| Role | Font Family | Usage |
| :--- | :--- | :--- |
| **Primary (UI / Body / Headings)** | `Inter`, system-ui, sans-serif | Clean, standard dashboard interface |
| **Monospace (Code / Data / Metrics)** | `Geist Mono` or `JetBrains Mono` | Endpoints, models, request IDs, latencies, tokens, logs |

### Monospace Formatting Standard
```
model       claude-sonnet-4-5
provider    anthropic
latency     842ms
tokens      2,481
request     req_8f29d10e7b
```

---

## 5. Logo & Geometry

Avoid literal 3D glass prisms. The logo uses a clean, abstract architectural prism mark that scales gracefully down to `20×20px` favicons and status bars.

### ASCII Conceptual Marks
```
       ╱╲
      ╱  ╲
─────╱────╲─────
    ╱      ╲
   ╱        ╲

       ◇  (Minimal Node Mark with intersecting beam)
```

* **Core Motif:** `Input Beam → Prism Node → Spectrum Refraction`
* **Icon Footprint:** Crisp line weights (1.5px), sharp or subtly rounded vertices.

---

## 6. Iconography

Standardized using **Lucide Icons**:
* **Stroke Weight:** `1.5px` (default) to `2px` (active/emphasis)
* **Style:** Rounded stroke terminals, unfilled geometric shapes (fills reserved exclusively for semantic status pills/dots).
* **Grid Units:** `16px` (compact inline), `20px` (standard navigation/action), `24px` (headers).

### Canonical Icon Mappings
| Domain Concept | Lucide Icon Equivalent |
| :--- | :--- |
| Provider / Compute | `Server` |
| Foundation Model | `BrainCircuit` |
| Gateway / Core | `Waypoints` |
| Routing / Fallback | `GitBranch` |
| API / Integration | `Brackets` |
| Outgoing Request | `ArrowUpRight` |
| Incoming Response | `ArrowDownLeft` |
| Tool / Function Calling | `Wrench` |
| Data Resource | `Database` |
| Autonomous Agent | `Bot` |
| Metrics / Telemetry | `Activity` |
| Auth / API Key | `KeyRound` |

---

## 7. Spacing System

Strict `4px` base increment scale. Maintains breathing room without wasting vertical space on developer dashboards.

| Token | Size | Application |
| :--- | :--- | :--- |
| `xs` | `4px` | Icon-to-text gap, badge padding |
| `sm` | `8px` | Button horizontal/vertical padding, compact list items |
| `md` | `12px` | Card internal padding (dense), gap between form fields |
| `lg` | `16px` | Standard card internal padding, layout gutters |
| `xl` | `24px` | Section margins, header spacing |
| `2xl` | `32px` | Major dashboard grid gaps |

Scale increments: `4px`, `8px`, `12px`, `16px`, `20px`, `24px`, `32px`, `40px`, `48px`, `64px`, `80px`, `96px`.

---

## 8. Border Radius

Prism maintains sharp, precise, geometric discipline. Avoid pill-shaped surfaces except for compact status badges and avatars.

* `xs` (`4px`): Micro-badges, small tooltips, code snippets
* `sm` (`6px`): Dropdowns, list item highlights, inputs
* `md` (`8px`): **Default buttons, standard inputs, control bars**
* `lg` (`12px`): **Default cards, modal containers, panels**
* `xl` (`16px`): Floating popovers, large preview modules

---

## 9. Button Hierarchy

Buttons must feel functional, tactical, and immediate.

* **Primary:** `[ Deploy Gateway ]` — High-contrast dark background with subtle violet/blue gradient border glow on hover.
* **Secondary:** `[ Configure ]` — Surface background (`#0F1115`) with neutral 1px border (`#242832`).
* **Ghost:** `[ View logs ]` — Transparent background, neutral text, subtle background tint on hover (`#151820`).
* **Destructive:** `[ Revoke key ]` — Subtle red tint border and text (`#EF4444`).

---

## 10. Card Architecture

Cards represent autonomous entities (providers, models, routes, or metrics).

```
┌──────────────────────────────────────────┐
│  Provider                                │
│                                          │
│  Anthropic                               │
│  claude-sonnet-4-5                       │
│                                          │
│  ● Healthy                     842ms     │
└──────────────────────────────────────────┘
```

* **Surface:** `Prism Surface` (`#0F1115`)
* **Border:** `1px solid` `Prism Border` (`#242832`)
* **Border Radius:** `12px`
* **Interaction / Hover:** Border transitions to `rgba(139, 92, 246, 0.4)` (Violet highlight).

---

## 11. Provider & Routing Visualization

The signature visual language of Prism represents active routing pipelines through restrained spectrum beams.

```
                 ┌─────────────┐
                 │ Application │
                 └──────┬──────┘
                        │ (Inbound Request)
                   ┌────▼────┐
                   │  PRISM  │
                   └────┬────┘
          ┌──────────────┼──────────────┐
          ▼              ▼              ▼
      OpenAI         Anthropic        Gemini
       ● 98%          ● 99%           ● 97%
```

* **Beam Animation:** Micro-pulse running along SVG path connections.
* **Dynamic Refraction:** Line color reflects active route (Violet) vs fallback route (Amber/Muted).

---

## 12. Information Architecture & Navigation

```
PRISM
├── Overview
├── Gateway
│   ├── Requests
│   ├── Models
│   ├── Providers
│   └── Routing
├── Infrastructure
│   ├── API Keys
│   ├── Resources
│   └── Tools
├── Observability
│   ├── Logs
│   ├── Usage
│   └── Analytics
└── Settings
```

**Top Bar Controls:**  
`Environment: Production ●` | `Search (⌘ K)` | `Notifications` | `Profile`

---

## 13. Overview Dashboard Wireframe

```
Good evening.
Your gateway is healthy.

┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│ Success Rate │  │ Requests     │  │ Models       │  │ Providers    │
│ 98.7%        │  │ 1.24M        │  │ 18           │  │ 6            │
└──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘

Request Traffic
Requests
1.2M ┤                    ╭──╮
     │               ╭────╯  ╰──╮
800k ┤          ╭────╯           ╰──
     │     ╭────╯
400k ┤─────╯
     └────────────────────────────

Provider Health
Provider       Requests      Latency     Status
────────────────────────────────────────────────────
OpenAI         482,920       412ms       ● Healthy
Anthropic      381,204       521ms       ● Healthy
Gemini         201,832       388ms       ● Healthy
Mistral        92,821        634ms       ● Degraded
```

---

## 14. Model Explorer

Models are presented as composable intelligence units.

```
┌────────────────────────────────────────────────────────┐
│ Claude Sonnet                                          │
│ anthropic / claude-sonnet-4-5                          │
│                                                        │
│ Context      200K                                      │
│ Input Cost   $3 / 1M tokens                            │
│ Output Cost  $15 / 1M tokens                           │
│ Status       ● Available                               │
└────────────────────────────────────────────────────────┘
```

**Attribute Checklist:**  
`Model Name` → `Provider Key` → `Context Window` → `Pricing (Input/Output)` → `Health / Latency`

---

## 15. Routing & Fallback Policies

Routing logic is visually transparent and inspectable.

### Sequential Fallback Flow
```
Request
   │
   ▼
[ Claude Sonnet ] (Primary)
   │
   ├── [200 OK] ────────► Normal Response
   │
   └── [Failure / 5xx]
          │
          ▼
       [ GPT-5 ] (Fallback #1)
          │
          ├── [200 OK] ──► Recovered Response
          │
          └── [Failure] ──► [ Gemini Pro ] (Fallback #2)
```

---

## 16. Developer Logs & Stream UI

High-density, real-time event telemetry in monospace.

```
20:41:23  POST /v1/chat/completions
          200   claude-sonnet-4-5   842ms   2,481 tokens   $0.041

20:41:21  POST /v1/chat/completions
          200   gpt-5               1.2s    3,012 tokens   $0.062

20:41:19  POST /v1/chat/completions
          503   gemini-2.5-pro      fallback ──► claude-sonnet-4-5 [200 OK]
```

---

## 17. Spectrum Gradient Guidelines

The signature gradient is `Violet (#8B5CF6) → Blue (#3B82F6) → Cyan (#06B6D4)`.

### Approved Use Cases
1. Logo icon stroke & core glyph.
2. Active left-border navigation indicator (`2px`).
3. Primary telemetry chart gradient fills (low opacity, e.g., 8–12%).
4. Live provider routing path highlights.
5. Focused state ring on active key inputs.

### Prohibited Use Cases
* Full-card backgrounds or entire dashboard gradients.
* Multicolored typography inside regular paragraph copy.
* Saturated rainbow buttons.

---

## 18. Core Design Principles

1. **01 — One Interface**  
   Developers interact with a single coherent API surface regardless of the backend complexity.
2. **02 — Invisible Complexity**  
   Retries, failovers, dynamic rate limiting, token streaming, and provider credential swapping occur effortlessly.
3. **03 — Observable by Default**  
   Every routing decision, millisecond of latency, and token cost is fully transparent and inspectable.
4. **04 — Provider Agnostic**  
   The UI maintains neutral elegance—it does not favor OpenAI, Anthropic, Google, or open-weight models.
5. **05 — Infrastructure First**  
   Constructed with the rigor, ergonomics, and density of enterprise developer tooling, not a consumer chatbot.

---

## 19. Brand Architecture & Positioning

```
ROOZYLABS
└── PRISM
    ├── Gateway (Proxy, Ingress, Traffic Management)
    ├── Providers (Integrations, Direct Connections)
    ├── Models (Directory, Capabilities, Token Pricing)
    ├── Routing (Policies, Fallbacks, Load Balancing)
    ├── Credentials (Key Vaults, Rate Limit Quotas)
    ├── Tools (Function Calling, Vector Retrieval)
    ├── Resources (System Prompts, Context Stores)
    └── Observability (Live Logs, Metrics, Cost Analytics)
```

> **Positioning Statement:**  
> *Prism is the AI infrastructure layer between your application and intelligence providers.*  
> **Visual Philosophy:** *One input → Prism → many possibilities.*
