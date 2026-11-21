import type { FC } from "react";

import ToggleInput from "@galaxy-io/dls/inputs/ToggleInput";

import {
  CONNECTION_FORM_INGESTION_ORDER,
  CONNECTION_FORM_INGESTION_TO_LABEL_MAP,
} from "@/pages/connections/components/form/constants";
import type { ConnectionFormIngestion } from "@/pages/connections/components/form/types";

interface ConnectionFormIngestionToggleProps {
  value: ConnectionFormIngestion | null;
  isLocked: boolean;
  isDisabled: boolean;
  onChange: (ingestion: ConnectionFormIngestion) => void;
}

const ConnectionFormIngestionToggle: FC<ConnectionFormIngestionToggleProps> = ({
  value,
  isLocked,
  isDisabled,
  onChange,
}) => (
  <ToggleInput
    label="Read from"
    options={CONNECTION_FORM_INGESTION_ORDER.map((ingestion) => ({
      id: ingestion,
      label: CONNECTION_FORM_INGESTION_TO_LABEL_MAP[ingestion],
    }))}
    value={value}
    onChange={onChange}
    description={isLocked ? "This is fixed once the connection exists." : undefined}
    isReadOnly={isLocked}
    isDisabled={isDisabled}
    fillWidth
  />
);

export default ConnectionFormIngestionToggle;
