import { useListConnectorsQuery } from "@/api/queries/connectors";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

export const useConnectorSpec = (name: string, kind?: ConnectorKind) => {
  const { data } = useListConnectorsQuery();

  return data?.connectors.find(
    (connector) => connector.name === name && (kind === undefined || connector.kind === kind),
  );
};
