import Field from "@galaxy-io/dls/inputs/Field";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import { Orientation } from "@galaxy-io/dls/theme/enums";
import ThemeSwitcher from "@galaxy-io/dls/theme/ThemeSwitcher";
import Widget from "@galaxy-io/dls/widget/Widget";

import SettingsPanelLayout from "@/pages/settings/components/SettingsPanelLayout";

const SettingsPreferencesPanel = () => {
  return (
    <SettingsPanelLayout title="Preferences">
      <Flex alignItems={AlignItems.START} padding={16} fillWidth>
        <FlexItem grow={1}>
          <Widget isFlush>
            <Flex padding={16} fillWidth>
              <Field
                label="Theme"
                description="System follows your operating system."
                orientation={Orientation.HORIZONTAL}
                fillWidth
              >
                <ThemeSwitcher />
              </Field>
            </Flex>
          </Widget>
        </FlexItem>
      </Flex>
    </SettingsPanelLayout>
  );
};

export default SettingsPreferencesPanel;
