import type { FC, ReactNode } from "react";

import type { JsonValue } from "@bufbuild/protobuf";

import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";

import type { ConfigField } from "@/gen/ingestion/v1/common_pb";

import { formatFieldLabel } from "@/components/fields/utils";

import {
  useCreatePipelineModalActions,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import {
  CREATE_PIPELINE_MODAL_RESOURCES_COLUMNS_PANE_FIELD_HEIGHT,
  CREATE_PIPELINE_MODAL_RESOURCES_COLUMNS_PANE_LABEL_WIDTH,
} from "@/pages/pipelines/components/create/constants";
import CreatePipelineModalResourcesCursorCell from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalResourcesCursorCell";
import CreatePipelineModalResourcesReadModeCell from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalResourcesReadModeCell";
import CreatePipelineModalResourcesWriteModeCell from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalResourcesWriteModeCell";
import type { CreatePipelineModalResourceRow } from "@/pages/pipelines/components/create/types";
import { usePipelineResourceSinkFields } from "@/pages/pipelines/components/resource/hooks/usePipelineResourceSinkFields";
import PipelineResourceSinkField from "@/pages/pipelines/components/resource/PipelineResourceSinkField";

interface CreatePipelineModalResourcesColumnsPaneFooterProps {
  row: CreatePipelineModalResourceRow;
}

interface CreatePipelineModalResourcesColumnsPaneFooterRow {
  label: string;
  tooltip?: string;
  cell: ReactNode;
}

const CreatePipelineModalResourcesColumnsPaneFooter: FC<
  CreatePipelineModalResourcesColumnsPaneFooterProps
> = ({ row }) => {
  const { activeSinkId, sinkConnections, nodeConfigs, hasReadLevers } =
    useCreatePipelineModalState();
  const { setResourceSinkConfig } = useCreatePipelineModalActions();
  const sinkFields = usePipelineResourceSinkFields(
    sinkConnections.find((connection) => connection.id === activeSinkId),
    nodeConfigs[activeSinkId] ?? {},
  );

  const handleSinkFieldChange = (field: ConfigField, value: JsonValue | undefined) => {
    const { [field.name]: _previous, ...rest } = row.sinkConfig;
    setResourceSinkConfig({
      sinkId: activeSinkId,
      resource: row.name,
      config: value === undefined ? rest : { ...rest, [field.name]: value },
    });
  };

  const rows: CreatePipelineModalResourcesColumnsPaneFooterRow[] = [
    ...(hasReadLevers
      ? [
          { label: "Read mode", cell: <CreatePipelineModalResourcesReadModeCell row={row} /> },
          { label: "Cursor", cell: <CreatePipelineModalResourcesCursorCell row={row} /> },
          { label: "Write mode", cell: <CreatePipelineModalResourcesWriteModeCell row={row} /> },
        ]
      : []),
    ...sinkFields.fields.map((field) => ({
      label: formatFieldLabel(field.name),
      tooltip: field.help || undefined,
      cell: (
        <PipelineResourceSinkField
          field={field}
          value={row.sinkConfig[field.name]}
          resourceName={row.displayName}
          isCompact
          pipelineValue={sinkFields.pipelineValues[field.name]}
          columns={row.columns}
          isDisabled={!row.isSelected}
          onChange={(value) => handleSinkFieldChange(field, value)}
        />
      ),
    })),
  ];

  if (rows.length === 0) return null;

  return (
    <>
      <Divider />
      <Flex
        direction={FlexDirection.COLUMN}
        alignItems={AlignItems.STRETCH}
        gap={8}
        padding={[12, 16]}
      >
        {rows.map(({ label, tooltip, cell }) => (
          <Flex
            key={label}
            alignItems={AlignItems.CENTER}
            gap={12}
            minHeight={CREATE_PIPELINE_MODAL_RESOURCES_COLUMNS_PANE_FIELD_HEIGHT}
          >
            <FlexItem shrink={0} basis={CREATE_PIPELINE_MODAL_RESOURCES_COLUMNS_PANE_LABEL_WIDTH}>
              <Tooltip body={tooltip}>
                <Text
                  size={TextSize.BODY_SM}
                  weight={TextWeight.MEDIUM}
                  variant={TextVariant.SECONDARY}
                >
                  {label}
                </Text>
              </Tooltip>
            </FlexItem>
            <Flex alignItems={AlignItems.CENTER} grow={1} basis={0} minWidth={0}>
              {cell}
            </Flex>
          </Flex>
        ))}
      </Flex>
    </>
  );
};

export default CreatePipelineModalResourcesColumnsPaneFooter;
