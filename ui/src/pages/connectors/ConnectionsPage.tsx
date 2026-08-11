import { type ReactElement, useMemo } from "react";

import { MagnifyingGlassIcon, PlusIcon } from "@phosphor-icons/react";
import { useNavigate, useSearch } from "@tanstack/react-router";
import pluralize from "pluralize";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper from "@galaxy-io/dls/containers/FlexWrapper";
import GridWrapper from "@galaxy-io/dls/containers/GridWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import DocsButton from "@/components/DocsButton";
import InfiniteScrollSentinel from "@/components/InfiniteScrollSentinel";

import EmptyLayout from "@/layouts/EmptyLayout";
import MainLayoutListPage from "@/layouts/main/MainLayoutListPage";

import ConnectionsPageSinksEmptyGraphic from "@/pages/connectors/components/ConnectionsPageSinksEmptyGraphic";
import ConnectionsPageSourcesEmptyGraphic from "@/pages/connectors/components/ConnectionsPageSourcesEmptyGraphic";
import ConnectionCard from "@/pages/connectors/components/card/ConnectionCard";
import {
  CONNECTOR_GRID_MIN_COLUMN_WIDTH,
  CONNECTOR_KIND_TO_EMPTY_MESSAGE_MAP,
  CONNECTOR_KIND_TO_LABEL_MAP,
} from "@/pages/connectors/constants";
import { usePipelineConnectionMap } from "@/pages/connectors/hooks/usePipelineConnectionMap";

import { Flow } from "@/routes/__root";

import { useSuspenseListConnectionsInfiniteQuery } from "@/api/queries/connections";

import { isSearchMatch } from "@/utils/search";

interface ConnectionsPageProps {
  kind: ConnectorKind.SOURCE | ConnectorKind.SINK;
}

const CONNECTOR_KIND_TO_EMPTY_GRAPHIC_MAP: Record<
  ConnectorKind.SOURCE | ConnectorKind.SINK,
  () => ReactElement
> = {
  [ConnectorKind.SOURCE]: ConnectionsPageSourcesEmptyGraphic,
  [ConnectorKind.SINK]: ConnectionsPageSinksEmptyGraphic,
};

const ConnectionsPage = ({ kind }: ConnectionsPageProps) => {
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
    useSuspenseListConnectionsInfiniteQuery({ input: { kind } });
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

  const filteredConnections = useMemo(
    () => kindConnections.filter((connection) => isSearchMatch(q, connection.name)),
    [kindConnections, q],
  );

  const handleConnectionClick = (connectionId: Connection["id"]) => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, connectionId }),
    });
  };

  const renderContent = () => {
    if (!kindConnections.length) {
      const EmptyGraphic = CONNECTOR_KIND_TO_EMPTY_GRAPHIC_MAP[kind];

      return (
        <EmptyLayout
          icon={<EmptyGraphic />}
          header={`No ${kindPlural} found`}
          message={CONNECTOR_KIND_TO_EMPTY_MESSAGE_MAP[kind]}
          actions={
            <FlexWrapper gap={8}>
              <Button
                label={`New ${kindLabel}`}
                icon={PlusIcon}
                variant={ButtonVariant.PRIMARY}
                size={ButtonSize.LARGE}
                onClick={handleOpenCreateConnectorModal}
              />
              <DocsButton
                label="Read the docs"
                path={`/pages/connectors/${kindPlural}`}
                variant={ButtonVariant.SECONDARY}
                size={ButtonSize.LARGE}
              />
            </FlexWrapper>
          }
        />
      );
    }

    if (!filteredConnections.length) {
      return (
        <EmptyLayout
          icon={<Icon component={MagnifyingGlassIcon} variant={IconVariant.TERTIARY} />}
          message={`No ${kindPlural} match your search`}
        />
      );
    }

    return (
      <>
        <GridWrapper
          columns={`repeat(auto-fill, minmax(${CONNECTOR_GRID_MIN_COLUMN_WIDTH}px, 1fr))`}
          gap={12}
        >
          {filteredConnections.map((connection) => (
            <ConnectionCard
              key={connection.id}
              connection={connection}
              pipelineCount={pipelineCountsByConnectionId.get(connection.id) ?? 0}
              onClick={() => handleConnectionClick(connection.id)}
            />
          ))}
        </GridWrapper>
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
