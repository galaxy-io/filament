import { useMemo, useState } from "react";

import { styled } from "@linaria/react";
import { MagnifyingGlassIcon, WarningCircleIcon } from "@phosphor-icons/react";

import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import ErrorLayout from "@/layouts/ErrorLayout";

import { useCreatePipelineModalState } from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import CreatePipelineModalResourcesTable from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalResourcesTable";
import CreatePipelineModalResourcesTabs from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalResourcesTabs";

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
  const { rowsBySink, sinks, activeSinkId, discoverError } = useCreatePipelineModalState();

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

  if (discoverError) {
    return (
      <FlexWrapper padding={24} fillWidth fillHeight>
        <ErrorLayout
          icon={<Icon component={WarningCircleIcon} size={20} variant={IconVariant.ERROR} />}
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
    </ResourcesWrapper>
  );
};

export default CreatePipelineModalResources;
