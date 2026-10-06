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
import { READ_MODE_TO_LABEL_MAP } from "@/pages/pipelines/components/create/constants";
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
  }));

  const handleReadModeChange = (id: string | null) => {
    if (id === null) return;
    dispatch({
      type: CreatePipelineModalActionType.SET_RESOURCE_READ_MODE,
      payload: { sinkId: activeSinkId, resource: row.name, readMode: Number(id) as ReadMode },
    });
  };

  return (
    <CellWrapper>
      <SelectInput
        options={options}
        value={String(row.readMode)}
        onChange={handleReadModeChange}
        variant={SelectInputVariant.TERTIARY}
        isDisabled={!row.isSelected}
        fillWidth
      />
    </CellWrapper>
  );
};

export default CreatePipelineModalResourcesReadModeCell;
