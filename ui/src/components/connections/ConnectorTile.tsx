import type { FC, MouseEvent } from "react";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import ConnectorLogoTile from "@/components/connections/ConnectorLogoTile";
import type { ConnectorTileBadge, ConnectorTileSize } from "@/components/connections/types";
import { formatConnectorName } from "@/components/connections/utils";

import { createGetConnectorInput, useGetConnectorQuery } from "@/api/queries/connectors";

interface ConnectorTileProps {
  connector: ConnectorSpec["name"];
  kind?: ConnectorKind;
  size?: ConnectorTileSize;
  onClick?: (e: MouseEvent) => void;
  isDeleted?: boolean;
  badge?: ConnectorTileBadge;
}

const ConnectorTile: FC<ConnectorTileProps> = ({
  connector,
  kind,
  size,
  onClick,
  isDeleted,
  badge,
}) => {
  const { data, isLoading } = useGetConnectorQuery({
    input: createGetConnectorInput(connector, kind),
    options: { enabled: !!connector && !!kind },
  });
  const catalogSpec = data?.connector;

  return (
    <ConnectorLogoTile
      name={catalogSpec ? formatConnectorName(catalogSpec) : connector}
      darkLogoUrl={catalogSpec?.darkLogoUrl}
      lightLogoUrl={catalogSpec?.lightLogoUrl}
      size={size}
      onClick={onClick}
      isDeleted={isDeleted}
      isLoading={isLoading}
      badge={badge}
    />
  );
};

export default ConnectorTile;
