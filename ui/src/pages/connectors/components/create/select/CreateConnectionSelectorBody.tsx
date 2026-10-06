import { useMemo } from "react";

import { styled } from "@linaria/react";
import { useSearch } from "@tanstack/react-router";

import Skeleton from "@galaxy-io/dls/feedback/Skeleton";
import Box from "@galaxy-io/dls/layout/Box";
import Grid from "@galaxy-io/dls/layout/Grid";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import CreateConnectionSelectorCard, {
  CreateConnectionSelectorEmptyCard,
} from "@/pages/connectors/components/create/select/CreateConnectionSelectorCard";
import { getConnectorFamily } from "@/pages/connectors/components/create/utils";
import {
  CREATE_CONNECTION_SELECTOR_GHOST_COUNT,
  CREATE_CONNECTION_SELECTOR_GRID_COLUMNS,
} from "@/pages/connectors/constants";

import { useListConnectorsQuery } from "@/api/queries/connectors";
import { MAX_LIST_SEARCH_LENGTH } from "@/api/utils";

interface CreateConnectionSelectorBodyProps {
  onConnectorSelect: (connector: ConnectorSpec) => void;
}

const BodyWrapper = styled.div`
  flex: 1;
  min-height: 0;
  width: 100%;
  overflow-y: auto;
  background-color: ${t.color.background.base};
  border-radius: 0 0 8px 8px;
  padding: 16px;
`;

const GhostCard = styled.div`
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px;
  border: 0.5px solid ${t.color.border.tertiary};
  border-radius: 8px;
  background-color: ${t.color.background.primary};
`;

const CreateConnectionSelectorBody = ({ onConnectorSelect }: CreateConnectionSelectorBodyProps) => {
  const { connectorKind, connectorSearch = "" } = useSearch({ from: "/_app" });
  const kind = connectorKind ?? ConnectorKind.UNSPECIFIED;

  const { data, isLoading } = useListConnectorsQuery();

  const filteredConnectors = useMemo(() => {
    const catalog = data?.connectors ?? [];
    const search = connectorSearch.trim().slice(0, MAX_LIST_SEARCH_LENGTH).toLowerCase();
    const families = new Map<string, ConnectorSpec>();
    for (const connector of catalog) {
      if (kind !== ConnectorKind.UNSPECIFIED && connector.kind !== kind) continue;
      if (
        search &&
        ![connector.name, connector.displayName, connector.description].some((value) =>
          value.toLowerCase().includes(search),
        )
      )
        continue;
      const family = getConnectorFamily(connector, catalog);
      families.set(`${family.kind}:${family.name}`, family);
    }
    return [...families.values()].sort((a, b) =>
      (a.displayName || a.name).localeCompare(b.displayName || b.name),
    );
  }, [data?.connectors, kind, connectorSearch]);

  if (isLoading) {
    return (
      <BodyWrapper>
        <Grid columns={CREATE_CONNECTION_SELECTOR_GRID_COLUMNS} gap={12}>
          {Array.from({ length: CREATE_CONNECTION_SELECTOR_GHOST_COUNT }, (_, index) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: static skeleton list
            <GhostCard key={index}>
              <Box width={24}>
                <Skeleton /* @dls-migrate skeleton.TextShimmer.height-other: Pick a rung, or wrap the real `Text` in `<Skeleton isLoading>` (wrapper mode). */
                  height={24}
                />
              </Box>
              <Box width={100}>
                <Skeleton />
              </Box>
            </GhostCard>
          ))}
        </Grid>
      </BodyWrapper>
    );
  }

  return (
    <BodyWrapper>
      <Grid columns={CREATE_CONNECTION_SELECTOR_GRID_COLUMNS} gap={12}>
        {filteredConnectors.map((connector) => (
          <CreateConnectionSelectorCard
            key={`${connector.name}-${connector.kind}`}
            connector={connector}
            onConnectorSelect={onConnectorSelect}
          />
        ))}
        <CreateConnectionSelectorEmptyCard />
      </Grid>
    </BodyWrapper>
  );
};

export default CreateConnectionSelectorBody;
