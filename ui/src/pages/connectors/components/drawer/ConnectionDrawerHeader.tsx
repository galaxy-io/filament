import { styled } from "@linaria/react";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
} from "@galaxy-io/dls/containers/FlexWrapper";

import BaseHeader from "@/layouts/components/BaseHeader";

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";
import { useConnectorSpec } from "@/pages/connectors/hooks";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

const HeaderWrapper = styled.div`
  padding: 12px 16px;
`;

interface ConnectionDrawerHeaderProps {
  connection: Connection;
  onClose: () => void;
}

const ConnectionDrawerHeader = ({ connection, onClose }: ConnectionDrawerHeaderProps) => {
  const connector = useConnectorSpec(connection.connector, connection.kind);

  return (
    <HeaderWrapper>
      <FlexWrapper fillWidth alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
        <FlexItem shrink={0}>
          <ConnectorTile
            connector={connection.connector}
            spec={connector}
            size={ConnectorTileSize.LARGE}
          />
        </FlexItem>
        <FlexWrapper fillWidth direction={FlexDirection.COLUMN} gap={FlexGap.XSMALL}>
          <BaseHeader
            title={connection.name}
            description={connector?.displayName || connection.connector}
            onClose={onClose}
          />
        </FlexWrapper>
      </FlexWrapper>
    </HeaderWrapper>
  );
};

export default ConnectionDrawerHeader;
