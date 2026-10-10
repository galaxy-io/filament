import type { FC } from "react";

import { WidgetSize } from "@galaxy-io/dls/widget/Widget";

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
      size={WidgetSize.LARGE}
      state={schedule}
      isOpenInitial
      onChange={(partial) => setSchedule(partial)}
    />
  );
};

export default CreatePipelineModalDeliverySchedule;
