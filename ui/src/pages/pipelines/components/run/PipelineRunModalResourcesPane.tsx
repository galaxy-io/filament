import { type FC, useMemo, useState } from "react";

import { FlowArrowIcon, MagnifyingGlassIcon } from "@phosphor-icons/react";

import SearchInput from "@galaxy-io/dls/inputs/SearchInput";
import ToggleInput, { ToggleInputSize } from "@galaxy-io/dls/inputs/ToggleInput";
import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import EmptyLayout, { EmptyLayoutSize } from "@galaxy-io/dls/layout/EmptyLayout";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import InfiniteTable, { InfiniteTableSize } from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn } from "@galaxy-io/dls/table/types";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { isSearchMatch } from "@galaxy-io/dls/utils/search";

import BaseHeader, { BaseHeaderSize } from "@/components/BaseHeader";

import { PIPELINE_RESOURCE_COLUMNS_TABLE_INSET } from "@/pages/pipelines/components/resource/constants";
import {
  PIPELINE_RUN_MODAL_RESOURCE_MODE_COLUMN_WIDTH,
  PIPELINE_RUN_MODAL_RESOURCES_PANE_WIDTH,
  PIPELINE_RUN_RESOURCE_MODE_OPTIONS,
} from "@/pages/pipelines/components/run/constants";
import PipelineRunModalSectionChip from "@/pages/pipelines/components/run/PipelineRunModalSectionChip";
import { PipelineRunResourceMode } from "@/pages/pipelines/components/run/types";
import { setPipelineRunResourceMode } from "@/pages/pipelines/components/run/utils";

interface PipelineRunModalResourcesPaneProps {
  resources: string[];
  selectedResources: string[];
  resyncResources: string[];
  isDefault: boolean;
  canSubset: boolean;
  onSelectedChange: (selectedResources: string[]) => void;
  onResyncChange: (resyncResources: string[]) => void;
  onClose: () => void;
}

const PipelineRunModalResourcesPane: FC<PipelineRunModalResourcesPaneProps> = ({
  resources,
  selectedResources,
  resyncResources,
  isDefault,
  canSubset,
  onSelectedChange,
  onResyncChange,
  onClose,
}) => {
  const [search, setSearch] = useState("");

  const visibleResources = useMemo(
    () => resources.filter((resource) => isSearchMatch(search, resource)),
    [resources, search],
  );

  const handleSelectedChange = (next: string[]) =>
    onSelectedChange(
      resources.filter(
        (resource) =>
          next.includes(resource) ||
          (!visibleResources.includes(resource) && selectedResources.includes(resource)),
      ),
    );

  const columns = useMemo<TableColumn<string>[]>(
    () => [
      {
        id: "name",
        header: "Resource",
        isRowHeader: true,
        cell: ({ row }) => (
          <Text size={TextSize.BODY_SM} family={FontFamily.MONO} lineClamp={1}>
            {row}
          </Text>
        ),
      },
      {
        id: "mode",
        header: "Mode",
        width: PIPELINE_RUN_MODAL_RESOURCE_MODE_COLUMN_WIDTH,
        align: "right",
        cell: ({ row }) => (
          <Flex justifyContent={JustifyContent.END} fillWidth>
            <ToggleInput
              ariaLabel={`${row} mode`}
              options={PIPELINE_RUN_RESOURCE_MODE_OPTIONS}
              value={
                resyncResources.includes(row)
                  ? PipelineRunResourceMode.RESYNC
                  : PipelineRunResourceMode.CONTINUE
              }
              onChange={(mode) =>
                onResyncChange(setPipelineRunResourceMode(resources, resyncResources, [row], mode))
              }
              size={ToggleInputSize.X_SMALL}
              isDisabled={!selectedResources.includes(row)}
            />
          </Flex>
        ),
      },
    ],
    [resources, selectedResources, resyncResources, onResyncChange],
  );

  return (
    <Box width={PIPELINE_RUN_MODAL_RESOURCES_PANE_WIDTH} height="100%">
      <Flex direction={FlexDirection.COLUMN} height="100%">
        <Flex alignItems={AlignItems.CENTER} padding={[12, 12, 12, 16]} shrink={0}>
          <BaseHeader
            title="Resources"
            icon={FlowArrowIcon}
            size={BaseHeaderSize.MEDIUM}
            actions={[<PipelineRunModalSectionChip key="status" isDefault={isDefault} />]}
            onClose={onClose}
          />
        </Flex>
        <Divider />
        <Box padding={8}>
          <SearchInput
            value={search}
            onChange={setSearch}
            placeholder="Search resources..."
            ariaLabel="Search resources"
            fillWidth
          />
        </Box>
        <Divider />
        <Flex direction={FlexDirection.COLUMN} grow={1} basis={0} minHeight={0}>
          <InfiniteTable<string>
            ariaLabel="Resources"
            columns={columns}
            data={visibleResources}
            getRowId={(row) => row}
            size={InfiniteTableSize.SMALL}
            inset={PIPELINE_RESOURCE_COLUMNS_TABLE_INSET}
            value={canSubset ? selectedResources : undefined}
            onChange={canSubset ? handleSelectedChange : undefined}
            isSelectable={canSubset}
            emptyState={
              <EmptyLayout
                size={EmptyLayoutSize.SMALL}
                icon={MagnifyingGlassIcon}
                header="No resources match your search"
              />
            }
          />
        </Flex>
      </Flex>
    </Box>
  );
};

export default PipelineRunModalResourcesPane;
