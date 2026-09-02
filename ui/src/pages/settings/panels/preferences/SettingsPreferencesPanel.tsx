import { useMemo } from "react";

import FlexWrapper, { AlignItems, JustifyContent } from "@galaxy-io/dls/containers/FlexWrapper";
import SwitcherInput, { type SwitcherInputItem } from "@galaxy-io/dls/inputs/SwitcherInput";
import Text, { TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { useGalaxyTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import { GalaxyTheme } from "@galaxy-io/dls/theme/types";
import Widget from "@galaxy-io/dls/widget/Widget";

import SettingsPanelLayout from "@/pages/settings/components/SettingsPanelLayout";

const SettingsPreferencesPanel = () => {
  const { selectedTheme, setTheme } = useGalaxyTheme();
  const themeItems = useMemo<SwitcherInputItem[]>(
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
      <FlexWrapper padding="16px" fillWidth>
        <Widget noPadding noHover fillWidth>
          <FlexWrapper
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.SPACE_BETWEEN}
            padding="16px"
            fillWidth
          >
            <Text variant={TextVariant.SECONDARY} weight={TextWeight.MEDIUM}>
              Theme
            </Text>
            <SwitcherInput items={themeItems} selectedId={selectedTheme} />
          </FlexWrapper>
        </Widget>
      </FlexWrapper>
    </SettingsPanelLayout>
  );
};

export default SettingsPreferencesPanel;
