import type { FC } from "react";

import SelectInput, { SelectInputVariant } from "@galaxy-io/dls/inputs/SelectInput";
import Box from "@galaxy-io/dls/layout/Box";

import { WriteMode } from "@/gen/ingestion/v1/common_pb";

import {
  useCreatePipelineModalActions,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import type { CreatePipelineModalResourceRow } from "@/pages/pipelines/components/create/types";
import { getWriteModeSelectOptions } from "@/pages/pipelines/components/resource/utils";

import { getEnumSelectValue, mapOptionIdToEnum } from "@/utils/select";

interface CreatePipelineModalResourcesWriteModeCellProps {
  row: CreatePipelineModalResourceRow;
}

const CreatePipelineModalResourcesWriteModeCell: FC<
  CreatePipelineModalResourcesWriteModeCellProps
> = ({ row }) => {
  const { activeSinkId } = useCreatePipelineModalState();
  const { setResourceWriteMode } = useCreatePipelineModalActions();
  const options = getWriteModeSelectOptions(row.writeModeOptions, row.writeMode);

  const handleWriteModeChange = (id: string | null) => {
    if (id === null) return;
    setResourceWriteMode({
      sinkId: activeSinkId,
      resource: row.name,
      writeMode: mapOptionIdToEnum(WriteMode, id),
    });
  };

  return (
    <Box fillWidth minWidth={0}>
      <SelectInput
        options={options}
        value={getEnumSelectValue(WriteMode, row.writeMode)}
        onChange={handleWriteModeChange}
        variant={SelectInputVariant.TERTIARY}
        isDisabled={!row.isSelected}
        ariaLabel={`Write mode for ${row.displayName}`}
        fillWidth
      />
    </Box>
  );
};

export default CreatePipelineModalResourcesWriteModeCell;
