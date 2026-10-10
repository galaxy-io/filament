import { useMemo } from "react";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import { getConnectorFamily, mapConnectorsToShuffled } from "@/components/connections/utils";

import { useListConnectorsQuery } from "@/api/queries/connectors";

export const useConnectorFamilies = (kind: ConnectorKind) => {
  const { data } = useListConnectorsQuery();

  return useMemo(() => {
    const catalog = data?.connectors ?? [];
    const families = new Map<ConnectorSpec["name"], ConnectorSpec>();
    for (const connector of catalog) {
      if (connector.kind !== kind) continue;
      const family = getConnectorFamily(connector, catalog);
      families.set(family.name, family);
    }
    return mapConnectorsToShuffled([...families.values()]);
  }, [data?.connectors, kind]);
};
