import { useMemo, useState } from "react";

import { create } from "@bufbuild/protobuf";
import { createQueryOptions, useTransport } from "@connectrpc/connect-query";
import { styled } from "@linaria/react";
import {
  BookOpenIcon,
  MagnifyingGlassIcon,
  PlusIcon,
  WarningCircleIcon,
} from "@phosphor-icons/react";
import { useQueries } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import GridWrapper from "@galaxy-io/dls/containers/GridWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import ConnectorsEmptyDark from "@/assets/components/ConnectorsEmptyDark";

import BaseToolbar from "@/layouts/components/BaseToolbar";
import EmptyLayout from "@/layouts/EmptyLayout";
import ErrorLayout from "@/layouts/ErrorLayout";

import ConnectionCard from "@/pages/connectors/components/card/ConnectionCard";
import ConnectionCardLoading from "@/pages/connectors/components/card/ConnectionCardLoading";
import { CONNECTOR_GRID_MIN_COLUMN_WIDTH } from "@/pages/connectors/constants";
import type { ConnectionsPageState } from "@/pages/connectors/types";

import { Flow } from "@/routes/__root";

import { useListConnectionsQuery } from "@/api/queries/connectors";
import { useListPipelinesQuery } from "@/api/queries/pipelines";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { ListConnectionsRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import { GetPipelineVersionRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";
import { IngestionService } from "@/gen/ingestion/v1/service_pb";

import { CONNECTORS_DOCS_URL } from "@/constants";

const LOADING_CARD_COUNT = 6;

const KIND_TO_COPY: Record<
  ConnectorKind.SOURCE | ConnectorKind.SINK,
  {
    plural: string;
    searchPlaceholder: string;
    newLabel: string;
    errorMessage: string;
    emptyHeader: string;
    emptyMessage: string;
  }
> = {
  [ConnectorKind.SOURCE]: {
    plural: "sources",
    searchPlaceholder: "Search sources",
    newLabel: "New source",
    errorMessage: "Failed to load sources. Please try again.",
    emptyHeader: "No sources found",
    emptyMessage: "Connect data sources to move data in.",
  },
  [ConnectorKind.SINK]: {
    plural: "sinks",
    searchPlaceholder: "Search sinks",
    newLabel: "New sink",
    errorMessage: "Failed to load sinks. Please try again.",
    emptyHeader: "No sinks found",
    emptyMessage: "Connect data sinks to move data out.",
  },
};

const ConnectorListScrollArea = styled.div`
  flex: 1;
  width: 100%;
  min-height: 0;

  padding: 12px;

  overflow-y: auto;
`;

const DEFAULT_STATE: ConnectionsPageState = {
  search: "",
};

interface ConnectionsPageProps {
  kind: ConnectorKind.SOURCE | ConnectorKind.SINK;
}

const ConnectionsPage = ({ kind }: ConnectionsPageProps) => {
  const navigate = useNavigate();

  const copy = KIND_TO_COPY[kind];

  const [state, setState] = useState<ConnectionsPageState>(DEFAULT_STATE);

  const handleOpenCreateConnectorModal = () => {
    void navigate({
      to: ".",
      search: { flow: Flow.CREATE_CONNECTION, connectorKind: kind },
    });
  };

  const handleSearchChange = (value: string) => {
    setState((prev) => ({ ...prev, search: value }));
  };

  const { data, isLoading, isError } = useListConnectionsQuery({
    input: create(ListConnectionsRequestSchema, { kind }),
  });

  // Each connection's pipeline count comes from the current graph version of
  // every pipeline; NotFound (no saved version yet) contributes nothing
  const transport = useTransport();
  const { data: pipelinesData } = useListPipelinesQuery();
  const versionResults = useQueries({
    queries: (pipelinesData?.pipelines ?? []).map((pipeline) => ({
      ...createQueryOptions(
        IngestionService.method.getPipelineVersion,
        create(GetPipelineVersionRequestSchema, { pipelineId: pipeline.id }),
        { transport },
      ),
      retry: false,
    })),
  });
  const pipelineCountsByConnectionId = useMemo(() => {
    const counts = new Map<string, number>();
    for (const result of versionResults) {
      // A pipeline counts once per connection even if two nodes share it
      const connectionIds = new Set(
        (result.data?.version?.nodes ?? []).map((node) => node.connectionId),
      );
      for (const connectionId of connectionIds) {
        counts.set(connectionId, (counts.get(connectionId) ?? 0) + 1);
      }
    }
    return counts;
  }, [versionResults]);

  const isToolbarDisabled = isLoading || isError;

  const filteredConnections = useMemo(() => {
    if (!data?.connections) return [];

    return data.connections.filter((connection) =>
      state.search ? connection.name.toLowerCase().includes(state.search.toLowerCase()) : true,
    );
  }, [data?.connections, state.search]);

  const handleConnectionClick = (connectionId: string) => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, connectionId }),
    });
  };

  const handleReadTheDocs = () => {
    window.open(CONNECTORS_DOCS_URL, "_blank", "noopener,noreferrer");
  };

  const renderContent = () => {
    if (isLoading) {
      return (
        <GridWrapper
          columns={`repeat(auto-fill, minmax(${CONNECTOR_GRID_MIN_COLUMN_WIDTH}px, 1fr))`}
          gap={12}
        >
          {Array.from({ length: LOADING_CARD_COUNT }).map((_, index) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: fixed-length loading skeleton, never reordered
            <ConnectionCardLoading key={index} />
          ))}
        </GridWrapper>
      );
    }

    if (isError) {
      return (
        <ErrorLayout
          icon={<Icon component={WarningCircleIcon} size={20} variant={IconVariant.ERROR} />}
          message={copy.errorMessage}
        />
      );
    }

    const hasConnections = (data?.connections?.length ?? 0) > 0;

    if (!hasConnections) {
      return (
        <EmptyLayout
          icon={<ConnectorsEmptyDark height={200} />}
          header={copy.emptyHeader}
          message={copy.emptyMessage}
          actions={
            <FlexWrapper gap={8}>
              <Button
                label={copy.newLabel}
                icon={PlusIcon}
                variant={ButtonVariant.PRIMARY}
                onClick={handleOpenCreateConnectorModal}
              />
              <Button
                label="Documentation"
                icon={BookOpenIcon}
                variant={ButtonVariant.SECONDARY}
                onClick={handleReadTheDocs}
              />
            </FlexWrapper>
          }
        />
      );
    }

    if (!filteredConnections.length) {
      return <EmptyLayout message={`No ${copy.plural} match your search`} />;
    }

    return (
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
    );
  };

  return (
    <FlexWrapper fillWidth fillHeight direction={FlexDirection.COLUMN}>
      <FlexWrapper padding={"8px 12px"} fillWidth>
        <BaseToolbar
          leadingActions={[
            <TextInput
              key="search"
              value={state.search}
              onChange={handleSearchChange}
              placeholder={copy.searchPlaceholder}
              leading={{ icon: MagnifyingGlassIcon }}
              isDisabled={isToolbarDisabled}
              fillWidth
            />,
          ]}
          trailingActions={[
            <Button
              key="new-connector"
              label={copy.newLabel}
              icon={PlusIcon}
              variant={ButtonVariant.PRIMARY}
              isDisabled={isToolbarDisabled}
              onClick={handleOpenCreateConnectorModal}
            />,
          ]}
        />
      </FlexWrapper>
      <FlexItem grow={0} shrink={0} fillWidth>
        <HorizontalDivider />
      </FlexItem>
      <ConnectorListScrollArea>{renderContent()}</ConnectorListScrollArea>
    </FlexWrapper>
  );
};

export default ConnectionsPage;
