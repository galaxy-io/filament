import { useMemo, useState } from "react";

import { styled } from "@linaria/react";
import { MagnifyingGlassIcon, WarningCircleIcon } from "@phosphor-icons/react";

import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import type { ReadMode } from "@/gen/ingestion/v1/common_pb";

import ErrorLayout from "@/layouts/ErrorLayout";

import CreatePipelineModalResourcesTable from "@/pages/pipelines/components/create/CreatePipelineModalResourcesTable";
import CreatePipelineModalResourcesTabs from "@/pages/pipelines/components/create/CreatePipelineModalResourcesTabs";
import type {
  CreatePipelineModalResourceRow,
  CreatePipelineModalSinkRow,
} from "@/pages/pipelines/components/create/types";

import { isSearchMatch } from "@/utils/search";

const ResourcesWrapper = styled.div`
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
`;

interface CreatePipelineModalResourcesProps {
  rows: CreatePipelineModalResourceRow[];
  sinks: CreatePipelineModalSinkRow[];
  activeSinkId: string;
  selectedCountBySink: Record<string, number>;
  issuesBySink: Record<string, string[]>;
  isCdc: boolean;
  isLoading: boolean;
  discoverError?: Error | null;
  onSinkSelect: (sinkId: string) => void;
  onSelectionChange: (
    sinkId: string,
    visibleNames: string[],
    selection: Record<string, boolean>,
  ) => void;
  onReadModeChange: (sinkId: string, resource: string, readMode: ReadMode) => void;
  onCursorChange: (sinkId: string, resource: string, cursorField: string) => void;
}

interface CreatePipelineModalResourcesState {
  search: string;
}

const DEFAULT_RESOURCES_STATE: CreatePipelineModalResourcesState = {
  search: "",
};

const CreatePipelineModalResources = ({
  rows,
  sinks,
  activeSinkId,
  selectedCountBySink,
  issuesBySink,
  isCdc,
  isLoading,
  discoverError,
  onSinkSelect,
  onSelectionChange,
  onReadModeChange,
  onCursorChange,
}: CreatePipelineModalResourcesProps) => {
  const [localState, setLocalState] =
    useState<CreatePipelineModalResourcesState>(DEFAULT_RESOURCES_STATE);

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
          <CreatePipelineModalResourcesTabs
            sinks={sinks}
            activeSinkId={activeSinkId}
            selectedCountBySink={selectedCountBySink}
            issuesBySink={issuesBySink}
            onSelect={onSinkSelect}
          />
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
        {isCdc && (
          <Text size={TextSize.CAPTION} variant={TextVariant.TERTIARY}>
            This connection replicates every insert, update and delete from the change stream. Read
            and write modes are managed by the connection.
          </Text>
        )}
      </FlexWrapper>
      <HorizontalDivider />
      <CreatePipelineModalResourcesTable
        rows={filteredRows}
        hasLevers={!isCdc}
        isLoading={isLoading}
        onSelectionChange={(visibleNames, selection) =>
          onSelectionChange(activeSinkId, visibleNames, selection)
        }
        onReadModeChange={(resource, readMode) =>
          onReadModeChange(activeSinkId, resource, readMode)
        }
        onCursorChange={(resource, cursorField) =>
          onCursorChange(activeSinkId, resource, cursorField)
        }
      />
    </ResourcesWrapper>
  );
};

export default CreatePipelineModalResources;
