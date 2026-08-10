import FlexWrapper, { FlexDirection, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import {
  useCreatePipelineModalActions,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import PipelineScheduleFields from "@/pages/pipelines/components/schedule/PipelineScheduleFields";
import { formatPipelineScheduleSummary } from "@/pages/pipelines/settings/utils";

const CreatePipelineModalSchedule = () => {
  const { schedule } = useCreatePipelineModalState();
  const { setSchedule } = useCreatePipelineModalActions();

  const summary = formatPipelineScheduleSummary(schedule);

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} fillWidth>
      <PipelineScheduleFields state={schedule} onChange={setSchedule} />
      {schedule.isEnabled && summary && (
        <Text size={TextSize.BODY_SM} variant={TextVariant.SUCCESS}>
          {summary}
        </Text>
      )}
    </FlexWrapper>
  );
};

export default CreatePipelineModalSchedule;
