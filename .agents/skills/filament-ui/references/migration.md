# Migrating an app from DLS 1.x

Use this when the app still imports 1.x modules (`containers/FlexWrapper`, `theme/GalaxyTheme`, `badge/Badge`) or styles with `withTheme`. Don't hand-port screens one by one: run the codemod first, then resolve what it flagged with the migration guide.

## Steps

1. Bump `@galaxy-io/dls` to `^2.0.0` and add the optional peers the app needs (`framer-motion` almost always; `d3-scale` + `d3-shape` for charts; `@tanstack/react-table` + `@tanstack/react-virtual` for tables).
2. Run the codemod over the source:

   ```bash
   npx @galaxy-io/dls-codemod src --report dls-migrate.json
   ```

   It rewrites module paths, component and enum renames, prop renames, token paths and `withTheme` removal, and leaves a `@dls-migrate <id>: <message>` comment wherever a person has to decide.
3. Format, then resolve every comment: `grep -rn "@dls-migrate" src`. Each `<id>` has one row in the [migration guide](https://github.com/galaxy-io/dls/blob/main/docs/MIGRATION.md) saying what to do.
4. Type-check (`tsc --noEmit`) and lint with `"extends": ["@galaxy-io/dls/biome"]`; remaining errors name the props or types to look up in the guide.
5. Compare every screen in Dark and Light with the 1.x build: it should look refined (taller `MEDIUM` controls at 32px, tighter radii, hairlines, monochrome selection), not redesigned.

## The renames you will meet most

| 1.x | 2.0 |
|---|---|
| `GalaxyThemeProvider` + `OverlayProvider` + `ToastProvider` | `GalaxyProvider` (`theme/GalaxyProvider`) |
| `withTheme`, `${({ theme }) => theme.color…}` | `t` from `theme/tokens/t`: `${t.color.text.primary}` |
| `background.primaryAlt`, `text.primaryAlt`, `*_ALT` variants | an inverse scope (a scoped `GalaxyProvider` with the opposite theme) |
| `FlexWrapper`, `GridWrapper`, `Wrapper` + `Spacing` | `Flex`, `Grid`, `Box` (`gap` / `padding` from the space scale, no margins) |
| `HorizontalDivider`, `VerticalDivider` | `Divider orientation` |
| `Badge` | `Chip count` |
| `TextShimmer`, `AvatarShimmer` | `Skeleton`, `Avatar isLoading` |
| `HotKeys` | `Kbd` |
| `MultiTextInput` | `TagInput` |
| `SwitcherInput` (segmented) | `ToggleInput` |
| `ToggleInput` (on / off) | `SwitchInput` |
| `Paragraph` | `Text as="p" isProse` |
| `open`, `onClose` | `isOpen`, `onOpenChange` |
| `isMonospace` | `family={FontFamily.MONO}` |
| `contentWhenDropdown` | `Button dropdown` |
| hue `variant`s (`ChipVariant.PURPLE`) | `color="purple"` |
| `InputVariant.TRANSPARENT`, `noBorder` on fields | `isGhost` |
| `DropdownPosition`, `TooltipPosition` | `Placement` (`theme/enums`) |

After the migration, build new UI with the rest of this skill.
