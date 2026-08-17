import Icon, { IconWeight } from "@galaxy-io/dls/icons/Icon";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";

import type { ConnectorMaturity } from "@/gen/ingestion/v1/connectors_pb";

import {
  CONNECTOR_MATURITY_TO_ICON_MAP,
  CONNECTOR_MATURITY_TO_ICON_VARIANT_MAP,
  CONNECTOR_MATURITY_TO_LABEL_MAP,
} from "@/pages/connectors/constants";

interface ConnectionKindChipProps {
  maturity: ConnectorMaturity;
  size?: number;
}

const ConnectionKindChip = ({ maturity, size = 14 }: ConnectionKindChipProps) => {
  return (
    <Tooltip body={CONNECTOR_MATURITY_TO_LABEL_MAP[maturity]}>
      <Icon
        component={CONNECTOR_MATURITY_TO_ICON_MAP[maturity]}
        variant={CONNECTOR_MATURITY_TO_ICON_VARIANT_MAP[maturity]}
        weight={IconWeight.FILL}
        size={size}
      />
    </Tooltip>
  );
};

export default ConnectionKindChip;
