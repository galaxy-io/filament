import Text from "@galaxy-io/dls/text/Text";

import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import PipelineScheduleFields from "@/pages/pipelines/components/schedule/PipelineScheduleFields";

const CreatePipelineModalDeliverySchedule = () => {
  const { schedule, executionMode } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  if (executionMode === ExecutionMode.CONTINUOUS)
    return <Text>Continuous pipelines run until paused or stopped. No schedule is needed.</Text>;

  return (
    <PipelineScheduleFields
      state={schedule}
      onChange={(partial) =>
        dispatch({
          type: CreatePipelineModalActionType.SET_SCHEDULE,
          payload: partial,
        })
      }
    />
  );
};

export default CreatePipelineModalDeliverySchedule;
