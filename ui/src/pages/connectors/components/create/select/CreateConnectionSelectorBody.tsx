import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";

import GridWrapper from "@galaxy-io/dls/containers/GridWrapper";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import CreateConnectionSelectorCard from "@/pages/connectors/components/create/select/CreateConnectionSelectorCard";

import { useListConnectorsQuery } from "@/api/queries/connectors";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { type ConnectorSpec, ListConnectorsRequestSchema } from "@/gen/ingestion/v1/providers_pb";

interface CreateConnectionSelectorBodyProps {
  search: string;
  connectorKindFilter: ConnectorKind;
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
  connectorKindFilter,
  onConnectorSelect,
}: CreateConnectionSelectorBodyProps) => {
  const { data } = useListConnectorsQuery({
    input: create(ListConnectorsRequestSchema, { kind: connectorKindFilter }),
  });

  const filteredConnectors = useMemo(() => {
    if (!data?.connectors) return [];

    const searchLower = search.toLowerCase();

    return data.connectors
      .filter((connector) => {
        const matchesSearch = search
          ? connector.name.toLowerCase().includes(searchLower) ||
            connector.displayName.toLowerCase().includes(searchLower) ||
            connector.description.toLowerCase().includes(searchLower)
          : true;

        const matchesKind =
          connectorKindFilter === ConnectorKind.UNSPECIFIED ||
          connector.kind === connectorKindFilter;

        return matchesSearch && matchesKind;
      })
      .sort((a, b) => {
        const labelA = a.displayName || a.name;
        const labelB = b.displayName || b.name;
        return labelA.localeCompare(labelB);
      });
  }, [data?.connectors, search, connectorKindFilter]);

  return (
    <BodyWrapper>
      <GridWrapper columns="repeat(3, 1fr)" gap={12}>
        {filteredConnectors.map((connector) => (
          <CreateConnectionSelectorCard
            key={connector.name}
            connector={connector}
            onConnectorSelect={onConnectorSelect}
          />
        ))}
      </GridWrapper>
    </BodyWrapper>
  );
};

export default CreateConnectionSelectorBody;
