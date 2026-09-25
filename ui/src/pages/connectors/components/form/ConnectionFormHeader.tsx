import pluralize from "pluralize";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorMaturity, ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import DocsButton from "@/components/DocsButton";

import BaseHeader, { BaseHeaderSize } from "@/layouts/components/BaseHeader";

import ConnectionKindChip from "@/pages/connectors/components/ConnectionKindChip";
import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";
import { CONNECTOR_KIND_TO_LABEL_MAP } from "@/pages/connectors/constants";

import ConnectorMaturityIcon from "../ConnectorMaturityIcon";

interface ConnectionFormHeaderProps {
  connectorName: ConnectorSpec["name"];
  connectorKind: ConnectorKind;
  connectorMaturity: ConnectorMaturity;
  connectorApiVersion: ConnectorSpec["apiVersion"];
  title: string;
  onClose: () => void;
}

const createDocsPath = (connectorName: ConnectorSpec["name"], connectorKind: ConnectorKind) => {
  const kindSegment = pluralize(CONNECTOR_KIND_TO_LABEL_MAP[connectorKind]).toLowerCase();
  return `/pages/connectors/${kindSegment}/${connectorName.toLowerCase()}`;
};

const ConnectionFormHeader = ({
  connectorName,
  connectorKind,
  connectorMaturity,
  connectorApiVersion,
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
          <FlexWrapper alignItems={AlignItems.CENTER} gap={8}>
            <ConnectionKindChip kind={connectorKind} size={ChipSize.SMALL} />
            {connectorApiVersion && (
              <Chip
                label={`Version ${connectorApiVersion}`}
                variant={ChipVariant.SECONDARY}
                size={ChipSize.SMALL}
              />
            )}
            <ConnectorMaturityIcon maturity={connectorMaturity} />
          </FlexWrapper>
        </FlexWrapper>
      </FlexItem>
    </FlexWrapper>
  );
};

export default ConnectionFormHeader;
