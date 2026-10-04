import { useMemo, useState } from "react";

import { styled } from "@linaria/react";
import { MagnifyingGlassIcon, PlusIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";

import ErrorLayout from "@/layouts/ErrorLayout";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import CreatePipelineModalResourcesTable from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalResourcesTable";
import CreatePipelineModalResourcesTabs from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalResourcesTabs";
import PipelineResourceCreateForm, {
  type PipelineResourceCreateState,
} from "@/pages/pipelines/components/resource/PipelineResourceCreateForm";

import { isSearchMatch } from "@/utils/search";

const ResourcesWrapper = styled.div`
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
`;

interface CreatePipelineModalResourcesState {
  search: string;
  isCreating: boolean;
}

const DEFAULT_RESOURCES_STATE: CreatePipelineModalResourcesState = {
  search: "",
  isCreating: false,
};

const CreatePipelineModalResources = () => {
  const { rowsBySink, sinks, activeSinkId, discoverError, executionMode } =
    useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  const [localState, setLocalState] =
    useState<CreatePipelineModalResourcesState>(DEFAULT_RESOURCES_STATE);

  const isContinuous = executionMode === ExecutionMode.CONTINUOUS;
  const rows = rowsBySink[activeSinkId] ?? [];
  const filteredRows = useMemo(
    () => rows.filter((row) => isSearchMatch(localState.search, row.displayName)),
    [rows, localState.search],
  );

  const activeSink = sinks.find((sink) => sink.connection.id === activeSinkId);

  const handleSearchChange = (search: string) => {
    setLocalState((prev) => ({ ...prev, search }));
  };

  const setCreating = (isCreating: boolean) => {
    setLocalState((prev) => ({ ...prev, isCreating }));
  };

  const getCreateError = ({ resource }: PipelineResourceCreateState) =>
    rows.some((row) => row.name === resource) ? "This resource is already listed." : null;

  const handleCreate = ({ resource }: PipelineResourceCreateState) => {
    dispatch({
      type: CreatePipelineModalActionType.ADD_RESOURCE,
      payload: { sinkId: activeSinkId, name: resource },
    });
    setLocalState(DEFAULT_RESOURCES_STATE);
  };

  if (discoverError) {
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
      {sinks.length > 1 && (
        <>
          <CreatePipelineModalResourcesTabs />
          <HorizontalDivider />
        </>
      )}
      <FlexWrapper gap={8} padding="8px" alignItems={AlignItems.CENTER} fillWidth>
        <TextInput
          value={localState.search}
          onChange={handleSearchChange}
          placeholder="Search resources..."
          leading={{ icon: MagnifyingGlassIcon }}
          fillWidth
        />
        {isContinuous && (
          <FlexItem shrink={0}>
            <Button
              label="Add resource"
              icon={PlusIcon}
              variant={ButtonVariant.SECONDARY}
              isDisabled={localState.isCreating}
              onClick={() => setCreating(true)}
            />
          </FlexItem>
        )}
      </FlexWrapper>
      <HorizontalDivider />
      {localState.isCreating && activeSink && (
        <>
          <PipelineResourceCreateForm
            sinks={[{ id: activeSink.connection.id, label: activeSink.connection.name }]}
            getError={getCreateError}
            onSave={handleCreate}
            onCancel={() => setCreating(false)}
          />
          <HorizontalDivider />
        </>
      )}
      <CreatePipelineModalResourcesTable rows={filteredRows} />
    </ResourcesWrapper>
  );
};

export default CreatePipelineModalResources;
