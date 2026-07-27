import { ChipSize } from "@galaxy-io/dls/chips/Chip";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";

import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

import BaseHeader, { BaseHeaderSize } from "@/layouts/components/BaseHeader";

import ConnectionKindChip from "@/pages/connectors/components/ConnectionKindChip";
import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";

interface CreateConnectionConfigureHeaderProps {
  connector: ConnectorSpec;
  onClose: () => void;
}

const CreateConnectionConfigureHeader = ({
  connector,
  onClose,
}: CreateConnectionConfigureHeaderProps) => {
  return (
    <FlexWrapper alignItems={AlignItems.CENTER} padding="12px 16px" gap={12} fillWidth>
      <FlexItem shrink={0}>
        <ConnectorTile
          connector={connector.name}
          spec={connector}
          size={ConnectorTileSize.MEDIUM}
        />
      </FlexItem>
      <FlexItem grow={1} minWidth={0}>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={4} fillWidth>
          <BaseHeader
            size={BaseHeaderSize.LARGE}
            title={`New ${connector.displayName || connector.name} connection`}
            onClose={onClose}
          />
          <ConnectionKindChip kind={connector.kind} size={ChipSize.SMALL} />
        </FlexWrapper>
      </FlexItem>
    </FlexWrapper>
  );
};

export default CreateConnectionConfigureHeader;
