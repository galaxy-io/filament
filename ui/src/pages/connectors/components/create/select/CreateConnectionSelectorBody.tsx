import { useMemo } from "react";

import { styled } from "@linaria/react";

import GridWrapper from "@galaxy-io/dls/containers/GridWrapper";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

import CreateConnectionSelectorCard from "@/pages/connectors/components/create/select/CreateConnectionSelectorCard";

import { useListConnectorsQuery } from "@/api/queries/connectors";

import { isSearchMatch } from "@/utils/search";

interface CreateConnectionSelectorBodyProps {
  search: string;
  connectorKind: ConnectorKind;
  onConnectorSelect: (connector: ConnectorSpec) => void;
}

const BodyWrapper = withTheme(styled.div<PropsWithTheme>`
  flex: 1;
  min-height: 0;
  width: 100%;
  overflow-y: auto;
  background-color: ${({ theme }) => theme.color.background.base};
  border-radius: 0 0 8px 8px;
  padding: 16px;
`);

const CreateConnectionSelectorBody = ({
  search,
  connectorKind,
  onConnectorSelect,
}: CreateConnectionSelectorBodyProps) => {
  const { data } = useListConnectorsQuery();

  const filteredConnectors = useMemo(
    () =>
      (data?.connectors ?? [])
        .filter(
          (connector) =>
            (connectorKind === ConnectorKind.UNSPECIFIED || connector.kind === connectorKind) &&
            isSearchMatch(search, connector.name, connector.displayName, connector.description),
        )
        .sort((a, b) => (a.displayName || a.name).localeCompare(b.displayName || b.name)),
    [data?.connectors, search, connectorKind],
  );

  return (
    <BodyWrapper>
      <GridWrapper columns="repeat(3, 1fr)" gap={12}>
        {filteredConnectors.map((connector) => (
          <CreateConnectionSelectorCard
            key={`${connector.name}-${connector.kind}`}
            connector={connector}
            onConnectorSelect={onConnectorSelect}
          />
        ))}
      </GridWrapper>
    </BodyWrapper>
  );
};

export default CreateConnectionSelectorBody;
