import type { FC } from "react";

import SelectInput, { SelectInputVariant } from "@galaxy-io/dls/inputs/SelectInput";
import Box from "@galaxy-io/dls/layout/Box";

import { ReadMode } from "@/gen/ingestion/v1/common_pb";

import {
  useCreatePipelineModalActions,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import type { CreatePipelineModalResourceRow } from "@/pages/pipelines/components/create/types";
import { getReadModeSelectOptions } from "@/pages/pipelines/components/resource/utils";

import { getEnumSelectValue, mapOptionIdToEnum } from "@/utils/select";

interface CreatePipelineModalResourcesReadModeCellProps {
  row: CreatePipelineModalResourceRow;
}

const CreatePipelineModalResourcesReadModeCell: FC<
  CreatePipelineModalResourcesReadModeCellProps
> = ({ row }) => {
  const { activeSinkId } = useCreatePipelineModalState();
  const { setResourceReadMode } = useCreatePipelineModalActions();
  const options = getReadModeSelectOptions(row.readModeOptions, row.readMode);

  const handleReadModeChange = (id: string | null) => {
    if (id === null) return;
    setResourceReadMode({
      sinkId: activeSinkId,
      resource: row.name,
      readMode: mapOptionIdToEnum(ReadMode, id),
    });
  };

  return (
    <Box fillWidth minWidth={0}>
      <SelectInput
        options={options}
        value={getEnumSelectValue(ReadMode, row.readMode)}
        onChange={handleReadModeChange}
        variant={SelectInputVariant.TERTIARY}
        isDisabled={!row.isSelected}
        ariaLabel={`Read mode for ${row.displayName}`}
        fillWidth
      />
    </Box>
  );
};

export default CreatePipelineModalResourcesReadModeCell;
