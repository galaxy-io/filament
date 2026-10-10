import type { FC } from "react";

import CopyInput from "@galaxy-io/dls/inputs/CopyInput";
import Field from "@galaxy-io/dls/inputs/Field";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import type { ServiceAccountCredentials } from "@/pages/settings/types";
import { buildCliLoginCommand } from "@/pages/settings/utils";

interface SettingsServiceAccountsPanelCredentialsProps {
  credentials: ServiceAccountCredentials;
}

const SettingsServiceAccountsPanelCredentials: FC<SettingsServiceAccountsPanelCredentialsProps> = ({
  credentials,
}) => (
  <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={12} fillWidth>
    <CopyInput label="Client ID" value={credentials.clientId} fillWidth family={FontFamily.MONO} />
    <CopyInput
      label="Client secret"
      value={credentials.clientSecret}
      fillWidth
      family={FontFamily.MONO}
    />
    <Field
      label="CLI command"
      labelTooltip="Run this command, then enter the client secret when prompted."
      fillWidth
    >
      <CopyInput
        value={buildCliLoginCommand(credentials.clientId)}
        fillWidth
        family={FontFamily.MONO}
      />
    </Field>
  </Flex>
);

export default SettingsServiceAccountsPanelCredentials;
