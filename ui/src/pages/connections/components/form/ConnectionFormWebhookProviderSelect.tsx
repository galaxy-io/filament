import type { FC } from "react";

import SelectInput from "@galaxy-io/dls/inputs/SelectInput";

import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import { formatConnectorName } from "@/components/connections/utils";
import { useWebhookConnectors } from "@/components/webhooks/hooks/useWebhookConnectors";

interface ConnectionFormWebhookProviderSelectProps {
  family: ConnectorSpec["name"] | undefined;
  onChange: (connector: ConnectorSpec["name"]) => void;
  isDisabled: boolean;
}

const ConnectionFormWebhookProviderSelect: FC<ConnectionFormWebhookProviderSelectProps> = ({
  family,
  onChange,
  isDisabled,
}) => {
  const { connectors, isLoading } = useWebhookConnectors();
  const options = connectors.map((connector) => ({
    id: connector.name,
    label: formatConnectorName(connector),
  }));

  const handleChange = (name: string | null) => {
    const connector = connectors.find((candidate) => candidate.name === name);
    if (!connector || connector.name === family) return;
    onChange(connector.aliasTarget || connector.name);
  };

  return (
    <SelectInput
      label="Provider"
      options={options}
      value={family ?? null}
      onChange={handleChange}
      isDisabled={isDisabled || isLoading}
      fillWidth
    />
  );
};

export default ConnectionFormWebhookProviderSelect;
