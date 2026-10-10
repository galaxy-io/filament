import { type FC, useMemo } from "react";

import { MagnifyingGlassIcon, PlusIcon } from "@phosphor-icons/react";
import { useNavigate, useSearch } from "@tanstack/react-router";
import pluralize from "pluralize";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import Grid from "@galaxy-io/dls/layout/Grid";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import DocsLink from "@/components/DocsLink";
import InfiniteScrollSentinel from "@/components/InfiniteScrollSentinel";

import EmptyLayout from "@/layouts/EmptyLayout";
import MainLayoutListPage from "@/layouts/main/MainLayoutListPage";

import ConnectionsPageSinksEmptyGraphic from "@/pages/connectors/components/ConnectionsPageSinksEmptyGraphic";
import ConnectionsPageSourcesEmptyGraphic from "@/pages/connectors/components/ConnectionsPageSourcesEmptyGraphic";
import ConnectionCard from "@/pages/connectors/components/card/ConnectionCard";
import {
  CONNECTOR_GRID_MIN_COLUMN_WIDTH,
  CONNECTOR_KIND_TO_DOCS_PATH_MAP,
  CONNECTOR_KIND_TO_EMPTY_MESSAGE_MAP,
  CONNECTOR_KIND_TO_LABEL_MAP,
} from "@/pages/connectors/constants";
import { usePipelineConnectionMap } from "@/pages/connectors/hooks/usePipelineConnectionMap";

import { Flow } from "@/module/types";

import {
  createListConnectionsInput,
  useSuspenseListConnectionsInfiniteQuery,
} from "@/api/queries/connections";

interface ConnectionsPageProps {
  kind: ConnectorKind.SOURCE | ConnectorKind.SINK;
}

const CONNECTOR_KIND_TO_EMPTY_GRAPHIC_MAP: Record<ConnectorKind.SOURCE | ConnectorKind.SINK, FC> = {
  [ConnectorKind.SOURCE]: ConnectionsPageSourcesEmptyGraphic,
  [ConnectorKind.SINK]: ConnectionsPageSinksEmptyGraphic,
};

const ConnectionsPage: FC<ConnectionsPageProps> = ({ kind }) => {
  const navigate = useNavigate();
  const { q = "" } = useSearch({ strict: false });

  const kindLabel = CONNECTOR_KIND_TO_LABEL_MAP[kind].toLowerCase();
  const kindPlural = pluralize(kindLabel);

  const handleOpenCreateConnectorModal = () => {
    void navigate({
      to: ".",
      search: (prev) => ({
        ...prev,
        connectionId: undefined,
        flow: Flow.CREATE_CONNECTION,
        connectorKind: kind,
      }),
    });
  };

  const { data, hasNextPage, isFetchingNextPage, fetchNextPage } =
    useSuspenseListConnectionsInfiniteQuery({ input: createListConnectionsInput(kind, { q }) });
  const kindConnections = useMemo(
    () => data.pages.flatMap((page) => page.connections),
    [data.pages],
  );

  const { connectionIdsByPipelineId } = usePipelineConnectionMap();
  const pipelineCountsByConnectionId = useMemo(() => {
    const counts = new Map<Connection["id"], number>();
    for (const connectionIds of connectionIdsByPipelineId.values()) {
      for (const connectionId of connectionIds) {
        counts.set(connectionId, (counts.get(connectionId) ?? 0) + 1);
      }
    }
    return counts;
  }, [connectionIdsByPipelineId]);

  const handleConnectionClick = (connectionId: Connection["id"]) => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, connectionId }),
    });
  };

  const renderContent = () => {
    if (!kindConnections.length && !q) {
      const EmptyGraphic = CONNECTOR_KIND_TO_EMPTY_GRAPHIC_MAP[kind];

      return (
        <EmptyLayout
          graphic={<EmptyGraphic />}
          header={`No ${kindPlural} found`}
          description={CONNECTOR_KIND_TO_EMPTY_MESSAGE_MAP[kind]}
          actions={
            <Flex alignItems={AlignItems.CENTER} gap={16}>
              <Button
                label={`New ${kindLabel}`}
                icon={PlusIcon}
                variant={ButtonVariant.PRIMARY}
                size={ButtonSize.LARGE}
                onClick={handleOpenCreateConnectorModal}
              />
              <DocsLink
                label={`Learn about ${kindPlural}`}
                path={CONNECTOR_KIND_TO_DOCS_PATH_MAP[kind]}
              />
            </Flex>
          }
        />
      );
    }

    if (!kindConnections.length) {
      return (
        <EmptyLayout icon={MagnifyingGlassIcon} header={`No ${kindPlural} match your search`} />
      );
    }

    return (
      <>
        <Grid
          columns={`repeat(auto-fill, minmax(${CONNECTOR_GRID_MIN_COLUMN_WIDTH}px, 1fr))`}
          gap={12}
        >
          {kindConnections.map((connection) => (
            <ConnectionCard
              key={connection.id}
              connection={connection}
              pipelineCount={pipelineCountsByConnectionId.get(connection.id) ?? 0}
              onClick={() => handleConnectionClick(connection.id)}
            />
          ))}
        </Grid>
        <InfiniteScrollSentinel
          hasNextPage={hasNextPage}
          isFetchingNextPage={isFetchingNextPage}
          fetchNextPage={fetchNextPage}
        />
      </>
    );
  };

  return (
    <MainLayoutListPage
      isScrollable={kindConnections.length > 0}
      actions={[
        <Button
          key="new-connector"
          label={`New ${kindLabel}`}
          icon={PlusIcon}
          variant={ButtonVariant.PRIMARY}
          onClick={handleOpenCreateConnectorModal}
        />,
      ]}
    >
      {renderContent()}
    </MainLayoutListPage>
  );
};

export default ConnectionsPage;
