import { useMemo } from "react";

import { styled } from "@linaria/react";
import { useSearch } from "@tanstack/react-router";

import GridWrapper from "@galaxy-io/dls/containers/GridWrapper";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

import EmptyLayout, { EmptyLayoutSize } from "@/layouts/EmptyLayout";

import CreateConnectionSelectorCard from "@/pages/connectors/components/create/select/CreateConnectionSelectorCard";
import { CONNECTOR_KIND_PARAM_TO_KIND_MAP } from "@/pages/connectors/constants";

import { useListConnectorsQuery } from "@/api/queries/connectors";

import { isSearchMatch } from "@/utils/search";

const CREATE_CONNECTION_SELECTOR_GHOST_COUNT = 6;

interface CreateConnectionSelectorBodyProps {
  search: string;
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

const GhostCard = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px;
  border: 0.5px solid ${({ theme }) => theme.color.border.tertiary};
  border-radius: 8px;
  background-color: ${({ theme }) => theme.color.background.primary};
`);

const CreateConnectionSelectorBody = ({
  search,
  onConnectorSelect,
}: CreateConnectionSelectorBodyProps) => {
  const { connectorKind } = useSearch({ from: "__root__" });
  const kind = connectorKind
    ? CONNECTOR_KIND_PARAM_TO_KIND_MAP[connectorKind]
    : ConnectorKind.UNSPECIFIED;

  const { data, isLoading } = useListConnectorsQuery();

  const filteredConnectors = useMemo(
    () =>
      (data?.connectors ?? [])
        .filter(
          (connector) =>
            (kind === ConnectorKind.UNSPECIFIED || connector.kind === kind) &&
            isSearchMatch(search, connector.name, connector.displayName, connector.description),
        )
        .sort((a, b) => (a.displayName || a.name).localeCompare(b.displayName || b.name)),
    [data?.connectors, search, kind],
  );

  if (isLoading) {
    return (
      <BodyWrapper>
        <GridWrapper columns="repeat(3, 1fr)" gap={12}>
          {Array.from({ length: CREATE_CONNECTION_SELECTOR_GHOST_COUNT }, (_, index) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: static skeleton list
            <GhostCard key={index}>
              <TextShimmer height={24} width={24} />
              <TextShimmer height={16} width={100} />
            </GhostCard>
          ))}
        </GridWrapper>
      </BodyWrapper>
    );
  }

  if (filteredConnectors.length === 0) {
    return (
      <BodyWrapper>
        <EmptyLayout
          size={EmptyLayoutSize.SMALL}
          header="No connectors found"
          message={search ? "No connectors match your search." : "No connectors are available."}
        />
      </BodyWrapper>
    );
  }

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
