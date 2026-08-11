import { styled } from "@linaria/react";

import SelectInput, {
  type SelectInputOption,
  SelectInputVariant,
} from "@galaxy-io/dls/inputs/SelectInput";

import type { ReadMode } from "@/gen/ingestion/v1/common_pb";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import {
  CREATE_PIPELINE_MODAL_READ_MODE_DROPDOWN_WIDTH,
  READ_MODE_TO_LABEL_MAP,
} from "@/pages/pipelines/components/create/constants";
import type { CreatePipelineModalResourceRow } from "@/pages/pipelines/components/create/types";

const CellWrapper = styled.div`
  width: 100%;
  min-width: 0;
`;

interface CreatePipelineModalResourcesReadModeCellProps {
  row: CreatePipelineModalResourceRow;
}

const CreatePipelineModalResourcesReadModeCell = ({
  row,
}: CreatePipelineModalResourcesReadModeCellProps) => {
  const { activeSinkId } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  const options: SelectInputOption[] = row.readModeOptions.map((mode) => ({
    id: String(mode),
    label: READ_MODE_TO_LABEL_MAP[mode],
    value: mode,
  }));

  const selectedOption = options.find((option) => option.value === row.readMode) ?? null;

  return (
    <CellWrapper>
      <SelectInput
        options={options}
        value={selectedOption}
        onChange={(option) =>
          dispatch({
            type: CreatePipelineModalActionType.SET_RESOURCE_READ_MODE,
            payload: {
              sinkId: activeSinkId,
              resource: row.name,
              readMode: option.value as ReadMode,
            },
          })
        }
        variant={SelectInputVariant.SECONDARY}
        dropdownWidth={CREATE_PIPELINE_MODAL_READ_MODE_DROPDOWN_WIDTH}
        isDisabled={!row.isSelected}
        fillWidth
      />
    </CellWrapper>
  );
};

export default CreatePipelineModalResourcesReadModeCell;
