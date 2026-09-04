import { useState } from "react";

import { styled } from "@linaria/react";
import { MagnifyingGlassIcon, PlusIcon, WarningCircleIcon } from "@phosphor-icons/react";
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
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import CheckboxInput from "@galaxy-io/dls/inputs/CheckboxInput";
import RadioInput from "@galaxy-io/dls/inputs/RadioInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Text from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import InfiniteScrollSentinel from "@/components/InfiniteScrollSentinel";

import { Flow } from "@/layouts/app/types";
import EmptyLayout, { EmptyLayoutSize } from "@/layouts/EmptyLayout";

import ConnectorTile from "@/pages/connectors/components/ConnectorTile";
import { CONNECTOR_KIND_TO_LABEL_MAP } from "@/pages/connectors/constants";
import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import { CREATE_PIPELINE_MODAL_CONNECTION_GHOST_COUNT } from "@/pages/pipelines/components/create/constants";

import { useListConnectionsInfiniteQuery } from "@/api/queries/connections";

import { NOOP } from "@/constants";

import { isSearchMatch } from "@/utils/search";

const RowWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 8px;
  flex-shrink: 0;

  border-radius: 4px;
  cursor: pointer;

  &:hover {
    background-color: ${({ theme }) => theme.color.background.tertiary};
  }
`);

const RowControlWrapper = styled.div`
  display: flex;
  align-items: center;
  flex-shrink: 0;
  pointer-events: none;
`;

const CreatePipelineModalConnectionsEmpty = ({
  message,
  icon,
  connectorKind,
}: {
  message: string;
  icon?: React.ReactNode;
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
            label={`Create ${CONNECTOR_KIND_TO_LABEL_MAP[connectorKind].toLowerCase()}`}
            icon={PlusIcon}
            size={ButtonSize.SMALL}
            onClick={handleCreateConnection}
          />,
        ]}
      />
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
  const { sourceConnection, sinkConnections } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  const isSource = kind === ConnectorKind.SOURCE;

  const handleClick = () => {
    dispatch(
      isSource
        ? { type: CreatePipelineModalActionType.SELECT_SOURCE, payload: connection }
        : { type: CreatePipelineModalActionType.TOGGLE_SINK, payload: connection },
    );
  };

  return (
    <RowWrapper onClick={handleClick}>
      <RowControlWrapper>
        {isSource ? (
          <RadioInput isSelected={sourceConnection?.id === connection.id} onChange={NOOP} />
        ) : (
          <CheckboxInput
            isChecked={sinkConnections.some((sink) => sink.id === connection.id)}
            onChange={NOOP}
            ariaLabel={connection.name}
          />
        )}
      </RowControlWrapper>
      <ConnectorTile connector={connection.connector} kind={connection.kind} />
      <FlexItem minWidth={0} overflow="hidden">
        <Text isEllipsis>{connection.name}</Text>
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

  const { data, isLoading, isError, hasNextPage, isFetchingNextPage, fetchNextPage } =
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

    if (isError) {
      return (
        <CreatePipelineModalConnectionsEmpty
          icon={<Icon component={WarningCircleIcon} size={20} variant={IconVariant.ERROR} />}
          message="Failed to load connections"
          connectorKind={kind}
        />
      );
    }

    if (!kindConnections.length) {
      return (
        <CreatePipelineModalConnectionsEmpty
          message={`No ${pluralize(CONNECTOR_KIND_TO_LABEL_MAP[kind].toLowerCase())} found`}
          connectorKind={kind}
        />
      );
    }

    if (!filteredConnections.length) {
      return (
        <CreatePipelineModalConnectionsEmpty
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
  return (
    <FlexWrapper alignItems={AlignItems.STRETCH} grow={1} basis={0} minHeight={0}>
      <CreatePipelineModalConnectionsPane kind={ConnectorKind.SOURCE} />
      <VerticalDivider />
      <CreatePipelineModalConnectionsPane kind={ConnectorKind.SINK} />
    </FlexWrapper>
  );
};

export default CreatePipelineModalConnections;
