import { useMemo, useState } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { MagnifyingGlassIcon, WarningCircleIcon } from "@phosphor-icons/react";

import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import EmptyLayout from "@/layouts/EmptyLayout";

import { PipelineCanvasActionType } from "@/pages/pipelines/canvas/actions";
import EditWidgetSelectorItem from "@/pages/pipelines/canvas/edit/EditWidgetSelectorItem";
import { usePipelineCanvas } from "@/pages/pipelines/canvas/hooks";
import { PipelineNodeType } from "@/pages/pipelines/canvas/types";
import { createNodeFromConnection, getNextNodePosition } from "@/pages/pipelines/canvas/utils";

import { useListConnectionsQuery } from "@/api/queries/connectors";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import { ListConnectionsRequestSchema } from "@/gen/ingestion/v1/connections_pb";

const BodyWrapper = styled.div`
  width: 320px;
  max-height: 480px;
  display: flex;
  flex-direction: column;
  border-radius: 8px;
  overflow: hidden;
`;

const SearchWrapper = withTheme(styled.div<PropsWithTheme>`
  padding: 8px;
  flex-shrink: 0;
  background-color: ${({ theme }) => theme.color.background.tertiary};
`);

const ConnectionList = withTheme(styled.div<PropsWithTheme>`
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 8px;
  background-color: ${({ theme }) => theme.color.background.primary};
`);

const EmptyState = ({ message, icon }: { message: string; icon?: React.ReactNode }) => (
  <FlexWrapper fillWidth padding={24}>
    <EmptyLayout message={message} icon={icon} />
  </FlexWrapper>
);

const EditWidgetSelectorBody = () => {
  const [searchQuery, setSearchQuery] = useState("");
  const { state, dispatch } = usePipelineCanvas();

  const { data, isLoading, isError } = useListConnectionsQuery({
    input: create(ListConnectionsRequestSchema, {
      kind: ConnectorKind.UNSPECIFIED,
    }),
  });

  const filteredConnections = useMemo(() => {
    if (!data?.connections) return [];
    if (!searchQuery) return data.connections;
    return data.connections.filter((connection) =>
      connection.name.toLowerCase().includes(searchQuery.toLowerCase()),
    );
  }, [data?.connections, searchQuery]);

  const hasSourceNode = useMemo(
    () => state.nodes.some((node) => node.type === PipelineNodeType.SOURCE),
    [state.nodes],
  );

  const handleConnectionClick = (connection: Connection) => {
    const position = getNextNodePosition(connection.kind, state.nodes);

    dispatch({
      type: PipelineCanvasActionType.ADD_NODE,
      payload: createNodeFromConnection(connection, position),
    });
  };

  const renderContent = () => {
    if (isLoading) {
      return <EmptyState message="Loading connections..." />;
    }

    if (isError) {
      return (
        <EmptyState
          icon={<Icon component={WarningCircleIcon} size={20} variant={IconVariant.ERROR} />}
          message="Failed to load connections"
        />
      );
    }

    if (!data?.connections?.length) {
      return <EmptyState message="No connections found" />;
    }

    if (!filteredConnections.length) {
      return <EmptyState message="No connections match your search" />;
    }

    return (
      <ConnectionList>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={2}>
          {filteredConnections.map((connection) => {
            const isDisabled = hasSourceNode && connection.kind === ConnectorKind.SOURCE;

            return (
              <EditWidgetSelectorItem
                key={connection.id}
                name={connection.name}
                connector={connection.connector}
                kind={connection.kind}
                isDisabled={isDisabled}
                onClick={() => handleConnectionClick(connection)}
              />
            );
          })}
        </FlexWrapper>
      </ConnectionList>
    );
  };

  return (
    <BodyWrapper>
      <SearchWrapper>
        <TextInput
          value={searchQuery}
          onChange={setSearchQuery}
          placeholder="Search connections..."
          leading={{ icon: MagnifyingGlassIcon }}
          fillWidth
        />
      </SearchWrapper>
      <HorizontalDivider />
      {renderContent()}
    </BodyWrapper>
  );
};

export default EditWidgetSelectorBody;
