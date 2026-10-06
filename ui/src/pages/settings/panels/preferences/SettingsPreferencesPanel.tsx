import ToggleInput, { type ToggleOption } from "@galaxy-io/dls/inputs/ToggleInput";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { GalaxyTheme } from "@galaxy-io/dls/theme/enums";
import { useGalaxyTheme } from "@galaxy-io/dls/theme/useGalaxyTheme";
import Widget from "@galaxy-io/dls/widget/Widget";

import SettingsPanelLayout from "@/pages/settings/components/SettingsPanelLayout";

const SETTINGS_PREFERENCES_THEME_OPTIONS: ToggleOption<GalaxyTheme>[] = [
  { id: GalaxyTheme.LIGHT, label: "Light" },
  { id: GalaxyTheme.DARK, label: "Dark" },
  { id: GalaxyTheme.SYSTEM, label: "System" },
];

const SettingsPreferencesPanel = () => {
  const { selectedTheme, setTheme } = useGalaxyTheme();

  return (
    <SettingsPanelLayout title="Preferences">
      <Flex alignItems={AlignItems.START} padding={16} fillWidth>
        <FlexItem grow={1}>
          <Widget isFlush>
            <Flex
              alignItems={AlignItems.CENTER}
              justifyContent={JustifyContent.SPACE_BETWEEN}
              padding={16}
              fillWidth
            >
              <Text variant={TextVariant.SECONDARY} weight={TextWeight.MEDIUM}>
                Theme
              </Text>
              <ToggleInput
                options={SETTINGS_PREFERENCES_THEME_OPTIONS}
                value={selectedTheme}
                onChange={setTheme}
              />
            </Flex>
          </Widget>
        </FlexItem>
      </Flex>
    </SettingsPanelLayout>
  );
};

export default SettingsPreferencesPanel;
