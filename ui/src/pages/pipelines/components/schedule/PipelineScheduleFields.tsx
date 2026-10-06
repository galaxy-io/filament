import { type ComponentProps, type PropsWithChildren, useState } from "react";

import CronInput from "@galaxy-io/dls/inputs/CronInput";
import Field from "@galaxy-io/dls/inputs/Field";
import SelectInput from "@galaxy-io/dls/inputs/SelectInput";
import SwitchInput from "@galaxy-io/dls/inputs/SwitchInput";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { Orientation } from "@galaxy-io/dls/theme/enums";
import Widget from "@galaxy-io/dls/widget/Widget";

import { PIPELINE_SCHEDULE_TIMEZONE_OPTIONS } from "@/pages/pipelines/settings/constants";
import type { PipelineSettingsPageScheduleState } from "@/pages/pipelines/settings/types";
import {
  formatPipelineScheduleSummary,
  isPipelineScheduleCronValid,
} from "@/pages/pipelines/settings/utils";

interface PipelineScheduleFieldsProps {
  header: string;
  size?: ComponentProps<typeof Widget>["size"];
  isOpenInitial?: boolean;
  state: PipelineSettingsPageScheduleState;
  onChange: (partial: Partial<PipelineSettingsPageScheduleState>) => void;
}

const PipelineScheduleFields = ({
  header,
  size,
  isOpenInitial = false,
  state,
  onChange,
  children,
}: PropsWithChildren<PipelineScheduleFieldsProps>) => {
  const [isOpen, setIsOpen] = useState(isOpenInitial);

  const summary = formatPipelineScheduleSummary(state);
  const cronError =
    state.cron.trim() !== "" && !isPipelineScheduleCronValid(state.cron)
      ? "Use 5 fields: minute hour day month weekday"
      : undefined;

  const handleEnabledChange = (isEnabled: boolean) => {
    setIsOpen(true);
    onChange({ isEnabled });
  };

  const handleCronChange = (cron: string) => {
    onChange({ cron });
  };

  const handleTimezoneChange = (id: string | null) => {
    if (id !== null) onChange({ timezone: id });
  };

  return (
    <Widget
      isCollapsible
      header={header}
      size={size}
      isOpen={isOpen}
      onOpenChange={() => setIsOpen((prev) => !prev)}
      actions={
        <Flex alignItems={AlignItems.CENTER} gap={12}>
          {state.isEnabled && summary && !isOpen && (
            <Text size={TextSize.BODY_SM} variant={TextVariant.SUCCESS}>
              {summary}
            </Text>
          )}
          <SwitchInput isChecked={state.isEnabled} onChange={handleEnabledChange} />
        </Flex>
      }
    >
      <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={16} fillWidth>
        <Field label="Cron" error={cronError} orientation={Orientation.HORIZONTAL} fillWidth>
          <CronInput value={state.cron} onChange={handleCronChange} fillWidth />
        </Field>
        <Field label="Timezone" orientation={Orientation.HORIZONTAL} fillWidth>
          <SelectInput
            options={PIPELINE_SCHEDULE_TIMEZONE_OPTIONS}
            value={state.timezone}
            onChange={handleTimezoneChange}
            isSearchable
            fillWidth
          />
        </Field>
        {children}
      </Flex>
    </Widget>
  );
};

export default PipelineScheduleFields;
