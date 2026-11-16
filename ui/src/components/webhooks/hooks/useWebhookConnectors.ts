import { useMemo } from "react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import { getConnectorFamily } from "@/components/connections/utils";
import { compareWebhookConnectors } from "@/components/webhooks/utils";

import { useListConnectorsQuery } from "@/api/queries/connectors";

export const useWebhookConnectors = () => {
  const { data, isLoading } = useListConnectorsQuery();

  const connectors = useMemo(() => {
    const catalog = data?.connectors ?? [];
    const families = new Map<ConnectorSpec["name"], ConnectorSpec>();
    for (const connector of catalog) {
      if (connector.kind !== ConnectorKind.SOURCE || !connector.webhook) continue;
      const family = getConnectorFamily(connector, catalog);
      families.set(family.name, family);
    }
    return [...families.values()].sort(compareWebhookConnectors);
  }, [data?.connectors]);

  return { connectors, isLoading };
};
