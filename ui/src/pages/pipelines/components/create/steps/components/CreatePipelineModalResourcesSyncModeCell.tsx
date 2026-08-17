import { styled } from "@linaria/react";

import { InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import type { StandardSyncMode } from "@/gen/ingestion/v1/common_pb";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import {
  CREATE_PIPELINE_MODAL_SYNC_MODE_DROPDOWN_WIDTH,
  STANDARD_SYNC_MODE_TO_LABEL_MAP,
} from "@/pages/pipelines/components/create/constants";
import type { CreatePipelineModalResourceRow } from "@/pages/pipelines/components/create/types";

const CellWrapper = styled.div`
  width: 100%;
  min-width: 0;
`;

interface CreatePipelineModalResourcesSyncModeCellProps {
  row: CreatePipelineModalResourceRow;
}

const CreatePipelineModalResourcesSyncModeCell = ({
  row,
}: CreatePipelineModalResourcesSyncModeCellProps) => {
  const { activeSinkId } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  const options: SelectInputOption[] = row.syncModeOptions.map((mode) => ({
    id: String(mode),
    label: STANDARD_SYNC_MODE_TO_LABEL_MAP[mode],
    value: mode,
  }));

  const selectedOption = options.find((option) => option.value === row.syncMode) ?? null;

  return (
    <CellWrapper>
      <SelectInput
        options={options}
        value={selectedOption}
        onChange={(option) =>
          dispatch({
            type: CreatePipelineModalActionType.SET_RESOURCE_SYNC_MODE,
            payload: {
              sinkId: activeSinkId,
              resource: row.name,
              syncMode: option.value as StandardSyncMode,
            },
          })
        }
        variant={InputVariant.TERTIARY}
        dropdownWidth={CREATE_PIPELINE_MODAL_SYNC_MODE_DROPDOWN_WIDTH}
        isDisabled={!row.isSelected}
        fillWidth
      />
    </CellWrapper>
  );
};

export default CreatePipelineModalResourcesSyncModeCell;
