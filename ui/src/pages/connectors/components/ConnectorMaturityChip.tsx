import Chip, { type ChipSize } from "@galaxy-io/dls/chips/Chip";

import { ConnectorMaturity } from "@/gen/ingestion/v1/providers_pb";

import {
  CONNECTOR_MATURITY_TO_CHIP_VARIANT_MAP,
  CONNECTOR_MATURITY_TO_LABEL_MAP,
} from "@/pages/connectors/constants";

interface ConnectorMaturityChipProps {
  maturity: ConnectorMaturity;
  size?: ChipSize;
}

const ConnectorMaturityChip = ({ maturity, size }: ConnectorMaturityChipProps) => {
  if (maturity === ConnectorMaturity.UNSPECIFIED) {
    return null;
  }

  return (
    <Chip
      label={CONNECTOR_MATURITY_TO_LABEL_MAP[maturity]}
      variant={CONNECTOR_MATURITY_TO_CHIP_VARIANT_MAP[maturity]}
      size={size}
    />
  );
};

export default ConnectorMaturityChip;
