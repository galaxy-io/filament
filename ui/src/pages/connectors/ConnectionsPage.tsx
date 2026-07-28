import { useMemo, useState } from "react";

import { create } from "@bufbuild/protobuf";
import { BookOpenIcon, MagnifyingGlassIcon, PlusIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";
import pluralize from "pluralize";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper from "@galaxy-io/dls/containers/FlexWrapper";
import GridWrapper from "@galaxy-io/dls/containers/GridWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { ListConnectionsRequestSchema } from "@/gen/ingestion/v1/connections_pb";

import ConnectorsEmptyDark from "@/assets/components/ConnectorsEmptyDark";

import EmptyLayout from "@/layouts/EmptyLayout";
import ListPageLayout from "@/layouts/ListPageLayout";

import ConnectionCard from "@/pages/connectors/components/card/ConnectionCard";
import {
  CONNECTOR_GRID_MIN_COLUMN_WIDTH,
  CONNECTOR_KIND_TO_EMPTY_MESSAGE_MAP,
  CONNECTOR_KIND_TO_LABEL_MAP,
} from "@/pages/connectors/constants";
import { usePipelineConnectionMap } from "@/pages/connectors/hooks/usePipelineConnectionMap";

import { useSuspenseListConnectionsQuery } from "@/api/queries/connections";

import { DOCUMENTATION_URL, Flow } from "@/constants";

import { isSearchMatch } from "@/utils/search";

interface ConnectionsPageProps {
  kind: ConnectorKind.SOURCE | ConnectorKind.SINK;
}

interface ConnectionsPageState {
  search: string;
}

const DEFAULT_STATE: ConnectionsPageState = {
  search: "",
};

const ConnectionsPage = ({ kind }: ConnectionsPageProps) => {
  const navigate = useNavigate();

  const kindLabel = CONNECTOR_KIND_TO_LABEL_MAP[kind].toLowerCase();
  const kindPlural = pluralize(kindLabel);

  const [state, setState] = useState<ConnectionsPageState>(DEFAULT_STATE);

  const handleSearchChange = (search: string) => {
    setState((prev) => ({ ...prev, search }));
  };

  const handleOpenCreateConnectorModal = () => {
    void navigate({
      to: ".",
      search: { flow: Flow.CREATE_CONNECTION, connectorKind: kind },
    });
  };

  const { data } = useSuspenseListConnectionsQuery({
    input: create(ListConnectionsRequestSchema, { kind }),
  });
  const kindConnections = data.connections;

  const { connectionIdsByPipelineId } = usePipelineConnectionMap();
  const pipelineCountsByConnectionId = useMemo(() => {
    const counts = new Map<string, number>();
    for (const connectionIds of connectionIdsByPipelineId.values()) {
      for (const connectionId of connectionIds) {
        counts.set(connectionId, (counts.get(connectionId) ?? 0) + 1);
      }
    }
    return counts;
  }, [connectionIdsByPipelineId]);

  const filteredConnections = useMemo(
    () => kindConnections.filter((connection) => isSearchMatch(state.search, connection.name)),
    [kindConnections, state.search],
  );

  const handleConnectionClick = (connectionId: string) => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, connectionId }),
    });
  };

  const handleReadTheDocs = () => {
    window.open(DOCUMENTATION_URL, "_blank", "noopener,noreferrer");
  };

  const renderContent = () => {
    if (!kindConnections.length) {
      return (
        <EmptyLayout
          icon={<ConnectorsEmptyDark height={200} />}
          header={`No ${kindPlural} found`}
          message={CONNECTOR_KIND_TO_EMPTY_MESSAGE_MAP[kind]}
          actions={
            <FlexWrapper gap={8}>
              <Button
                label={`New ${kindLabel}`}
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
      return (
        <EmptyLayout
          icon={<Icon component={MagnifyingGlassIcon} variant={IconVariant.TERTIARY} />}
          message={`No ${kindPlural} match your search`}
        />
      );
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
    <ListPageLayout
      search={state.search}
      onSearchChange={handleSearchChange}
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
    </ListPageLayout>
  );
};

export default ConnectionsPage;
