import { useState } from "react";

import { styled } from "@linaria/react";
import { MagnifyingGlassIcon, PlusIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";
import pluralize from "pluralize";

import Button, { ButtonSize } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import VerticalDivider from "@galaxy-io/dls/dividers/VerticalDivider";
import CheckboxInput from "@galaxy-io/dls/inputs/CheckboxInput";
import RadioInput from "@galaxy-io/dls/inputs/RadioInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import InfiniteScrollSentinel from "@/components/InfiniteScrollSentinel";

import { Flow } from "@/layouts/app/types";
import EmptyLayout from "@/layouts/EmptyLayout";
import ErrorLayout from "@/layouts/ErrorLayout";
import { LayoutSize } from "@/layouts/types";

import ConnectorTile from "@/pages/connectors/components/ConnectorTile";
import { CONNECTOR_KIND_TO_LABEL_MAP } from "@/pages/connectors/constants";
import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import { CREATE_PIPELINE_MODAL_CONNECTION_GHOST_COUNT } from "@/pages/pipelines/components/create/constants";
import CreatePipelineModalConnectionsExecutionMode from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalConnectionsExecutionMode";

import { useListConnectionsInfiniteQuery } from "@/api/queries/connections";

import { NOOP } from "@/constants";

import { isSearchMatch } from "@/utils/search";

const RowWrapper = withTheme(styled.div<PropsWithTheme<{ $isDisabled?: boolean }>>`
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 8px;
  flex-shrink: 0;

  border-radius: 4px;
  cursor: ${({ $isDisabled }) => ($isDisabled ? "default" : "pointer")};

  &:hover {
    background-color: ${({ theme, $isDisabled }) =>
      $isDisabled ? "transparent" : theme.color.background.tertiary};
  }
`);

const RowControlWrapper = styled.div`
  display: flex;
  align-items: center;
  flex-shrink: 0;
  pointer-events: none;
`;

const CreatePipelineModalConnectionsState = ({
  message,
  error,
  connectorKind,
}: {
  message: string;
  error?: Error | null;
  connectorKind: ConnectorKind;
}) => {
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
      size={ButtonSize.SMALL}
      onClick={handleCreateConnection}
    />
  );

  return (
    <FlexWrapper
      fillWidth
      fillHeight
      minHeight={240}
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
      padding={24}
    >
      {error ? (
        <ErrorLayout size={LayoutSize.SMALL} message={message} error={error} actions={actions} />
      ) : (
        <EmptyLayout size={LayoutSize.SMALL} message={message} actions={actions} />
      )}
    </FlexWrapper>
  );
};

const CreatePipelineModalConnectionRow = ({
  connection,
  kind,
}: {
  connection: Connection;
  kind: ConnectorKind;
}) => {
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
        <Text isEllipsis variant={isDisabled ? TextVariant.DISABLED : TextVariant.PRIMARY}>
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

const CreatePipelineModalConnectionsPane = ({ kind }: CreatePipelineModalConnectionsPaneProps) => {
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
        <FlexWrapper
          direction={FlexDirection.COLUMN}
          alignItems={AlignItems.STRETCH}
          gap={2}
          padding={"8px"}
          grow={1}
          basis={0}
          minHeight={0}
        >
          {Array.from({ length: CREATE_PIPELINE_MODAL_CONNECTION_GHOST_COUNT }, (_, index) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: static skeleton list
            <RowWrapper key={index}>
              <TextShimmer height={16} width={16} />
              <TextShimmer height={24} width={24} />
              <TextShimmer height={16} width={140} />
            </RowWrapper>
          ))}
        </FlexWrapper>
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
      <FlexWrapper
        direction={FlexDirection.COLUMN}
        alignItems={AlignItems.STRETCH}
        gap={2}
        padding={"8px"}
        grow={1}
        basis={0}
        minHeight={0}
        overflow="auto"
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
      </FlexWrapper>
    );
  };

  return (
    <FlexWrapper
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.STRETCH}
      grow={1}
      basis={0}
      minWidth={0}
    >
      <FlexWrapper direction={FlexDirection.COLUMN} gap={8} padding={"8px"} fillWidth>
        <TextInput
          value={state.search}
          onChange={handleSearchChange}
          placeholder={`Search ${pluralize(CONNECTOR_KIND_TO_LABEL_MAP[kind].toLowerCase())}...`}
          leading={{ icon: MagnifyingGlassIcon }}
          fillWidth
        />
      </FlexWrapper>
      <HorizontalDivider />
      {renderList()}
    </FlexWrapper>
  );
};

const CreatePipelineModalConnections = () => {
  const { supportedExecutionModes } = useCreatePipelineModalState();

  return (
    <FlexWrapper
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.STRETCH}
      grow={1}
      basis={0}
      minHeight={0}
    >
      <FlexWrapper alignItems={AlignItems.STRETCH} grow={1} basis={0} minHeight={0}>
        <CreatePipelineModalConnectionsPane kind={ConnectorKind.SOURCE} />
        <VerticalDivider />
        <CreatePipelineModalConnectionsPane kind={ConnectorKind.SINK} />
      </FlexWrapper>
      {supportedExecutionModes.length > 1 && (
        <>
          <HorizontalDivider />
          <FlexWrapper padding="12px" shrink={0} fillWidth>
            <CreatePipelineModalConnectionsExecutionMode />
          </FlexWrapper>
        </>
      )}
    </FlexWrapper>
  );
};

export default CreatePipelineModalConnections;
