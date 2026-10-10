import { type FC, useMemo, useState } from "react";

import { MagnifyingGlassIcon, PlusIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Divider from "@galaxy-io/dls/layout/Divider";
import ErrorLayout from "@galaxy-io/dls/layout/ErrorLayout";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import { isSearchMatch } from "@galaxy-io/dls/utils/search";
import { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";

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

import { IS_DEBUG } from "@/constants";

interface CreatePipelineModalResourcesState {
  search: string;
  isCreating: boolean;
}

const DEFAULT_RESOURCES_STATE: CreatePipelineModalResourcesState = {
  search: "",
  isCreating: false,
};

const CreatePipelineModalResources: FC = () => {
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
      <Flex alignItems={AlignItems.START} padding={24} fillWidth height="100%">
        <ErrorLayout
          header="Could not list resources"
          description="This source could not be inspected. Go back and check the connection, or continue to replicate everything it exposes."
          detail={IS_DEBUG ? discoverError.message : undefined}
        />
      </Flex>
    );
  }

  return (
    <Flex direction={FlexDirection.COLUMN} grow={1} basis={0} minHeight={0}>
      {sinks.length > 1 && <CreatePipelineModalResourcesTabs />}
      <Flex gap={8} padding={8} alignItems={AlignItems.CENTER} fillWidth>
        <TextInput
          value={localState.search}
          onChange={handleSearchChange}
          placeholder="Search resources..."
          icon={MagnifyingGlassIcon}
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
      </Flex>
      <Divider />
      {localState.isCreating && activeSink && (
        <>
          <PipelineResourceCreateForm
            sinks={[{ id: activeSink.connection.id, label: activeSink.connection.name }]}
            getError={getCreateError}
            onSave={handleCreate}
            onCancel={() => setCreating(false)}
            variant={WidgetVariant.PRIMARY}
          />
          <Divider />
        </>
      )}
      <CreatePipelineModalResourcesTable rows={filteredRows} />
    </Flex>
  );
};

export default CreatePipelineModalResources;
