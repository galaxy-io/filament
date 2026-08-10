import { useState } from "react";

import { create } from "@bufbuild/protobuf";
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
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { type Connection, ListConnectionsRequestSchema } from "@/gen/ingestion/v1/connections_pb";

import EmptyLayout, { EmptyLayoutSize } from "@/layouts/EmptyLayout";

import ConnectorTile from "@/pages/connectors/components/ConnectorTile";
import { CONNECTOR_KIND_TO_LABEL_MAP } from "@/pages/connectors/constants";
import {
  useCreatePipelineModalActions,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";

import { Flow } from "@/routes/__root";

import { useListConnectionsQuery } from "@/api/queries/connections";

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
      search: { flow: Flow.CREATE_CONNECTION, connectorKind },
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

interface CreatePipelineModalConnectionsPaneProps {
  kind: ConnectorKind;
  renderControl: (connection: Connection) => React.ReactNode;
  onConnectionClick: (connection: Connection) => void;
}

interface CreatePipelineModalConnectionsPaneState {
  search: string;
}

const DEFAULT_PANE_STATE: CreatePipelineModalConnectionsPaneState = {
  search: "",
};

const CreatePipelineModalConnectionsPane = ({
  kind,
  renderControl,
  onConnectionClick,
}: CreatePipelineModalConnectionsPaneProps) => {
  const [state, setState] = useState<CreatePipelineModalConnectionsPaneState>(DEFAULT_PANE_STATE);

  const handleSearchChange = (search: string) => {
    setState((prev) => ({ ...prev, search }));
  };

  const { data, isLoading, isError } = useListConnectionsQuery({
    input: create(ListConnectionsRequestSchema, { kind }),
  });

  const kindConnections = data?.connections ?? [];
  const filteredConnections = kindConnections.filter((connection) =>
    isSearchMatch(state.search, connection.name),
  );

  const renderList = () => {
    if (isLoading) {
      return (
        <CreatePipelineModalConnectionsEmpty
          message="Loading connections..."
          connectorKind={kind}
        />
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
          <RowWrapper key={connection.id} onClick={() => onConnectionClick(connection)}>
            <RowControlWrapper>{renderControl(connection)}</RowControlWrapper>
            <ConnectorTile connector={connection.connector} />
            <FlexItem minWidth={0} overflow="hidden">
              <Text isEllipsis>{connection.name}</Text>
            </FlexItem>
          </RowWrapper>
        ))}
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
  const { sourceConnection, sinkConnections } = useCreatePipelineModalState();
  const { selectSource, toggleSink } = useCreatePipelineModalActions();

  return (
    <FlexWrapper alignItems={AlignItems.STRETCH} grow={1} basis={0} minHeight={0}>
      <CreatePipelineModalConnectionsPane
        kind={ConnectorKind.SOURCE}
        renderControl={(connection) => (
          <RadioInput isSelected={sourceConnection?.id === connection.id} onChange={NOOP} />
        )}
        onConnectionClick={selectSource}
      />
      <VerticalDivider />
      <CreatePipelineModalConnectionsPane
        kind={ConnectorKind.SINK}
        renderControl={(connection) => (
          <CheckboxInput
            isChecked={sinkConnections.some((sink) => sink.id === connection.id)}
            onChange={NOOP}
            ariaLabel={connection.name}
          />
        )}
        onConnectionClick={toggleSink}
      />
    </FlexWrapper>
  );
};

export default CreatePipelineModalConnections;
