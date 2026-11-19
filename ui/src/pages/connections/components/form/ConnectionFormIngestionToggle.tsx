import type { FC } from "react";

import ToggleInput from "@galaxy-io/dls/inputs/ToggleInput";

import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import { formatConnectorName } from "@/components/connections/utils";

import {
  CONNECTION_FORM_INGESTION_ORDER,
  CONNECTION_FORM_INGESTION_TO_LABEL_MAP,
  CONNECTION_FORM_INGESTION_TO_TOOLTIP_MAP,
} from "@/pages/connections/components/form/constants";
import type { ConnectionFormIngestion } from "@/pages/connections/components/form/types";

interface ConnectionFormIngestionToggleProps {
  connector: ConnectorSpec;
  value: ConnectionFormIngestion | null;
  isLocked: boolean;
  isDisabled: boolean;
  onChange: (ingestion: ConnectionFormIngestion) => void;
}

const ConnectionFormIngestionToggle: FC<ConnectionFormIngestionToggleProps> = ({
  connector,
  value,
  isLocked,
  isDisabled,
  onChange,
}) => {
  const connectorName = formatConnectorName(connector);

  return (
    <ToggleInput
      label="Read from"
      options={CONNECTION_FORM_INGESTION_ORDER.map((ingestion) => ({
        id: ingestion,
        label: CONNECTION_FORM_INGESTION_TO_LABEL_MAP[ingestion],
        tooltip: CONNECTION_FORM_INGESTION_TO_TOOLTIP_MAP[ingestion](connectorName),
      }))}
      value={value}
      onChange={onChange}
      description={isLocked ? "This is fixed once the connection exists." : undefined}
      isReadOnly={isLocked}
      isDisabled={isDisabled}
      fillWidth
    />
  );
};

export default ConnectionFormIngestionToggle;
