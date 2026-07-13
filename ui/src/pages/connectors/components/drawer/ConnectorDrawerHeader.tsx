import { styled } from "@linaria/react";

import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
} from "@galaxy-io/dls/containers/FlexWrapper";

import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

import BaseHeader from "@/layouts/components/BaseHeader";

import ConnectorTile, {
  ConnectorTileSize,
} from "@/pages/connectors/components/ConnectorTile";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";

const HeaderWrapper = styled.div`
  padding: 12px 16px;
`;

interface ConnectorDrawerHeaderProps {
  connector: ConnectorSpec;
  onClose: () => void;
}

const ConnectorDrawerHeader = ({
  connector,
  onClose,
}: ConnectorDrawerHeaderProps) => {
  return (
    <HeaderWrapper>
      <FlexWrapper
        fillWidth
        alignItems={AlignItems.CENTER}
        gap={FlexGap.MEDIUM}
      >
        <FlexItem shrink={0}>
          <ConnectorTile
            connector={connector.name}
            size={ConnectorTileSize.LARGE}
          />
        </FlexItem>
        <FlexWrapper
          fillWidth
          direction={FlexDirection.COLUMN}
          gap={FlexGap.XSMALL}
        >
          <BaseHeader
            title={connector.displayName}
            description={connector.name}
            onClose={onClose}
          />
        </FlexWrapper>
      </FlexWrapper>
    </HeaderWrapper>
  );
};

export default ConnectorDrawerHeader;
