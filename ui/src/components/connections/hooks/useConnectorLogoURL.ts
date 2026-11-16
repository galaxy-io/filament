import { useState } from "react";

import { match } from "ts-pattern";

import { GalaxyTheme } from "@galaxy-io/dls/theme/enums";
import { useGalaxyTheme } from "@galaxy-io/dls/theme/useGalaxyTheme";

import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

interface ConnectorLogoURLState {
  failedLogoURL?: string;
}

const DEFAULT_STATE: ConnectorLogoURLState = {};

export const useConnectorLogoURL = (
  darkLogoUrl?: ConnectorSpec["darkLogoUrl"],
  lightLogoUrl?: ConnectorSpec["lightLogoUrl"],
) => {
  const { activeTheme } = useGalaxyTheme();
  const [state, setState] = useState<ConnectorLogoURLState>(DEFAULT_STATE);
  const themeLogoURL = match(activeTheme)
    .with(GalaxyTheme.DARK, () => darkLogoUrl)
    .with(GalaxyTheme.LIGHT, () => lightLogoUrl)
    .exhaustive();
  const logoURL = themeLogoURL && themeLogoURL !== state.failedLogoURL ? themeLogoURL : undefined;

  const handleLogoError = () => {
    setState((prev) => ({ ...prev, failedLogoURL: themeLogoURL }));
  };

  return { logoURL, handleLogoError };
};
