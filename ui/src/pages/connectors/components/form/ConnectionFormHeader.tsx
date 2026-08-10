import pluralize from "pluralize";

import { ChipSize } from "@galaxy-io/dls/chips/Chip";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import DocsButton from "@/components/DocsButton";

import BaseHeader, { BaseHeaderSize } from "@/layouts/components/BaseHeader";

import ConnectionKindChip from "@/pages/connectors/components/ConnectionKindChip";
import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";
import { CONNECTOR_KIND_TO_LABEL_MAP } from "@/pages/connectors/constants";

interface ConnectionFormHeaderProps {
  connectorName: string;
  connectorKind: ConnectorKind;
  title: string;
  onClose: () => void;
}

const createDocsPath = (connectorName: string, connectorKind: ConnectorKind) => {
  const kindSegment = pluralize(CONNECTOR_KIND_TO_LABEL_MAP[connectorKind]);
  return `/pages/connectors/${kindSegment}/${connectorName.toLowerCase()}`;
};

const ConnectionFormHeader = ({
  connectorName,
  connectorKind,
  title,
  onClose,
}: ConnectionFormHeaderProps) => {
  return (
    <FlexWrapper alignItems={AlignItems.CENTER} padding="12px 16px" gap={12} fillWidth>
      <FlexItem shrink={0}>
        <ConnectorTile
          connector={connectorName}
          kind={connectorKind}
          size={ConnectorTileSize.LARGE}
        />
      </FlexItem>
      <FlexItem grow={1} minWidth={0}>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={4} fillWidth>
          <BaseHeader
            size={BaseHeaderSize.LARGE}
            title={title}
            actions={[
              <DocsButton key="docs" path={createDocsPath(connectorName, connectorKind)} />,
            ]}
            onClose={onClose}
          />
          <ConnectionKindChip kind={connectorKind} size={ChipSize.SMALL} />
        </FlexWrapper>
      </FlexItem>
    </FlexWrapper>
  );
};

export default ConnectionFormHeader;
