import { WidgetSize } from "@galaxy-io/dls/widget/Widget";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import PipelineScheduleFields from "@/pages/pipelines/components/schedule/PipelineScheduleFields";

const CreatePipelineModalDeliverySchedule = () => {
  const { schedule } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  return (
    <PipelineScheduleFields
      header="Schedule"
      size={WidgetSize.LARGE}
      state={schedule}
      isOpenInitial
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
