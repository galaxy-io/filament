import FlexWrapper, { FlexDirection, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import PipelineScheduleFields from "@/pages/pipelines/components/schedule/PipelineScheduleFields";
import type { PipelineSettingsPageScheduleState } from "@/pages/pipelines/settings/types";
import { formatPipelineScheduleSummary } from "@/pages/pipelines/settings/utils";

interface CreatePipelineModalScheduleProps {
  schedule: PipelineSettingsPageScheduleState;
  onScheduleChange: (partial: Partial<PipelineSettingsPageScheduleState>) => void;
}

const CreatePipelineModalSchedule = ({
  schedule,
  onScheduleChange,
}: CreatePipelineModalScheduleProps) => {
  const summary = formatPipelineScheduleSummary(schedule);

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.SMALL} fillWidth>
      <PipelineScheduleFields state={schedule} onChange={onScheduleChange} />
      {schedule.isEnabled && summary && (
        <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
          {summary}
        </Text>
      )}
    </FlexWrapper>
  );
};

export default CreatePipelineModalSchedule;
