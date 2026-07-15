import { useMemo, useState } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import {
  BookOpenIcon,
  MagnifyingGlassIcon,
  PlusIcon,
  WarningCircleIcon,
} from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  FlexDirection,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Dropdown, { DropdownPosition } from "@galaxy-io/dls/dropdown/Dropdown";
import DropdownButton from "@galaxy-io/dls/dropdown/DropdownButton";
import DropdownItem from "@galaxy-io/dls/dropdown/DropdownItem";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import ConnectorEmptyDark from "@/assets/components/ConnectorsEmptyDark";

import BaseToolbar from "@/layouts/components/BaseToolbar";
import EmptyLayout from "@/layouts/EmptyLayout";
import ErrorLayout from "@/layouts/ErrorLayout";

import PipelineCardGroup from "@/pages/pipelines/components/PipelineCardGroup";
import PipelineCardLoading from "@/pages/pipelines/components/PipelineCardLoading";
import {
  PIPELINE_GROUP_TO_LABEL_MAP,
  PIPELINE_SEARCH_WIDTH,
} from "@/pages/pipelines/constants";
import { PipelineGroup } from "@/pages/pipelines/types";
import { toPipelineGroups, toPipelineResource } from "@/pages/pipelines/utils";

import { ToastVariant } from "@/providers/toast/ToastProvider";
import { useToast } from "@/providers/toast/useToast";

import {
  useCreatePipelineMutation,
  useListPipelinesQuery,
} from "@/api/queries/pipelines";

import { CreatePipelineRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";

import { CONNECTORS_DOCS_URL } from "@/constants";

const LOADING_ROW_COUNT = 20;

const PipelineListWrapper = styled.div`
  flex: 1;
  width: 100%;
  min-height: 0;

  display: flex;
  flex-direction: column;

  overflow-y: auto;
`;

export interface PipelinesPageState {
  search: string;
  groupFilter: PipelineGroup | null;
  isFiltersOpen: boolean;
}

const DEFAULT_STATE: PipelinesPageState = {
  search: "",
  groupFilter: null,
  isFiltersOpen: false,
};

const PipelinesPage = () => {
  const navigate = useNavigate();

  const [state, setState] = useState<PipelinesPageState>(DEFAULT_STATE);
  const { showToast } = useToast();

  const handleSearchChange = (value: string) => {
    setState((prev) => ({ ...prev, search: value }));
  };

  const handleSelectGroupFilter = (group: PipelineGroup | null) => {
    setState((prev) => ({ ...prev, groupFilter: group, isFiltersOpen: false }));
  };

  const handleCloseFilters = () => {
    setState((prev) => ({ ...prev, isFiltersOpen: false }));
  };

  const handleToggleFiltersOpen = () => {
    setState((prev) => ({ ...prev, isFiltersOpen: !prev.isFiltersOpen }));
  };

  const { data, isLoading, isError } = useListPipelinesQuery();
  const { mutate: createPipeline, isPending: isCreatingPipeline } =
    useCreatePipelineMutation();

  const isToolbarDisabled = isLoading || isError;

  const pipelineResources = useMemo(
    () => data?.pipelines.map(toPipelineResource) ?? [],
    [data],
  );

  const groups = useMemo(() => {
    const query = state.search.trim().toLowerCase();
    const filtered = query
      ? pipelineResources.filter((item) =>
          item.name.toLowerCase().includes(query),
        )
      : pipelineResources;
    return toPipelineGroups(filtered);
  }, [pipelineResources, state.search]);

  const visibleGroups = state.groupFilter
    ? [state.groupFilter]
    : Object.values(PipelineGroup);

  const totalVisiblePipelines = visibleGroups.reduce(
    (sum, group) => sum + groups[group].length,
    0,
  );

  const handleNewPipeline = () => {
    createPipeline(
      create(CreatePipelineRequestSchema, {
        name: "Untitled Pipeline",
      }),
      {
        onSuccess: (response) => {
          if (response.pipeline?.id) {
            navigate({
              to: "/pipelines/$id",
              params: { id: response.pipeline.id },
            });
          }
        },
        onError: (error) => {
          console.error(error);
          showToast({
            variant: ToastVariant.ERROR,
            header: "Failed to create pipeline",
            subheader: error.message,
          });
        },
      },
    );
  };

  const handleReadTheDocs = () => {
    window.open(CONNECTORS_DOCS_URL, "_blank");
  };

  const renderContent = () => {
    if (isLoading) {
      return (
        <FlexWrapper fillWidth direction={FlexDirection.COLUMN}>
          {Array.from({ length: LOADING_ROW_COUNT }).map((_, index) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: fixed-length loading skeleton, never reordered
            <PipelineCardLoading key={index} />
          ))}
        </FlexWrapper>
      );
    }

    if (isError) {
      return (
        <ErrorLayout
          icon={
            <Icon
              component={WarningCircleIcon}
              size={20}
              variant={IconVariant.ERROR}
            />
          }
          message="Failed to load pipelines. Please try again."
        />
      );
    }

    if (!data?.pipelines.length) {
      return (
        <EmptyLayout
          icon={<ConnectorEmptyDark height={200} />}
          header="No pipelines found"
          message="Create pipelines to move data between your connectors."
          actions={
            <FlexWrapper gap={8}>
              <Button
                label={isCreatingPipeline ? "Creating..." : "New pipeline"}
                icon={PlusIcon}
                variant={ButtonVariant.PRIMARY}
                isDisabled={isCreatingPipeline}
                onClick={handleNewPipeline}
              />
              <Button
                label="Read the docs"
                icon={BookOpenIcon}
                variant={ButtonVariant.SECONDARY}
                onClick={handleReadTheDocs}
              />
            </FlexWrapper>
          }
        />
      );
    }

    if (totalVisiblePipelines === 0) {
      return (
        <EmptyLayout
          message={
            state.search
              ? "No pipelines match your search"
              : "No pipelines match your filters"
          }
        />
      );
    }

    return visibleGroups.map((group) => (
      <PipelineCardGroup
        key={group}
        group={group}
        pipelines={groups[group]}
        defaultExpanded={group !== PipelineGroup.PAUSED}
      />
    ));
  };

  return (
    <FlexWrapper fillWidth fillHeight direction={FlexDirection.COLUMN}>
      <FlexWrapper padding={"8px 12px"} fillWidth>
        <BaseToolbar
          leadingActions={[
            <TextInput
              key="search"
              value={state.search}
              onChange={handleSearchChange}
              placeholder="Search"
              width={PIPELINE_SEARCH_WIDTH}
              leading={{ icon: MagnifyingGlassIcon }}
              isDisabled={isToolbarDisabled}
            />,
            <Dropdown
              key="filters"
              isOpen={state.isFiltersOpen}
              onClose={handleCloseFilters}
              position={DropdownPosition.BOTTOM_START}
              body={
                <>
                  <DropdownItem
                    label="All pipelines"
                    onClick={() => handleSelectGroupFilter(null)}
                  />
                  {Object.values(PipelineGroup).map((group) => (
                    <DropdownItem
                      key={group}
                      label={PIPELINE_GROUP_TO_LABEL_MAP[group]}
                      onClick={() => handleSelectGroupFilter(group)}
                    />
                  ))}
                </>
              }
            >
              <DropdownButton
                label={
                  state.groupFilter
                    ? PIPELINE_GROUP_TO_LABEL_MAP[state.groupFilter]
                    : "Filters"
                }
                isOpen={state.isFiltersOpen}
                onClick={handleToggleFiltersOpen}
                variant={ButtonVariant.SECONDARY}
                isDisabled={isToolbarDisabled}
              />
            </Dropdown>,
          ]}
          trailingActions={[
            <Button
              key="new-pipeline"
              label={isCreatingPipeline ? "Creating..." : "New pipeline"}
              icon={PlusIcon}
              variant={ButtonVariant.PRIMARY}
              isDisabled={isToolbarDisabled || isCreatingPipeline}
              onClick={handleNewPipeline}
            />,
          ]}
        />
      </FlexWrapper>
      <FlexItem grow={0} shrink={0} fillWidth>
        <HorizontalDivider />
      </FlexItem>
      <PipelineListWrapper>{renderContent()}</PipelineListWrapper>
    </FlexWrapper>
  );
};

export default PipelinesPage;
