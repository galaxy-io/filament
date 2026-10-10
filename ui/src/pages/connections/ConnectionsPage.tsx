import { type FC, type RefObject, useMemo } from "react";

import { MagnifyingGlassIcon, PlusIcon } from "@phosphor-icons/react";
import pluralize from "pluralize";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { useEndReached } from "@galaxy-io/dls/hooks/useEndReached";
import EmptyLayout from "@galaxy-io/dls/layout/EmptyLayout";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import Grid from "@galaxy-io/dls/layout/Grid";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { CONNECTOR_KIND_TO_LABEL_MAP } from "@/components/connections/constants";
import DocsLink from "@/components/DocsLink";

import MainLayoutListPage from "@/layouts/main/MainLayoutListPage";

import ConnectionsPageSinksEmptyGraphic from "@/pages/connections/components/ConnectionsPageSinksEmptyGraphic";
import ConnectionsPageSourcesEmptyGraphic from "@/pages/connections/components/ConnectionsPageSourcesEmptyGraphic";
import ConnectionCard from "@/pages/connections/components/card/ConnectionCard";
import {
  CONNECTOR_GRID_MIN_COLUMN_WIDTH,
  CONNECTOR_KIND_TO_DOCS_PATH_MAP,
  CONNECTOR_KIND_TO_EMPTY_MESSAGE_MAP,
} from "@/pages/connections/constants";
import { usePipelineConnectionMap } from "@/pages/connections/hooks/usePipelineConnectionMap";

import { useConnectionsSearch, useFilamentFlowOpen, useFilamentSearchUpdate } from "@/module/hooks";
import type { FilamentLayoutSearch } from "@/module/schemas";
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
  const updateSearch = useFilamentSearchUpdate<FilamentLayoutSearch>();
  const openFlow = useFilamentFlowOpen();
  const { q = "" } = useConnectionsSearch();

  const kindLabel = CONNECTOR_KIND_TO_LABEL_MAP[kind].toLowerCase();
  const kindPlural = pluralize(kindLabel);

  const handleOpenCreateConnectorModal = () => {
    openFlow(Flow.CREATE_CONNECTION, kind);
  };

  const { data, hasNextPage, isFetchingNextPage, fetchNextPage } =
    useSuspenseListConnectionsInfiniteQuery({ input: createListConnectionsInput(kind, { q }) });
  const kindConnections = useMemo(
    () => data.pages.flatMap((page) => page.connections),
    [data.pages],
  );
  const endRef = useEndReached({
    onEndReached: hasNextPage ? () => void fetchNextPage() : undefined,
    isEnabled: !isFetchingNextPage,
    itemCount: kindConnections.length,
  });

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
    void updateSearch((prev) => ({ ...prev, connectionId }));
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
        <div ref={endRef as RefObject<HTMLDivElement>} />
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
