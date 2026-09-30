import { styled } from "@linaria/react";

import { InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput from "@galaxy-io/dls/inputs/SelectInput";

import type { ReadMode } from "@/gen/ingestion/v1/common_pb";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import { CREATE_PIPELINE_MODAL_READ_MODE_DROPDOWN_WIDTH } from "@/pages/pipelines/components/create/constants";
import type { CreatePipelineModalResourceRow } from "@/pages/pipelines/components/create/types";
import { getReadModeSelectOptions } from "@/pages/pipelines/components/resource/utils";

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
  const options = getReadModeSelectOptions(row.readModeOptions);

  return (
    <CellWrapper>
      <SelectInput
        options={options}
        value={options.find((option) => option.value === row.readMode) ?? null}
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
        variant={InputVariant.TERTIARY}
        dropdownWidth={CREATE_PIPELINE_MODAL_READ_MODE_DROPDOWN_WIDTH}
        isDisabled={!row.isSelected}
        fillWidth
      />
    </CellWrapper>
  );
};

export default CreatePipelineModalResourcesReadModeCell;
