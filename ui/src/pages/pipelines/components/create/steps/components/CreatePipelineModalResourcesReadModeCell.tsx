import type { FC } from "react";

import SelectInput, { SelectInputVariant } from "@galaxy-io/dls/inputs/SelectInput";
import Box from "@galaxy-io/dls/layout/Box";

import { ReadMode } from "@/gen/ingestion/v1/common_pb";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import type { CreatePipelineModalResourceRow } from "@/pages/pipelines/components/create/types";
import { getReadModeSelectOptions } from "@/pages/pipelines/components/resource/utils";

import { mapOptionIdToEnum } from "@/utils/select";

const CreatePipelineModalResourcesReadModeCell: FC<{
  row: CreatePipelineModalResourceRow;
}> = ({ row }) => {
  const { activeSinkId } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();
  const options = getReadModeSelectOptions(row.readModeOptions);

  const handleReadModeChange = (id: string | null) => {
    if (id === null) return;
    dispatch({
      type: CreatePipelineModalActionType.SET_RESOURCE_READ_MODE,
      payload: {
        sinkId: activeSinkId,
        resource: row.name,
        readMode: mapOptionIdToEnum(ReadMode, id),
      },
    });
  };

  return (
    <Box fillWidth minWidth={0}>
      <SelectInput
        options={options}
        value={String(row.readMode)}
        onChange={handleReadModeChange}
        variant={SelectInputVariant.TERTIARY}
        isDisabled={!row.isSelected}
        fillWidth
      />
    </Box>
  );
};

export default CreatePipelineModalResourcesReadModeCell;
