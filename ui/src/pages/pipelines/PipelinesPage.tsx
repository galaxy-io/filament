import { useMemo, useState } from "react";

import {
  BookOpenIcon,
  MagnifyingGlassIcon,
  PlusIcon,
  WarningCircleIcon,
} from "@phosphor-icons/react";
import { styled } from "@linaria/react";
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

import PipelinesEmptyDark from "@/assets/components/PipelinesEmptyDark";
import BaseToolbar from "@/layouts/components/BaseToolbar";
import EmptyLayout from "@/layouts/EmptyLayout";
import ErrorLayout from "@/layouts/ErrorLayout";

import PipelineCardGroup from "@/pages/pipelines/components/PipelineCardGroup";
import PipelineCardLoading from "@/pages/pipelines/components/PipelineCardLoading";
import {
  PIPELINE_GROUP_TO_LABEL_MAP,
  PIPELINE_SEARCH_WIDTH,
} from "@/pages/pipelines/constants";
import {
  groupPipelineItems,
  usePipelineListItems,
} from "@/pages/pipelines/hooks";
import { PipelineGroup } from "@/pages/pipelines/types";

const LOADING_ROW_COUNT = 20;

const ToolbarWrapper = styled.div`
  width: 100%;

  padding: 8px 12px;
`;

const PipelineListWrapper = styled.div`
  flex: 1;
  width: 100%;
  min-height: 0;

  display: flex;
  flex-direction: column;

  overflow-y: auto;
`;

const PipelinesPage = () => {
  const navigate = useNavigate();

  const [search, setSearch] = useState("");
  const [groupFilter, setGroupFilter] = useState<PipelineGroup | null>(null);
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);

  const { items, isLoading, isError } = usePipelineListItems();

  const isToolbarDisabled = isLoading || isError;

  const handleNewPipeline = () => {
    const id = crypto.randomUUID();
    navigate({ to: "/pipelines/$id", params: { id } });
  };

  const groups = useMemo(() => {
    const query = search.trim().toLowerCase();
    const filtered = query
      ? items.filter((item) => item.name.toLowerCase().includes(query))
      : items;
    return groupPipelineItems(filtered);
  }, [items, search]);

  const visibleGroups = groupFilter
    ? [groupFilter]
    : Object.values(PipelineGroup);

  const totalVisiblePipelines = visibleGroups.reduce(
    (sum, group) => sum + groups[group].length,
    0,
  );

  const handleSelectGroupFilter = (group: PipelineGroup | null) => {
    setGroupFilter(group);
    setIsFiltersOpen(false);
  };

  const handleReadTheDocs = () => {
    window.open("https://filament.getgalaxy.io/pipelines", "_blank");
  };

  const renderContent = () => {
    if (isLoading) {
      return (
        <FlexWrapper fillWidth direction={FlexDirection.COLUMN}>
          {Array.from({ length: LOADING_ROW_COUNT }).map((_, index) => (
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

    const hasPipelines = items.length > 0;

    if (!hasPipelines) {
      return (
        <EmptyLayout
          icon={<PipelinesEmptyDark height={200} />}
          header="No pipelines found"
          message="Create pipelines to move data between your connectors."
          actions={
            <FlexWrapper gap={8}>
              <Button
                label="New pipeline"
                icon={PlusIcon}
                variant={ButtonVariant.PRIMARY}
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
            search
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
      <ToolbarWrapper>
        <BaseToolbar
          leadingActions={[
            <TextInput
              key="search"
              value={search}
              onChange={setSearch}
              placeholder="Search"
              width={PIPELINE_SEARCH_WIDTH}
              leading={{ icon: MagnifyingGlassIcon }}
              isDisabled={isToolbarDisabled}
            />,
            <Dropdown
              key="filters"
              isOpen={isFiltersOpen}
              onClose={() => setIsFiltersOpen(false)}
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
                  groupFilter
                    ? PIPELINE_GROUP_TO_LABEL_MAP[groupFilter]
                    : "Filters"
                }
                isOpen={isFiltersOpen}
                onClick={() => setIsFiltersOpen((prev) => !prev)}
                variant={ButtonVariant.SECONDARY}
                isDisabled={isToolbarDisabled}
              />
            </Dropdown>,
          ]}
          trailingActions={[
            <Button
              key="new-pipeline"
              label="New pipeline"
              icon={PlusIcon}
              variant={ButtonVariant.PRIMARY}
              isDisabled={isToolbarDisabled}
              onClick={handleNewPipeline}
            />,
          ]}
        />
      </ToolbarWrapper>
      <FlexItem grow={0} shrink={0} fillWidth>
        <HorizontalDivider />
      </FlexItem>
      <PipelineListWrapper>{renderContent()}</PipelineListWrapper>
    </FlexWrapper>
  );
};

export default PipelinesPage;
