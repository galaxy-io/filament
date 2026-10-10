# filament-ui skill

The Claude skill for designing and building Galaxy product UI in this repository. Claude loads it when someone designs or builds a screen in `ui/`, adds a route or a query hook, picks DLS components, styles with DLS tokens or reviews Galaxy UI. It is committed with the app so every session has the same guidance.

The skill has two layers.

| Layer | Files | Maintained |
|---|---|---|
| Design system | `references/components.md`, `tokens.md`, `patterns.md`, `recipes.md` | with `@galaxy-io/dls`. Updated with a DLS version bump. App conventions never go here. |
| App | `SKILL.md`, `references/architecture.md`, `routing.md`, `data.md`, `screens.md`, `forms.md`, `dashboards.md`, `workflow.md` | here. They describe how Filament's `ui/src` is built and are updated with the code. |

| File | What |
|---|---|
| `SKILL.md` | The workflow, the stack, the brand rules, the rules that catch most mistakes, quick decisions, the hand-over checklist. |
| `references/architecture.md` | The standalone and module builds, the package surface, folders, naming, the file contract, imports, proto types, the state doctrine, styling habits, and the recipe for building a Galaxy module. |
| `references/routing.md` | `FilamentPath`, route options, the URL read and write hooks, search params as view state, URL-driven overlays, auth in the standalone. |
| `references/data.md` | The connect-query layer, building requests, which hook where, mutations, cache gotchas. |
| `references/screens.md` | The shells and how each kind of screen is composed, state layouts, status marks, formatting and copy. |
| `references/forms.md` | Local-state forms, validation timing, schema-driven fields, the provider contract, wizards. |
| `references/dashboards.md` | URL view state, per-panel loading, DLS charts, KPI tiles, filters. |
| `references/workflow.md` | Proto to verified screen, `pnpm check` and the greps, browser verification, false alarms, the final checklist. |
| `references/components.md` | Every DLS module, generated from the DLS docs. |
| `references/tokens.md` | Themes and tokens, generated from the DLS docs. |
| `references/patterns.md` | DLS-wide UX guidance, generated from the DLS docs. |
| `references/recipes.md` | Complete DLS-only screens. |

## Keeping it current

- When `@galaxy-io/dls` is bumped, update the four design-system references to match the new release.
- When a convention in `ui/src` changes, change the app reference that describes it in the same pull request.
- `.claude` at the repository root is a symlink to `.agents`, so Claude Code finds the skill at `.claude/skills/filament-ui` and git tracks it at `.agents/skills/filament-ui`.
