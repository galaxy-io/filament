import { AccordionSize } from "@galaxy-io/dls/accordion/Accordion";

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
      size={AccordionSize.LARGE}
      padding="16px"
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
