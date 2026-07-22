import Chip, { ChipSize } from "@galaxy-io/dls/chips/Chip";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";

import BaseHeader from "@/layouts/components/BaseHeader";
import { BaseHeaderSize } from "@/layouts/components/types";

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";
import {
  CONNECTOR_KIND_TO_CHIP_VARIANT_MAP,
  CONNECTOR_KIND_TO_LABEL_MAP,
} from "@/pages/connectors/constants";

import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

interface CreateConnectionConfigureHeaderProps {
  connector: ConnectorSpec;
  onClose: () => void;
}

const CreateConnectionConfigureHeader = ({
  connector,
  onClose,
}: CreateConnectionConfigureHeaderProps) => {
  return (
    <FlexWrapper alignItems={AlignItems.CENTER} padding="16px" gap={12} fillWidth>
      <FlexItem shrink={0}>
        <ConnectorTile
          connector={connector.name}
          spec={connector}
          size={ConnectorTileSize.MEDIUM}
        />
      </FlexItem>
      {/* minWidth 0 lets the column shrink so the title ellipsizes instead of
          pushing the close button out of view */}
      <FlexWrapper
        direction={FlexDirection.COLUMN}
        alignItems={AlignItems.START}
        gap={4}
        grow={1}
        minWidth={0}
      >
        <BaseHeader
          size={BaseHeaderSize.LARGE}
          title={`New ${connector.displayName || connector.name} connection`}
          onClose={onClose}
        />
        <Chip
          label={CONNECTOR_KIND_TO_LABEL_MAP[connector.kind]}
          variant={CONNECTOR_KIND_TO_CHIP_VARIANT_MAP[connector.kind]}
          size={ChipSize.SMALL}
        />
      </FlexWrapper>
    </FlexWrapper>
  );
};

export default CreateConnectionConfigureHeader;
