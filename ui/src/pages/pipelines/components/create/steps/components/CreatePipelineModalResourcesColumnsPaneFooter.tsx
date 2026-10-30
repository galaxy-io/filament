import type { FC } from "react";

import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";

import {
  CREATE_PIPELINE_MODAL_RESOURCES_COLUMNS_PANE_FIELD_HEIGHT,
  CREATE_PIPELINE_MODAL_RESOURCES_COLUMNS_PANE_LABEL_WIDTH,
} from "@/pages/pipelines/components/create/constants";
import CreatePipelineModalResourcesCursorCell from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalResourcesCursorCell";
import CreatePipelineModalResourcesReadModeCell from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalResourcesReadModeCell";
import CreatePipelineModalResourcesWriteModeCell from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalResourcesWriteModeCell";
import type { CreatePipelineModalResourceRow } from "@/pages/pipelines/components/create/types";

interface CreatePipelineModalResourcesColumnsPaneFooterProps {
  row: CreatePipelineModalResourceRow;
}

const CreatePipelineModalResourcesColumnsPaneFooter: FC<
  CreatePipelineModalResourcesColumnsPaneFooterProps
> = ({ row }) => (
  <Flex direction={FlexDirection.COLUMN} alignItems={AlignItems.STRETCH} gap={8} padding={[12, 16]}>
    {[
      { label: "Read mode", cell: <CreatePipelineModalResourcesReadModeCell row={row} /> },
      { label: "Cursor", cell: <CreatePipelineModalResourcesCursorCell row={row} /> },
      { label: "Write mode", cell: <CreatePipelineModalResourcesWriteModeCell row={row} /> },
    ].map(({ label, cell }) => (
      <Flex
        key={label}
        alignItems={AlignItems.CENTER}
        gap={12}
        minHeight={CREATE_PIPELINE_MODAL_RESOURCES_COLUMNS_PANE_FIELD_HEIGHT}
      >
        <FlexItem shrink={0} basis={CREATE_PIPELINE_MODAL_RESOURCES_COLUMNS_PANE_LABEL_WIDTH}>
          <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM} variant={TextVariant.SECONDARY}>
            {label}
          </Text>
        </FlexItem>
        <Flex alignItems={AlignItems.CENTER} grow={1} basis={0} minWidth={0}>
          {cell}
        </Flex>
      </Flex>
    ))}
  </Flex>
);

export default CreatePipelineModalResourcesColumnsPaneFooter;
