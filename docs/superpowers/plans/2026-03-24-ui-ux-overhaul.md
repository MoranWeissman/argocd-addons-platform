# UI/UX Overhaul — Implementation Plan

> Execute sequentially, commit after each item. Test UI build after every change.

**Goal:** Transform AAP from a functional tool into a polished operational dashboard with proper information hierarchy, reduced cognitive load, and an always-present AI assistant.

**Naming convention throughout:** "Add-ons" (hyphenated, plural) in UI labels. "addons" in code/URLs.

---

## 1. Remove date/time display from top bar

**Files:** `ui/src/components/Layout.tsx`

- Remove the `<DateTimeDisplay />` component from the top bar header
- Remove the import
- Frees space and stops unnecessary 1-second re-renders

**Commit:** `fix: remove clock from top bar — declutters header`

---

## 2. Rename UI labels to proper English

**Files:** `ui/src/components/Layout.tsx`, `ui/src/views/AddonCatalog.tsx`, `ui/src/views/UpgradeChecker.tsx`, and any other views referencing these names in headings/titles

**Changes:**
- "Addon Catalog" → "Add-ons Catalog" (sidebar + page heading)
- "Upgrade Checker" → "Add-on Upgrade Checker" (sidebar + page heading)
- Other occurrences of "Addon" in user-facing text → "Add-on" or "Add-ons" as appropriate
- Keep code identifiers as `addon`/`addons` (no change to variable names, routes, API)

**Commit:** `fix: rename to Add-ons Catalog and Add-on Upgrade Checker`

---

## 3. Group sidebar navigation into sections

**Files:** `ui/src/components/Layout.tsx`

**Current:** 12 flat nav items, no grouping.

**New structure:**
```
MONITOR
  Dashboard
  Clusters
  Add-ons Catalog
  Version Matrix
  Observability

OPERATE
  Add-on Upgrade Checker
  Migration
  Dashboards

CONFIGURE
  Settings

HELP
  Docs
```

- Remove "AI Assistant" from sidebar (moving to floating bubble — item 10)
- Remove "User Info" from sidebar (moving to avatar dropdown — item 8)
- Add section labels: small uppercase text (text-[10px] text-slate-500) with top margin
- Labels hidden when sidebar is collapsed
- Divider lines between sections (border-t border-slate-700)

**Commit:** `feat: group sidebar nav into Monitor/Operate/Configure/Help sections`

---

## 4. Move User Info to top bar avatar dropdown

**Files:** `ui/src/components/Layout.tsx`

- Remove "User Info" nav item from sidebar
- Add a user avatar circle (initials or User icon) in the top bar, right side, before logout
- On click: dropdown menu with:
  - User name/role (from auth context)
  - "Change Password" link → navigates to user info page
  - "Theme: Dark/Light" toggle (move from standalone button to dropdown item)
  - Divider
  - "Log out" button (move from standalone to dropdown)
- This consolidates: theme toggle + user info + logout into one dropdown

**Commit:** `feat: user avatar dropdown in top bar — consolidates theme, user info, logout`

---

## 5. Add breadcrumbs to top bar

**Files:** `ui/src/components/Layout.tsx`, `ui/src/components/Breadcrumbs.tsx` (new)

- Create a `Breadcrumbs` component that reads the current route path
- Renders in the empty left side of the top bar (where `{/* Breadcrumb placeholder */}` is)
- Maps routes to human-readable labels:
  - `/dashboard` → Dashboard
  - `/clusters` → Clusters
  - `/clusters/cluster-prod-1` → Clusters > cluster-prod-1
  - `/addons` → Add-ons Catalog
  - `/addons/datadog` → Add-ons Catalog > datadog
  - `/migration/mig-123` → Migration > mig-123
- Clickable segments (navigate to parent)
- Last segment is non-clickable (current page)

**Commit:** `feat: add breadcrumbs to top bar for navigation context`

---

## 6. Dashboard — Add "Needs Attention" section

**Files:** `ui/src/views/Dashboard.tsx`

**Add below the hero banner, above stat cards:**

A "Needs Attention" section that shows:
- Degraded applications (addon name + cluster, clickable → cluster detail)
- Disconnected clusters (name, clickable → cluster detail)
- Version drift count (clickable → version matrix filtered to drift)
- Active/failed migrations (clickable → migration page)

**Design:**
- Cards with red/amber left border
- Only shows if there ARE problems. If everything is healthy, shows a green "All systems operational" card.
- Each item is clickable — navigates to the relevant filtered view

**API:** The dashboard stats API already returns `by_health_status.degraded` count and `disconnected_from_argocd`. For specific degraded apps, we may need to add a new field to the dashboard response, or call the existing addons API.

**Approach:** Use existing `getDashboardStats()` for counts. Add a new `getAttentionItems()` API call that returns a list of specific degraded apps and disconnected clusters (lightweight — just names and status).

**Backend:** Add `GET /api/v1/dashboard/attention` endpoint that queries ArgoCD for degraded apps and disconnected clusters, returns a list.

**Commit:** `feat: add Needs Attention section to dashboard — surfaces problems first`

---

## 7. Dashboard — Make stat cards clickable

**Files:** `ui/src/views/Dashboard.tsx`

- "Total Clusters" → navigates to `/clusters`
- "Applications" → navigates to `/addons`
- "Available Add-ons" → navigates to `/addons`
- "Active Deployments" → navigates to `/addons?filter=deployed`

Each card uses the existing `onClick` prop on `StatCard`.

**Commit:** `feat: make dashboard stat cards clickable — navigate to relevant pages`

---

## 8. Dashboard — Replace pie charts with horizontal stacked bars

**Files:** `ui/src/views/Dashboard.tsx`

**Current:** Two Recharts PieCharts (donut) for health and connectivity.

**Replace with:** Horizontal stacked segment bars showing proportions with number labels. These are better at showing small proportions (2 degraded out of 45 total is invisible in a donut but clear in a bar).

**Design:**
```
Application Health
[==========================================||==]
 45 Healthy                                2 Degraded  1 Progressing

Cluster Connectivity
[======================================================|]
 21 Connected                                           1 Disconnected
```

- Each segment is colored (green/red/yellow/gray)
- Number + label below each segment
- Total shown at the end
- Remove Recharts dependency for this section (use pure CSS/Tailwind divs)

**Commit:** `feat: replace dashboard pie charts with stacked health bars`

---

## 9. Mobile hamburger menu

**Files:** `ui/src/components/Layout.tsx`

- On screens below `lg` breakpoint:
  - Sidebar is hidden by default
  - A hamburger button (Menu icon) appears in the top bar left side
  - Clicking it opens the sidebar as an overlay (with backdrop)
  - Clicking a nav link or the backdrop closes it
- On screens `lg` and above: sidebar behaves as current (fixed, collapsible)

**Implementation:**
- Add `mobileOpen` state
- Sidebar gets: `lg:relative lg:flex` + `fixed inset-0 z-50` on mobile when open
- Backdrop overlay with `onClick={() => setMobileOpen(false)}`

**Commit:** `feat: mobile hamburger menu — sidebar overlay on small screens`

---

## 10. Floating AI Assistant bubble

**Files:**
- Create: `ui/src/components/FloatingAssistant.tsx`
- Modify: `ui/src/components/Layout.tsx` — add FloatingAssistant to layout
- Modify: `ui/src/App.tsx` — remove `/assistant` route
- Modify: `ui/src/components/Layout.tsx` — remove AI Assistant from sidebar (done in item 3)

**Design:**
- Fixed position bottom-right: `fixed bottom-6 right-6 z-50`
- Bubble: 56px circle, cyan gradient, MessageSquare icon, subtle pulse animation when idle
- On click: slide-up chat panel (400px wide, 500px tall, rounded-xl, shadow-2xl)
- Chat panel contains the SAME chat UI from AIAssistant.tsx:
  - Message history
  - Suggested prompts (on empty state)
  - Input field + send button
  - Tool execution indicator
  - Export button
  - Reset button
- Close button (X) in panel header minimizes back to bubble
- Badge on bubble shows unread count if assistant responded while minimized

**Context awareness:**
- `useLocation()` to read current route
- Build page context string: "User is on /clusters/cluster-prod-1"
- On the cluster detail page: include cluster name + addon count
- On addon detail: include addon name + deployment count
- On migration detail: include migration ID + status + current step
- Inject this as a system-level context prefix when sending chat messages

**Refactor:**
- Extract chat logic from `AIAssistant.tsx` into a reusable `useChat()` hook
- `FloatingAssistant.tsx` uses `useChat()` with page context
- `AIAssistant.tsx` page can be removed (or kept as a full-page fallback)

**Commit:** `feat: floating AI assistant bubble — context-aware, always accessible`

---

## 11. Add global search (Cmd+K)

**Files:**
- Create: `ui/src/components/CommandPalette.tsx`
- Modify: `ui/src/components/Layout.tsx` — add keyboard listener + palette

**Design:**
- `Cmd+K` (Mac) / `Ctrl+K` (Windows) opens a centered modal
- Search input at top, results below
- Searches across:
  - Cluster names (navigates to `/clusters/{name}`)
  - Addon names (navigates to `/addons/{name}`)
  - Page names (navigates to the page)
  - Doc titles (navigates to `/docs` with that page selected)
- Results show: icon + name + type label (Cluster/Addon/Page)
- Arrow keys to navigate, Enter to select, Escape to close
- Fuzzy matching (simple `includes` on lowercase)

**Data source:** On mount, fetch cluster list + addon list. Cache in memory. Page names are static.

**Also:** Add a search icon/button in the top bar that opens the palette (for users who don't know Cmd+K).

**Commit:** `feat: global command palette (Cmd+K) for quick navigation`

---

## Execution Order

1. Remove date/time (5 min)
2. Rename labels (10 min)
3. Group sidebar sections (30 min)
4. User avatar dropdown (30 min)
5. Breadcrumbs (30 min)
6. Needs Attention section (1 hr — includes new API endpoint)
7. Clickable stat cards (15 min)
8. Replace pie charts with bars (1 hr)
9. Mobile hamburger (45 min)
10. Floating AI bubble (2 hr — biggest item)
11. Command palette (2 hr)

**Total: ~9 hours estimated**

Commit after each item. PR + merge + deploy after items 5, 8, and 11 (three deploys total).
