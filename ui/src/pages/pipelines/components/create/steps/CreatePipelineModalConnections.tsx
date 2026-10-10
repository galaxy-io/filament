import { type FC, useState } from "react";

import { styled } from "@linaria/react";
import { MagnifyingGlassIcon, PlusIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";
import pluralize from "pluralize";

import Button from "@galaxy-io/dls/buttons/Button";
import Skeleton, { SkeletonSize, SkeletonVariant } from "@galaxy-io/dls/feedback/Skeleton";
import CheckboxInput from "@galaxy-io/dls/inputs/CheckboxInput";
import RadioInput from "@galaxy-io/dls/inputs/RadioInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import EmptyLayout, { EmptyLayoutSize } from "@galaxy-io/dls/layout/EmptyLayout";
import ErrorLayout, { ErrorLayoutSize } from "@galaxy-io/dls/layout/ErrorLayout";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";
import { Orientation } from "@galaxy-io/dls/theme/enums";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import InfiniteScrollSentinel from "@/components/InfiniteScrollSentinel";

import ConnectorTile from "@/pages/connectors/components/ConnectorTile";
import { CONNECTOR_KIND_TO_LABEL_MAP } from "@/pages/connectors/constants";
import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import { CREATE_PIPELINE_MODAL_CONNECTION_GHOST_COUNT } from "@/pages/pipelines/components/create/constants";
import CreatePipelineModalConnectionsExecutionMode from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalConnectionsExecutionMode";

import { Flow } from "@/module/types";

import { useListConnectionsInfiniteQuery } from "@/api/queries/connections";

import { IS_DEBUG, NOOP } from "@/constants";

import { isSearchMatch } from "@/utils/search";

const RowWrapper = styled.div<{ $isDisabled?: boolean }>`
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 8px;
  flex-shrink: 0;

  border-radius: ${t.radius.md};
  cursor: ${({ $isDisabled }) => ($isDisabled ? "default" : "pointer")};

  &:hover {
    background-color: ${({ $isDisabled }) =>
      $isDisabled ? "transparent" : t.color.background.hovered};
  }
`;

const RowControlWrapper = styled.div`
  display: flex;
  align-items: center;
  flex-shrink: 0;
  pointer-events: none;
`;

const CreatePipelineModalConnectionsState: FC<{
  message: string;
  error?: Error | null;
  connectorKind: ConnectorKind;
}> = ({ message, error, connectorKind }) => {
  const navigate = useNavigate();

  const handleCreateConnection = () => {
    void navigate({
      to: ".",
      search: (prev) => ({
        ...prev,
        connectionId: undefined,
        flow: Flow.CREATE_CONNECTION,
        connectorKind,
      }),
    });
  };

  const actions = (
    <Button
      label={`Create ${CONNECTOR_KIND_TO_LABEL_MAP[connectorKind].toLowerCase()}`}
      icon={PlusIcon}
      onClick={handleCreateConnection}
    />
  );

  return (
    <Flex
      fillWidth
      height="100%"
      minHeight={240}
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
      padding={24}
    >
      {error ? (
        <ErrorLayout
          size={ErrorLayoutSize.SMALL}
          header={message}
          detail={IS_DEBUG ? error.message : undefined}
          actions={actions}
        />
      ) : (
        <EmptyLayout size={EmptyLayoutSize.SMALL} header={message} actions={actions} />
      )}
    </Flex>
  );
};

const CreatePipelineModalConnectionRow: FC<{
  connection: Connection;
  kind: ConnectorKind;
}> = ({ connection, kind }) => {
  const { sourceConnection, sinkConnections, executionMode } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  const isSource = kind === ConnectorKind.SOURCE;
  const isDisabled = isSource
    ? connection.executionModes.length === 0
    : !connection.executionModes.includes(executionMode);

  const handleClick = () => {
    dispatch(
      isSource
        ? { type: CreatePipelineModalActionType.SELECT_SOURCE, payload: connection }
        : { type: CreatePipelineModalActionType.TOGGLE_SINK, payload: connection },
    );
  };

  return (
    <RowWrapper $isDisabled={isDisabled} onClick={isDisabled ? undefined : handleClick}>
      <RowControlWrapper>
        {isSource ? (
          <RadioInput
            isSelected={sourceConnection?.id === connection.id}
            isDisabled={isDisabled}
            onChange={NOOP}
          />
        ) : (
          <CheckboxInput
            isChecked={sinkConnections.some((sink) => sink.id === connection.id)}
            isDisabled={isDisabled}
            onChange={NOOP}
            ariaLabel={connection.name}
          />
        )}
      </RowControlWrapper>
      <ConnectorTile connector={connection.connector} kind={connection.kind} />
      <FlexItem minWidth={0} overflow="hidden">
        <Text lineClamp={1} variant={isDisabled ? TextVariant.DISABLED : TextVariant.PRIMARY}>
          {connection.name}
        </Text>
      </FlexItem>
    </RowWrapper>
  );
};

interface CreatePipelineModalConnectionsPaneProps {
  kind: ConnectorKind;
}

interface CreatePipelineModalConnectionsPaneState {
  search: string;
}

const DEFAULT_PANE_STATE: CreatePipelineModalConnectionsPaneState = {
  search: "",
};

const CreatePipelineModalConnectionsPane: FC<CreatePipelineModalConnectionsPaneProps> = ({
  kind,
}) => {
  const [state, setState] = useState<CreatePipelineModalConnectionsPaneState>(DEFAULT_PANE_STATE);

  const handleSearchChange = (search: string) => {
    setState((prev) => ({ ...prev, search }));
  };

  const { data, isLoading, error, hasNextPage, isFetchingNextPage, fetchNextPage } =
    useListConnectionsInfiniteQuery({ input: { kind } });

  const kindConnections = data?.pages.flatMap((page) => page.connections) ?? [];
  const filteredConnections = kindConnections.filter((connection) =>
    isSearchMatch(state.search, connection.name),
  );

  const renderList = () => {
    if (isLoading) {
      return (
        <Flex
          direction={FlexDirection.COLUMN}
          alignItems={AlignItems.STRETCH}
          gap={2}
          padding={8}
          grow={1}
          basis={0}
          minHeight={0}
        >
          {Array.from({ length: CREATE_PIPELINE_MODAL_CONNECTION_GHOST_COUNT }, (_, index) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: static skeleton list
            <RowWrapper key={index}>
              <Box width={16}>
                <Skeleton />
              </Box>
              <Box width={24}>
                <Skeleton variant={SkeletonVariant.RECT} size={SkeletonSize.SMALL} />
              </Box>
              <Box width={140}>
                <Skeleton />
              </Box>
            </RowWrapper>
          ))}
        </Flex>
      );
    }

    if (error) {
      return (
        <CreatePipelineModalConnectionsState
          message="Failed to load connections"
          error={error}
          connectorKind={kind}
        />
      );
    }

    if (!kindConnections.length) {
      return (
        <CreatePipelineModalConnectionsState
          message={`No ${pluralize(CONNECTOR_KIND_TO_LABEL_MAP[kind].toLowerCase())} found`}
          connectorKind={kind}
        />
      );
    }

    if (!filteredConnections.length) {
      return (
        <CreatePipelineModalConnectionsState
          message="No connections match your search"
          connectorKind={kind}
        />
      );
    }

    return (
      <FlexItem grow={1} basis={0} minHeight={0}>
        <ScrollArea>
          <Flex
            direction={FlexDirection.COLUMN}
            alignItems={AlignItems.STRETCH}
            gap={2}
            padding={8}
          >
            {filteredConnections.map((connection) => (
              <CreatePipelineModalConnectionRow
                key={connection.id}
                connection={connection}
                kind={kind}
              />
            ))}
            <InfiniteScrollSentinel
              hasNextPage={hasNextPage}
              isFetchingNextPage={isFetchingNextPage}
              fetchNextPage={fetchNextPage}
            />
          </Flex>
        </ScrollArea>
      </FlexItem>
    );
  };

  return (
    <Flex
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.STRETCH}
      grow={1}
      basis={0}
      minWidth={0}
    >
      <Flex
        alignItems={AlignItems.START}
        direction={FlexDirection.COLUMN}
        gap={8}
        padding={8}
        fillWidth
      >
        <TextInput
          value={state.search}
          onChange={handleSearchChange}
          placeholder={`Search ${pluralize(CONNECTOR_KIND_TO_LABEL_MAP[kind].toLowerCase())}...`}
          icon={MagnifyingGlassIcon}
          fillWidth
        />
      </Flex>
      <Divider />
      {renderList()}
    </Flex>
  );
};

const CreatePipelineModalConnections: FC = () => {
  const { supportedExecutionModes } = useCreatePipelineModalState();

  return (
    <Flex
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.STRETCH}
      grow={1}
      basis={0}
      minHeight={0}
    >
      <Flex alignItems={AlignItems.STRETCH} grow={1} basis={0} minHeight={0}>
        <CreatePipelineModalConnectionsPane kind={ConnectorKind.SOURCE} />
        <Divider orientation={Orientation.VERTICAL} />
        <CreatePipelineModalConnectionsPane kind={ConnectorKind.SINK} />
      </Flex>
      {supportedExecutionModes.length > 1 && (
        <>
          <Divider />
          <Flex alignItems={AlignItems.START} padding={12} shrink={0} fillWidth>
            <CreatePipelineModalConnectionsExecutionMode />
          </Flex>
        </>
      )}
    </Flex>
  );
};

export default CreatePipelineModalConnections;
