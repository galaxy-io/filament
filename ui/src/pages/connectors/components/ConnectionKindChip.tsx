import Chip, { type ChipSize } from "@galaxy-io/dls/chips/Chip";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import {
  CONNECTOR_KIND_TO_CHIP_VARIANT_MAP,
  CONNECTOR_KIND_TO_LABEL_MAP,
} from "@/pages/connectors/constants";

interface ConnectionKindChipProps {
  kind: ConnectorKind;
  size?: ChipSize;
}

const ConnectionKindChip = ({ kind, size }: ConnectionKindChipProps) => {
  return (
    <Chip
      label={CONNECTOR_KIND_TO_LABEL_MAP[kind]}
      variant={CONNECTOR_KIND_TO_CHIP_VARIANT_MAP[kind]}
      size={size}
    />
  );
};

export default ConnectionKindChip;
