import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import CopyInput from "@galaxy-io/dls/inputs/CopyInput";

import type { ServiceAccountCredentials } from "@/pages/settings/types";
import { buildCliLoginCommand } from "@/pages/settings/utils";

interface SettingsServiceAccountsPanelCredentialsProps {
  credentials: ServiceAccountCredentials;
}

const SettingsServiceAccountsPanelCredentials = ({
  credentials,
}: SettingsServiceAccountsPanelCredentialsProps) => (
  <FlexWrapper direction={FlexDirection.COLUMN} gap={12} fillWidth>
    <CopyInput label="Client ID" value={credentials.clientId} fillWidth isMonospace />
    <CopyInput label="Client secret" value={credentials.clientSecret} fillWidth isMonospace />
    <CopyInput
      label="CLI command"
      labelTooltip="Run this command, then enter the client secret when prompted."
      value={buildCliLoginCommand(credentials.clientId)}
      fillWidth
      isMonospace
    />
  </FlexWrapper>
);

export default SettingsServiceAccountsPanelCredentials;
