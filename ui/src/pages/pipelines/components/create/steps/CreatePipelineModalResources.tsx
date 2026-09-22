import { useMemo, useState } from "react";

import { styled } from "@linaria/react";
import { MagnifyingGlassIcon } from "@phosphor-icons/react";

import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import { ConnectorKind, ExecutionMode } from "@/gen/ingestion/v1/common_pb";

import ErrorLayout from "@/layouts/ErrorLayout";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import CreatePipelineModalDeliveryNodeConfig from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliveryNodeConfig";
import CreatePipelineModalResourcesTable from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalResourcesTable";
import CreatePipelineModalResourcesTabs from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalResourcesTabs";
import StreamResourcesTable from "@/pages/pipelines/components/StreamResourcesTable";

import { isSearchMatch } from "@/utils/search";

const ResourcesWrapper = styled.div`
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
`;

interface CreatePipelineModalResourcesState {
  search: string;
}

const DEFAULT_RESOURCES_STATE: CreatePipelineModalResourcesState = {
  search: "",
};

const CreatePipelineModalResources = () => {
  const { rowsBySink, sinks, activeSinkId, discoverError, executionMode, sourceConnection } =
    useCreatePipelineModalState();

  const dispatch = useCreatePipelineModalDispatch();
  const isContinuous = executionMode === ExecutionMode.CONTINUOUS;
  const [localState, setLocalState] =
    useState<CreatePipelineModalResourcesState>(DEFAULT_RESOURCES_STATE);

  const rows = rowsBySink[activeSinkId] ?? [];
  const filteredRows = useMemo(
    () => rows.filter((row) => isSearchMatch(localState.search, row.displayName)),
    [rows, localState.search],
  );

  const handleSearchChange = (search: string) => {
    setLocalState((prev) => ({ ...prev, search }));
  };

  if (discoverError && !isContinuous) {
    return (
      <FlexWrapper padding={24} fillWidth fillHeight>
        <ErrorLayout
          header="Could not list resources"
          message="This source could not be inspected. Go back and check the connection, or continue to replicate everything it exposes."
          error={discoverError}
        />
      </FlexWrapper>
    );
  }

  return (
    <ResourcesWrapper>
      {isContinuous && (
        <FlexWrapper direction={FlexDirection.COLUMN} gap={8} padding="16px" shrink={0} fillWidth>
          {discoverError && (
            <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
              Discovery is unavailable. You can still add subjects or topics manually.
            </Text>
          )}
          {sourceConnection && (
            <CreatePipelineModalDeliveryNodeConfig
              header="Source configuration"
              connection={sourceConnection}
              kind={ConnectorKind.SOURCE}
            />
          )}
        </FlexWrapper>
      )}
      {sinks.length > 1 && (
        <>
          <CreatePipelineModalResourcesTabs />
          <HorizontalDivider />
        </>
      )}
      {isContinuous && (
        <StreamResourcesTable
          fillHeight
          rows={rows.map((row) => ({
            id: row.name,
            label: row.destinationResource ?? "",
            subject: row.subject ?? row.name,
            selected: row.isSelected,
          }))}
          onChange={(row) =>
            dispatch({ type: CreatePipelineModalActionType.SET_STREAM_RESOURCE, payload: row })
          }
          onSelection={(selection) =>
            dispatch({
              type: CreatePipelineModalActionType.SET_RESOURCE_SELECTION,
              payload: {
                sinkId: activeSinkId,
                visibleNames: rows.map((row) => row.name),
                selection,
              },
            })
          }
          onAdd={() =>
            dispatch({
              type: CreatePipelineModalActionType.ADD_STREAM_RESOURCE,
              payload: { id: crypto.randomUUID(), sinkId: activeSinkId },
            })
          }
        />
      )}
      {!isContinuous && (
        <>
          <FlexWrapper
            direction={FlexDirection.COLUMN}
            gap={8}
            padding="8px"
            alignItems={AlignItems.STRETCH}
            fillWidth
          >
            <TextInput
              value={localState.search}
              onChange={handleSearchChange}
              placeholder="Search resources..."
              leading={{ icon: MagnifyingGlassIcon }}
              fillWidth
            />
          </FlexWrapper>
          <HorizontalDivider />
          <CreatePipelineModalResourcesTable rows={filteredRows} />
        </>
      )}
    </ResourcesWrapper>
  );
};

export default CreatePipelineModalResources;
