# filament-ui skill

The Claude skill for designing and building Filament's web UI. Claude loads it when someone designs, builds or reviews anything in `ui/`. It is written so an engineer who does not usually write frontend can drive an agent to a screen that fits the product. It is committed with the app so every session has the same guidance.

The skill pairs with the org's `galaxy-ui` skill, which owns `@galaxy-io/dls` (components, props, tokens, DLS-wide patterns and recipes). This skill owns what is specific to Filament, and `SKILL.md` lists where Filament departs from `galaxy-ui`.

| File | What |
|---|---|
| `SKILL.md` | Loading `galaxy-ui`, where Filament departs from it, the product in one glance, the workflow, the stack, the rules that catch most mistakes, quick decisions, the hand-over checklist. |
| `references/product.md` | Who uses Filament, how it should feel (fast, calm, safe), voice, screen size, the seven core flows, the interaction rules, keyboard. |
| `references/architecture.md` | The standalone and module builds, the package surface, folders, naming, the file contract, imports, proto types, the state doctrine, styling habits, and the recipe for building a Galaxy module. |
| `references/routing.md` | `FilamentPath`, route options, the URL read and write hooks, search params as view state, URL-driven overlays, auth in the standalone. |
| `references/data.md` | The connect-query layer, building requests, which hook where, mutations, cache gotchas. |
| `references/screens.md` | The shells and how each kind of screen is composed, state layouts, status marks, formatting and copy. |
| `references/forms.md` | Local-state forms, validation timing, schema-driven fields, the provider contract, wizards. |
| `references/dashboards.md` | URL view state, per-panel loading, DLS charts, KPI tiles, filters. |
| `references/workflow.md` | Proto to verified screen, `pnpm check` and the greps, browser verification and proof, false alarms, the final checklist. |

## Keeping it current

- When a convention in `ui/src` changes, change the reference that describes it in the same pull request.
- When a product decision changes (a flow, the voice, an interaction rule), change `product.md` first.
- DLS changes belong in `galaxy-ui`, not here. Add a row to the departures table in `SKILL.md` only when Filament deliberately does something else.
- `.claude` at the repository root is a symlink to `.agents`, so Claude Code finds the skill at `.claude/skills/filament-ui` and git tracks it at `.agents/skills/filament-ui`.
