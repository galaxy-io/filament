# Product

Who uses Filament, what they come to do, how each core flow should feel, and the interaction rules that follow from it. Read it before designing anything. The other references say how to build a screen. This one says what a good one is.

## Who it is for

Two kinds of people share every screen.

- **Data and platform engineers set it up.** They connect sources and sinks, build pipelines, shape data on the canvas and debug failures. They read ids, raw errors and JSON without help.
- **Ops and analytics people watch it run.** They open Observability, check that data arrived, re-run what failed and tell an engineer when something is broken. They need plain words before raw ones.

Design for the second group and hide nothing from the first. A plain summary comes first, and the raw detail sits right below it. A server's message is never hidden behind `IS_DEBUG`. Only internal render errors are.

## How it should feel

Fast, calm and safe, in that order. When a choice trades one for another, the earlier one wins.

| | Means | Looks like |
|---|---|---|
| **Fast** | Zero to data flowing in minutes. Defaults that work. Every frequent action is one click or one key away. | The create wizard can be clicked through on defaults. `C` opens create, `/` searches, `⌘↵` saves. Run now starts a run from the header without a dialog. |
| **Calm** | Quiet until something needs you. Every state explained. Nothing moves unless the data changed. | Monochrome. Status color only on what is failing or live. Polling only while something runs. One banner at a time. Skeletons, never a flash of stale data. |
| **Safe** | Hard to break production by accident. | Canvas edits are a draft until Save. Previewing a version changes nothing. A connection is validated before it can be saved. Deletes confirm by typed name. |

Fast before safe means guardrails go only where a mistake is expensive or permanent. Run now, rename, a schedule toggle saved with its section and a re-run get no dialog. Delete, and anything that drops data or credentials, gets one.

The product to picture is **Vercel**. Its deployment list is our run history, its build log is our run activity, its status marks and its plain copy are the tone. When you are unsure what good looks like, ask how Vercel would show it.

## Voice

Plain and technical. Name the thing, say what happened, stop. No personality, no exclamation marks, no "Oops", no "Please", no "successfully".

| Write | Not |
|---|---|
| Pipeline saved | Your pipeline has been saved successfully! |
| Start failed + the server's message | Oops, something went wrong |
| No pipelines yet. Create one to move data from a source to a sink. | It's lonely in here |
| Something went wrong | Looks like there was a glitch in the matrix |
| Delete pipeline? | Are you sure? |
| Connected | Success! We were able to connect. |
| Next run in 2 hours | Your pipeline will run again soon |

There are no exceptions, error pages included. The copy mechanics (sentence case, verb buttons, toast shapes, placeholders) are in [screens.md](./screens.md#formatting-and-copy).

## Screen size

Every screen works from **1024px** wide up. At 1024 nothing clips, overlaps or scrolls the page sideways.

- Tables keep the naming column readable and drop or narrow optional columns first.
- Settings and form columns stay `maxWidth={640}`, so wide screens get whitespace, not long lines.
- Drawers keep their width and the main region gives way. The canvas sidebar and Activity pane resize within their limits, nothing covers the canvas, and the canvas keeps at least 240px on both axes.
- Nothing is designed below 1024. There is no mobile layout.

## The core flows

Seven flows carry the product, four to set it up and three to operate it. A new screen either serves one of them or starts a new one, so know which before you design. Each entry says who is there, what they do today, how it should feel and what breaks the feel.

### 1. Connect a source or a sink

**Who and why.** An engineer gives Filament access to a system to read from or write to.

**The path.** Sources (or Sinks) → **New source** → pick a connector from the searchable catalog → name it and fill the fields the connector's schema asks for → **Validate** → a green "Connected" beacon and the save button → the toast "Connection created" → the connection's drawer opens.

**How it should feel.** Certain. The user knows it works before it exists. Every field is labelled, examples in placeholders show the expected shape (`Enter host...`), and a failed validation shows the server's reason under the form, stacked as a plain line over the raw message.

**What breaks it.** Saving an untested connection. A generic "Invalid configuration" with no server detail. Losing what was typed when validation fails. Asking for a field the connector does not need.

### 2. Create a pipeline

**Who and why.** An engineer moves data from one source to one or more sinks.

**The path.** Pipelines → **New pipeline** (`C`) → the wizard. **Connections** (pick a source and sinks) → **Resources** (which streams, read mode, cursor) → **Delivery** (each sink's write mode and options) → **Details** (name, schedule, notifiers) → "Pipeline created" → the pipeline's canvas.

**How it should feel.** Fast. Every step arrives with working defaults, so a user who only picks a source and a sink can press Next to the end and get a pipeline that runs. Each step is there to review, not to fill in. While Next is blocked, a red "Invalid" chip in the footer lists exactly why.

**What breaks it.** A required decision with no default. A step that cannot be understood without docs. An error discovered only on the last step. Landing anywhere but the new pipeline.

### 3. Shape data on the canvas

**Who and why.** An engineer adds transforms and routes between the source and its sinks.

**The path.** The pipeline's **Canvas** tab → select a node or an edge, which opens the right sidebar → edit in the sidebar → the header shows unsaved changes and any save issues → **Save** writes a new version ("Pipeline saved"). Any earlier version can be previewed from history behind a preview banner.

**How it should feel.** Safe to explore. Nothing reaches production until Save, Undo always works, and a preview is read-only. Problems are listed as you go, not at save time.

**What breaks it.** Autosave. A change that applies without Save. Validation that waits for the click. A preview you can accidentally edit.

### 4. Schedule and notify

**Who and why.** An engineer makes a pipeline run unattended and tells people when it fails.

**The path.** The pipeline's **Settings** tab → the Schedule section (on, cron or interval, Save) and the Notifications section (add a notifier, pick events, Save). The header then shows a yellow schedule banner with "Next run in …", and the pipelines list shows the schedule.

**How it should feel.** Set and forget. The fields are always visible, no "Add schedule" step. The next run is always stated as a time, so the user can check the cron did what they meant.

**What breaks it.** A toggle that applies immediately while the rest of the section waits for Save. A schedule with no visible next run.

### 5. Run and watch

**Who and why.** Anyone with access starts a run now and follows it to the end.

**The path.** Pipeline header → **Run now** → "Starting run..." becomes "Run started" and the page lands on the Canvas tab with nothing else changed → the header switches to the live run's actions (cancel) → the Activity pane under the canvas shows Running in its header and holds the live log when opened. It never opens by itself → History shows the run with a live status square, its activity streaming, its resources filling in → it lands as Succeeded or Failed.

**How it should feel.** Live while it runs and still when it is done. Polling runs only while a run is active, and everything stops moving at the end. Durations and record counts are real numbers in mono.

**What breaks it.** A confirmation before Run now. A spinner with no status. A finished run that keeps polling. Status shown by color alone.

### 6. Triage a failure

**Who and why.** Ops sees a failure, an engineer fixes it. Often two people.

**The path.** Observability's failure count or a red square in a runs table → the run in the pipeline's History, expanded → the failure, stacked as a plain summary line ("Sink `warehouse` rejected 120 records") with the server's raw error directly below in mono, always visible and selectable → a direct way to the fix (open the connection, open the canvas) → **Run now** again.

**How it should feel.** Obvious. Ops can say what failed and where without reading a stack trace, and the engineer finds the raw error one glance away, ready to copy. Every step is a link, so a URL pasted into chat opens exactly the same view.

**What breaks it.** The raw error hidden behind a click, a tooltip or `IS_DEBUG`. A summary that only says "Run failed". A failure you cannot link to. A dead end with no route to the fix.

### 7. The daily glance

**Who and why.** Anyone opens Filament to confirm everything is fine, then leaves.

**The path.** Observability, the default landing page. Before setup is complete it shows the setup checklist instead.

**How it should feel.** Answered in seconds, without scrolling. Above the fold, in order of attention.

1. **Failures**, anything failing now, each a link to its run.
2. **Live runs**, what is running right now.
3. **The KPI row**, runs, success, records and volume for the timeframe.
4. **Next scheduled**, what runs next and when.

Everything healthy should look boring. Charts sit below for anyone who wants to dig, with the timeframe and pivots in the URL.

**What breaks it.** A healthy dashboard full of color. A failure you have to scroll or filter to find. A panel that blanks the page while it loads.

## Interaction rules

The calls an engineer would otherwise guess at. Each follows from the flows above.

| Question | Rule |
|---|---|
| Page or drawer for an entity's detail? | **A page when it has parts** (history, settings, an editor), with `PageLayout` `tabs`. **A drawer** when it is one record you inspect and act on beside its list. Pipelines are pages, connections are drawers. |
| Inline edit or Save? | **Inline only for identity.** Name and description in the page header are DLS `EditableText` and commit on Enter or blur. Everything else is a settings section with one Save. |
| Create flow? | A `Modal` opened by a `Flow` param. A `Stepper` wizard only when a later step depends on an earlier answer. Defaults on every step. Land on the created thing. |
| How live? | **Poll only while something is active** (a run running, a schedule about to fire), through the query's `refetchInterval` function. Otherwise rely on focus refetch. Streams only for a run's activity feed. |
| Confirm? | Only for delete and for anything that drops data or credentials, by typed name. Never for run, rename or save. |
| Timestamps? | **Relative with an absolute tooltip** in lists, cards and banners ("3m ago", tooltip with the local date, time and zone). **Absolute** in drawers, detail rows and run info. Proto `0n` is `EMPTY_VALUE`. |
| Failure text? | **Stacked.** A plain one-line summary, then the server's raw message in mono, selectable, always visible. Toasts carry the raw message as their description through `getErrorMessage`. |
| Bulk actions or row selection? | **Ask first.** No table has selection today. Propose the design and wait for a yes. |
| Category color? | From the **one registry map** of domain concept to `RoleColor`. Transforms are blue, schedules are yellow. A new concept gets an entry there, never a hue picked in a feature. |
| Status color? | Only for status, through `variant` or the run hue map. A healthy screen is monochrome. |
| Tooltips on the canvas panel chrome? | **None** (decided 2026-10-12). The open panel, Back, Delete, Close and activity caret buttons use standard icons and sit under the cursor all the time, so tooltips there fired on every pass. Each keeps its `ariaLabel`. The exception covers that chrome only, every other icon-only control keeps its tooltip. |
| Something the DLS lacks? | **Compose, then request.** Build it from DLS parts with as little local `styled` as possible, then open a GitHub issue on `galaxy-io/dls` with the file path, a screenshot and the API you wish existed. |

## Keyboard

Every screen owes keyboard users the same things.

| Key | Does | Built with |
|---|---|---|
| `C` | The page's primary create action | `Button` `hotKeys={["c"]} shouldBindHotKey` on the `PRIMARY` create button |
| `/` | Focus the list search | `ListSearch` passes `hotKeys={["/"]} shouldBindHotKey` to DLS `SearchInput` |
| `⌘↵` | Save the focused settings section, or submit a modal's primary action | `Button` `hotKeys={["mod", "enter"]} shouldBindHotKey`, scoped to the section or modal with `useHotkey`'s `scope` |
| `G` then a letter | Go to a top-level destination (`G O` Observability, `G P` Pipelines, `G S` Sources, `G K` Sinks) | needs sequence support, which DLS `useHotkey` does not have yet. Compose it and request it per the rule above |
| `↑` `↓` `↵` | Move between table rows and open one | `InfiniteTable` `onRowClick`, which brings roving row focus with it |
| `⌘K` | The command palette | the host. Every new top-level page and create flow adds its item to `useFilamentCommandItems` in `module/commands.ts` |
| `Esc` | Close the top overlay and return focus | DLS overlays do this. Never break it |

A bound `Button` ignores a shortcut without a modifier while focus is in a text field, so typing `c` in a field never opens a modal, and `⌘↵` still saves from inside one. `hotKeys` also draws the `Kbd` caps on the button or in its tooltip.
