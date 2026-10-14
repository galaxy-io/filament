import type { FC } from "react";

import { CalendarIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Alert from "@galaxy-io/dls/feedback/Alert";
import { formatRelativeTime } from "@galaxy-io/dls/utils/format";

import { getPipelineNextFireAt } from "@/components/pipelines/utils";
import RouterLink from "@/components/RouterLink";

import { PIPELINE_SCHEDULE_DEFAULT_TIMEZONE } from "@/pages/pipelines/components/schedule/constants";
import { formatPipelineScheduleSummary } from "@/pages/pipelines/components/schedule/utils";

import { usePipelineParams } from "@/module/hooks";
import { createFilamentHref, FilamentPath } from "@/module/paths";

import { createGetPipelineInput, useGetPipelineQuery } from "@/api/queries/pipelines";

const PipelinePageScheduleBanner: FC = () => {
  const { id } = usePipelineParams();

  const { data } = useGetPipelineQuery({
    input: createGetPipelineInput(id),
  });
  const pipeline = data?.pipeline;
  const nextFireAt = getPipelineNextFireAt(pipeline);
  const config = pipeline?.schedule?.config;

  if (!nextFireAt || !config) {
    return null;
  }

  return (
    <Alert
      isBanner
      color="yellow"
      icon={CalendarIcon}
      header={`Next run ${formatRelativeTime(nextFireAt)}`}
      actions={
        <Button
          label="Edit schedule"
          size={ButtonSize.SMALL}
          variant={ButtonVariant.SECONDARY}
          href={createFilamentHref(FilamentPath.PIPELINE_SETTINGS, { id })}
          as={RouterLink}
        />
      }
    >
      {formatPipelineScheduleSummary({
        isEnabled: config.isEnabled,
        cron: config.cron,
        timezone: config.timezone || PIPELINE_SCHEDULE_DEFAULT_TIMEZONE,
      })}
    </Alert>
  );
};

export default PipelinePageScheduleBanner;
