import type { FC } from "react";

import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import BaseHeader, { BaseHeaderSize } from "@/layouts/components/BaseHeader";

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";

interface ConnectionFormHeaderProps {
  connectorName: ConnectorSpec["name"];
  connectorKind: ConnectorKind;
  title: string;
}

const ConnectionFormHeader: FC<ConnectionFormHeaderProps> = ({
  connectorName,
  connectorKind,
  title,
}) => {
  return (
    <Flex alignItems={AlignItems.CENTER} gap={12} fillWidth>
      <FlexItem shrink={0}>
        <ConnectorTile
          connector={connectorName}
          kind={connectorKind}
          size={ConnectorTileSize.LARGE}
        />
      </FlexItem>
      <FlexItem grow={1} minWidth={0}>
        <BaseHeader size={BaseHeaderSize.LARGE} title={title} />
      </FlexItem>
    </Flex>
  );
};

export default ConnectionFormHeader;
