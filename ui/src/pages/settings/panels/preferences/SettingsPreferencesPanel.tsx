import { useMemo } from "react";

import ToggleInput, { type ToggleOption } from "@galaxy-io/dls/inputs/ToggleInput";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Text, { TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { GalaxyTheme } from "@galaxy-io/dls/theme/enums";
import { useGalaxyTheme } from "@galaxy-io/dls/theme/useGalaxyTheme";
import Widget from "@galaxy-io/dls/widget/Widget";

import SettingsPanelLayout from "@/pages/settings/components/SettingsPanelLayout";

const SettingsPreferencesPanel = () => {
  const { selectedTheme, setTheme } = useGalaxyTheme();
  const themeItems = useMemo<ToggleOption[]>(
    () => [
      {
        id: GalaxyTheme.LIGHT,
        label: "Light",
        onClick: () => setTheme(GalaxyTheme.LIGHT),
      },
      {
        id: GalaxyTheme.DARK,
        label: "Dark",
        onClick: () => setTheme(GalaxyTheme.DARK),
      },
      {
        id: GalaxyTheme.SYSTEM,
        label: "System",
        onClick: () => setTheme(GalaxyTheme.SYSTEM),
      },
    ],
    [setTheme],
  );

  return (
    <SettingsPanelLayout title="Preferences">
      <Flex alignItems={AlignItems.START} padding={16} fillWidth>
        <Widget
          isFlush /* @dls-migrate widget.fillWidth: Grow the card with a `FlexItem` or a `Grid` track. */
          fillWidth
        >
          <Flex
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.SPACE_BETWEEN}
            padding={16}
            fillWidth
          >
            <Text variant={TextVariant.SECONDARY} weight={TextWeight.MEDIUM}>
              Theme
            </Text>
            <ToggleInput options={themeItems} value={selectedTheme} />
          </Flex>
        </Widget>
      </Flex>
    </SettingsPanelLayout>
  );
};

export default SettingsPreferencesPanel;
