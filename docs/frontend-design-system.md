# Prism App — Frontend Design System & Component Guidelines

**Version:** 2.2.0  
**Application:** RoozyLabs Prism Dashboard (`apps/app`)  
**Design Philosophy:** Dark-first, console-grade engineering aesthetic with high-density data visualizations, deterministic states, and strict type safety.

---

## 1. Design Tokens & Foundations

### 1.1 Color Palette
Prism uses Tailwind CSS with CSS custom variables defined in `globals.css` that support both dark (default) and light themes with semantic tokens:

| Token | Class | Semantic Purpose |
| :--- | :--- | :--- |
| **Background** | `bg-background` | Primary surface background (`#090A0F` dark) |
| **Foreground** | `text-foreground` | Primary text (`#F8FAFC` dark) |
| **Card / Surface** | `bg-card` | Elevated card surfaces (`#0D0F18` dark) |
| **Border / Muted** | `border-border`, `bg-muted` | Structural dividers & secondary backgrounds (`#1E2235`) |
| **Primary (ink)** | `bg-primary`, `text-primary` | Primary actions, active nav bar, focus ring; the foreground value (`#0F1729` light / `#F8FAFC` dark). Violet `#8B5CF6` is brand-mark and chart-series-1 only |
| **Success / Healthy** | `text-emerald-500`, `bg-emerald-500` | Operational status, positive trends, healthy state |
| **Warning / Degraded** | `text-amber-500`, `bg-amber-500` | Cooldown, approaching budget threshold |
| **Destructive / Error** | `text-destructive`, `bg-destructive` | Failed requests, circuit breaker tripped, delete actions |
| **Info / Cyan** | `text-cyan-400`, `bg-cyan-500` | Real-time streams, telemetry, speed scores |

### 1.2 Status Indicators
Status is consistently presented through semantic atom components:

- **`<StatusDot status="..." />`**: Visual pulse indicator supporting:
  - `active` / `healthy` (Emerald pulse)
  - `degraded` (Amber pulse)
  - `exhausted` / `unhealthy` (Red static)
  - `cooldown` (Amber/Yellow static)
  - `disabled` (Muted gray)
- **`<Badge variant="..." />`**: Tag badges supporting:
  - `default` / `outline` / `secondary`
  - `violet` / `success` / `warning` / `destructive` / `info`

---

## 2. Component Hierarchy & Atomic Structure

The UI is structured following a strict atomic layout:

```
components/
├── atoms/               # Pure UI primitives with zero external state
│   ├── Button.tsx       # Standard buttons with 'default' | 'outline' | 'ghost' | 'destructive' | 'prismViolet'
│   ├── Badge.tsx        # Categorical badges & StatusDot
│   ├── Input.tsx        # Form text & numeric inputs
│   ├── Textarea.tsx     # Code & prompt multi-line inputs
│   ├── Switch.tsx       # Boolean toggles
│   ├── Slider.tsx       # Numerical weight & ratio sliders
│   ├── Progress.tsx     # Percentage completion bars
│   └── Tooltip.tsx      # Hover explanation overlays
│
├── molecules/           # Composite, reusable domain components
│   ├── Card.tsx         # Standard card containers with Header/Title/Description/Content/Footer
│   ├── PageHeader.tsx   # Consistent top page title, description, and primary action slot
│   ├── MetricCard.tsx   # Dashboard KPI metrics with trend delta and loading skeleton
│   ├── ConfirmDialog.tsx# Reusable modal confirmation for destructive / tier changes
│   ├── StateAlerts.tsx  # Standardized EmptyState and ErrorState with retry triggers
│   ├── Dialog.tsx       # Radix-based modal overlay
│   ├── Sheet.tsx        # Radix-based slide-over drawers
│   ├── Select.tsx       # Custom dropdown select menus
│   └── Form.tsx         # React Hook Form context integration
│
├── organisms/           # High-density data views & complex widgets
│   ├── DataTable.tsx    # Generic, typed table with pagination, sorting & custom column renderers
│   └── ChartContainer.tsx# Lazy-loaded Recharts wrapper with custom tooltip formatting
│
└── AppLayout.tsx        # Shell layout containing Navigation, Header, TenantSelector, and UserProfile
```

---

## 3. Form Architecture & Validation Standards

In compliance with `.agents/rules/react-form-and-mutation-guidelines.md`:

### 3.1 Rules for Forms
1. **No Scattered Primitive `useState`:** Every form MUST use `useForm` from `react-hook-form` with a Zod schema resolver (`@hookform/resolvers/zod`).
2. **Dedicated Schema Files:** All Zod schemas are located under `features/[domain]/schemas/[name].schema.ts` and exported alongside TypeScript infer types:
   ```ts
   export const providerSchema = z.object({
     name: z.string().min(1, 'Name is required'),
     type: z.enum(['openai', 'anthropic', 'google', 'opencode', 'custom']),
     baseUrl: z.string().url('Must be a valid URL').or(z.literal('')),
   });
   export type ProviderFormValues = z.infer<typeof providerSchema>;
   ```
3. **Form Component Binding:** Form fields are bound using `<FormField>`, `<FormItem>`, `<FormLabel>`, `<FormControl>`, and `<FormMessage>`.
4. **All Mutations Wrapped:** Every backend write action must be encapsulated within a React Query `useMutation` hook in `hooks/mutations/`.

---

## 4. Feedback & Deterministic State Patterns

### 4.1 Loading States
Every asynchronous page and component provides explicit loading feedback:
- **`CardSkeletonGrid`**: Used in catalog pages (`/providers`, `/models`, `/agents`, `/mcp`).
- **`Skeleton` / `MetricCard.loading`**: Used in analytics metric cards.

### 4.2 Error States
Pages implement `<ErrorState onRetry={refetch} />` with an actionable retry button when queries fail.

### 4.3 Empty States
When data collections contain zero items, `<EmptyState title="..." description="..." action="..." />` provides user guidance and next-step actions.

### 4.4 Confirmation on Mutating Actions
All destructive, state-altering, or financial actions (e.g. Delete, Reset Cooldown, Set Default Policy, Upgrade Subscription) are guarded by `<ConfirmDialog />`:
```tsx
<ConfirmDialog
  title="Reset Cooldown"
  description="Are you sure you want to reset cooldown for this credential?"
  confirmText="Reset"
  onConfirm={handleResetCooldown}
  trigger={<Button size="sm" variant="outline">Reset</Button>}
/>
```

---

## 5. Strict Type Safety & Zero-`any` Standard

In compliance with `.agents/rules/typescript-enums-and-no-any.md`:
- The entire frontend codebase has **0 occurrences of `: any` or `as any`**.
- All API entities are strictly typed in `lib/api/types/*.ts`.
- Generic components (`DataTable<T>`, `Column<T>`) strictly preserve type inference across table column definitions.
