import { useMemo, useState } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { MagnifyingGlassIcon, PlusIcon, WarningCircleIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";

import Button, { ButtonSize } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, { AlignItems, JustifyContent } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { type Connection, ListConnectionsRequestSchema } from "@/gen/ingestion/v1/connections_pb";

import EmptyLayout, { EmptyLayoutSize } from "@/layouts/EmptyLayout";

import { PipelineCanvasActionType } from "@/pages/pipelines/canvas/actions";
import { createNodeFromConnection, getNextNodePosition } from "@/pages/pipelines/canvas/graph";
import PipelineCanvasConnectionSelectorItem from "@/pages/pipelines/canvas/PipelineCanvasConnectionSelectorItem";
import { usePipelineCanvas } from "@/pages/pipelines/canvas/PipelineCanvasProvider";
import { PipelineNodeType } from "@/pages/pipelines/canvas/types";

import { useListConnectionsQuery } from "@/api/queries/connections";

import { Flow } from "@/constants";

import { isSearchMatch } from "@/utils/search";

const DEFAULT_BODY_WIDTH = 320;

const BodyWrapper = styled.div<{ $width: number; $fillHeight?: boolean }>`
  width: ${({ $width }) => $width}px;
  height: ${({ $fillHeight }) => ($fillHeight ? "100%" : "auto")};
  max-height: 480px;
  display: flex;
  flex-direction: column;
  border-radius: 8px;
  overflow: hidden;
`;

const SearchWrapper = withTheme(styled.div<PropsWithTheme>`
  padding: 8px;
  flex-shrink: 0;
  background-color: ${({ theme }) => theme.color.background.primary};
`);

const ConnectionList = withTheme(styled.div<PropsWithTheme>`
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 8px;
  background-color: ${({ theme }) => theme.color.background.primary};

  display: flex;
  flex-direction: column;
  gap: 2px;
`);

const EmptyState = ({ message, icon }: { message: string; icon?: React.ReactNode }) => {
  const navigate = useNavigate();

  const handleCreateConnection = () => {
    navigate({ to: ".", search: { flow: Flow.CREATE_CONNECTION } });
  };

  return (
    <FlexWrapper
      fillWidth
      fillHeight
      minHeight={240}
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
      padding={24}
    >
      <EmptyLayout
        size={EmptyLayoutSize.SMALL}
        message={message}
        icon={icon}
        actions={[
          <Button
            key="create-connection"
            label="Create connection"
            icon={PlusIcon}
            size={ButtonSize.SMALL}
            onClick={handleCreateConnection}
          />,
        ]}
      />
    </FlexWrapper>
  );
};

interface PipelineCanvasConnectionSelectorProps {
  kindFilter?: ConnectorKind;
  onSelect?: (connection: Connection) => void;
  width?: number;
  fillHeight?: boolean;
}

interface PipelineCanvasConnectionSelectorState {
  search: string;
}

const DEFAULT_STATE: PipelineCanvasConnectionSelectorState = {
  search: "",
};

const PipelineCanvasConnectionSelector = ({
  kindFilter = ConnectorKind.UNSPECIFIED,
  onSelect,
  width = DEFAULT_BODY_WIDTH,
  fillHeight = false,
}: PipelineCanvasConnectionSelectorProps) => {
  const [state, setState] = useState<PipelineCanvasConnectionSelectorState>(DEFAULT_STATE);

  const handleSearchChange = (search: string) => {
    setState((prev) => ({ ...prev, search }));
  };
  const { state: canvasState, dispatch } = usePipelineCanvas();

  const { data, isLoading, isError } = useListConnectionsQuery({
    input: create(ListConnectionsRequestSchema, { kind: kindFilter }),
  });

  const kindConnections = data?.connections ?? [];

  const filteredConnections = useMemo(
    () => kindConnections.filter((connection) => isSearchMatch(state.search, connection.name)),
    [kindConnections, state.search],
  );

  const hasSourceNode = useMemo(
    () => canvasState.nodes.some((node) => node.type === PipelineNodeType.SOURCE),
    [canvasState.nodes],
  );

  const handleConnectionClick = (connection: Connection) => {
    if (onSelect) {
      onSelect(connection);
      return;
    }

    const position = getNextNodePosition(connection.kind, canvasState.nodes);

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

    if (!kindConnections.length) {
      return <EmptyState message="No connections found" />;
    }

    if (!filteredConnections.length) {
      return <EmptyState message="No connections match your search" />;
    }

    return (
      <ConnectionList>
        {filteredConnections.map((connection) => {
          const isDisabled = hasSourceNode && connection.kind === ConnectorKind.SOURCE;

          return (
            <PipelineCanvasConnectionSelectorItem
              key={connection.id}
              connection={connection}
              isDisabled={isDisabled}
              onClick={() => handleConnectionClick(connection)}
            />
          );
        })}
      </ConnectionList>
    );
  };

  return (
    <BodyWrapper $width={width} $fillHeight={fillHeight}>
      <SearchWrapper>
        <TextInput
          value={state.search}
          onChange={handleSearchChange}
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

export default PipelineCanvasConnectionSelector;
