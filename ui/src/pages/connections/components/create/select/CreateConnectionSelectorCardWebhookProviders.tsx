import type { FC, MouseEvent } from "react";

import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";

import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import ConnectorLogoTile from "@/components/connections/ConnectorLogoTile";
import ConnectorOverflowTile from "@/components/connections/ConnectorOverflowTile";
import { ConnectorTileSize } from "@/components/connections/types";
import { formatConnectorName } from "@/components/connections/utils";
import { useWebhookConnectors } from "@/components/webhooks/hooks/useWebhookConnectors";
import { isWebhookProviderConnector } from "@/components/webhooks/utils";

import { CREATE_CONNECTION_SELECTOR_WEBHOOK_PROVIDER_LIMIT } from "@/pages/connections/constants";

interface CreateConnectionSelectorCardWebhookProvidersProps {
  onProviderSelect: (connector: ConnectorSpec) => void;
}

const CreateConnectionSelectorCardWebhookProviders: FC<
  CreateConnectionSelectorCardWebhookProvidersProps
> = ({ onProviderSelect }) => {
  const { connectors } = useWebhookConnectors();
  const providers = connectors.filter(isWebhookProviderConnector);
  const shownProviders = providers.slice(0, CREATE_CONNECTION_SELECTOR_WEBHOOK_PROVIDER_LIMIT);
  const hiddenCount = providers.length - shownProviders.length;

  if (providers.length === 0) {
    return null;
  }

  return (
    <Flex
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.SPACE_BETWEEN}
      gap={8}
      fillWidth
    >
      <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
        Supported providers
      </Text>
      <Flex alignItems={AlignItems.CENTER} gap={4}>
        {shownProviders.map((provider) => (
          <Tooltip key={provider.name} body={`${formatConnectorName(provider)} webhooks`}>
            <ConnectorLogoTile
              name={`${formatConnectorName(provider)} webhooks`}
              darkLogoUrl={provider.darkLogoUrl}
              lightLogoUrl={provider.lightLogoUrl}
              size={ConnectorTileSize.MEDIUM}
              onClick={(event: MouseEvent) => {
                event.stopPropagation();
                onProviderSelect(provider);
              }}
            />
          </Tooltip>
        ))}
        {hiddenCount > 0 && <ConnectorOverflowTile count={hiddenCount} />}
      </Flex>
    </Flex>
  );
};

export default CreateConnectionSelectorCardWebhookProviders;
