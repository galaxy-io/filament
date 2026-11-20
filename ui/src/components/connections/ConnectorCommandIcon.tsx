import type { FC } from "react";

import type { Transport } from "@connectrpc/connect";
import { styled } from "@linaria/react";
import type { Icon as PhosphorIcon } from "@phosphor-icons/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import { CONNECTOR_COMMAND_ICON_SIZE } from "@/components/connections/constants";
import { useConnectorLogoURL } from "@/components/connections/hooks/useConnectorLogoURL";

import { createGetConnectorInput, useGetConnectorQuery } from "@/api/queries/connectors";

const ConnectorCommandLogo = styled.img`
  display: block;
  flex-shrink: 0;
  width: ${CONNECTOR_COMMAND_ICON_SIZE}px;
  height: ${CONNECTOR_COMMAND_ICON_SIZE}px;
  object-fit: contain;
`;

interface ConnectorCommandIconProps {
  connector: ConnectorSpec["name"];
  kind: ConnectorKind;
  fallbackIcon: PhosphorIcon;
  transport?: Transport;
}

const ConnectorCommandIcon: FC<ConnectorCommandIconProps> = ({
  connector,
  kind,
  fallbackIcon,
  transport,
}) => {
  const { data } = useGetConnectorQuery({
    input: createGetConnectorInput(connector, kind),
    options: { enabled: !!connector, transport },
  });
  const { logoURL, handleLogoError } = useConnectorLogoURL(
    data?.connector?.darkLogoUrl,
    data?.connector?.lightLogoUrl,
  );

  if (!logoURL) {
    return (
      <Icon
        component={fallbackIcon}
        size={CONNECTOR_COMMAND_ICON_SIZE}
        variant={IconVariant.SECONDARY}
        aria-hidden
      />
    );
  }

  return <ConnectorCommandLogo src={logoURL} alt="" onError={handleLogoError} />;
};

export default ConnectorCommandIcon;
