# galaxy-ui skill

A Claude skill for designing and building Galaxy product UI (Filament, the GX app, any Galaxy React surface) with `@galaxy-io/dls`. Claude loads it when someone designs or builds a Galaxy screen, picks DLS components, styles with DLS tokens or reviews Galaxy UI.

| File | What |
|---|---|
| `SKILL.md` | The workflow, the brand rules, the rules that catch most mistakes, quick component decisions, a pre-handoff checklist. |
| `references/components.md` | Every module of the package: when to use it, when not, key props, an example. Generated from `docs/components.md`. |
| `references/tokens.md` | Themes and tokens. Generated from `docs/theming.md`. |
| `references/patterns.md` | UX guidance for Galaxy screens. Generated from `docs/patterns.md`. |
| `references/recipes.md` | Complete screens: settings, a table page, a form in a modal, a dashboard, a detail page, empty states, a command palette. |
| `references/migration.md` | A short 1.x → 2.0 pointer to the codemod. |

Every `tsx` example in the skill type-checks against the DLS source, and every link resolves (`pnpm skill:check`, part of `pnpm verify`).

## Install

**Claude Code (CLI or desktop).** Copy the folder into a skills directory:

```bash
# for you, in every project
cp -R skills/galaxy-ui ~/.claude/skills/
# or for one project, committed with it
cp -R skills/galaxy-ui path/to/app/.claude/skills/
```

**Claude apps (claude.ai, desktop, mobile).** Build the zip and upload it as a custom skill:

```bash
pnpm skill:pack   # writes dist-skill/galaxy-ui.zip
```

Then in Claude: Settings → Capabilities → Skills → Upload skill, and choose `dist-skill/galaxy-ui.zip`.

Re-copy or re-upload after the DLS changes; the skill describes the package version in this repository.

## Changing the skill

- Edit `SKILL.md`, `references/recipes.md` and `references/migration.md` here.
- `references/components.md`, `tokens.md` and `patterns.md` are generated: edit `docs/components.md`, `docs/theming.md` or `docs/patterns.md`, then run `pnpm skill:sync`.
- Run `pnpm skill:check` (or `pnpm verify`) before committing.
