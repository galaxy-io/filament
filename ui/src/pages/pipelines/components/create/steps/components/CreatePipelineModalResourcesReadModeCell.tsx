import { styled } from "@linaria/react";

import SelectInput, {
  SelectInputVariant,
  type SelectOption,
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

const CreatePipelineModalResourcesReadModeCell = ({
  row,
}: {
  row: CreatePipelineModalResourceRow;
}) => {
  const { activeSinkId } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();
  const options: SelectOption[] = row.readModeOptions.map((mode) => ({
    id: String(mode),
    label: READ_MODE_TO_LABEL_MAP[mode],
    value: mode,
  }));

  return (
    <CellWrapper>
      <SelectInput
        options={options}
        /* @dls-migrate selectinput.value: `value` and `onChange` now carry option ids, not option objects. */ value={
          options.find((option) => option.value === row.readMode) ?? null
        }
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
        variant={SelectInputVariant.TERTIARY}
        isDisabled={!row.isSelected}
        fillWidth
      />
    </CellWrapper>
  );
};

export default CreatePipelineModalResourcesReadModeCell;
