import type { FC } from "react";

import {
  useCreatePipelineModalActions,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import PipelineScheduleFields from "@/pages/pipelines/components/schedule/PipelineScheduleFields";

const CreatePipelineModalDeliverySchedule: FC = () => {
  const { schedule } = useCreatePipelineModalState();
  const { setSchedule } = useCreatePipelineModalActions();

  return (
    <PipelineScheduleFields
      header="Schedule"
      state={schedule}
      isOpenInitial={schedule.isEnabled}
      onChange={(partial) => setSchedule(partial)}
    />
  );
};

export default CreatePipelineModalDeliverySchedule;
