import type { FC } from "react";

import { InfoIcon } from "@phosphor-icons/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import type { RunInfo, RunStatus } from "@/gen/ingestion/v1/runs_pb";

import PipelineRunStatusSwatch from "@/components/runs/PipelineRunStatusSwatch";
import {
  formatPipelineRunFailedResources,
  getPipelineRunStatusLabel,
  isPipelineRunCompletedWithFailures,
} from "@/components/runs/utils";

interface PipelineRunStatusProps {
  status: RunStatus;
  error?: RunInfo["error"];
  executionStatus?: RunInfo["executionStatus"];
  failedResources?: RunInfo["failedResources"];
}

const PipelineRunStatus: FC<PipelineRunStatusProps> = ({
  status,
  error,
  executionStatus,
  failedResources = [],
}) => {
  const failuresReason = isPipelineRunCompletedWithFailures(status, failedResources)
    ? formatPipelineRunFailedResources(failedResources)
    : undefined;
  const reason = executionStatus?.reason || error || failuresReason;

  return (
    <Flex alignItems={AlignItems.CENTER} gap={8}>
      <PipelineRunStatusSwatch
        status={status}
        failedResources={failedResources}
        executionStatus={executionStatus}
      />
      <Text size={TextSize.BODY_SM} lineClamp={1}>
        {getPipelineRunStatusLabel(status, failedResources, executionStatus)}
      </Text>
      {reason && (
        <Icon
          component={InfoIcon}
          variant={IconVariant.TERTIARY}
          size={14}
          ariaLabel="Status reason"
          tooltip={
            <Text size={TextSize.CAPTION} family={FontFamily.MONO} isSelectable>
              {reason}
            </Text>
          }
        />
      )}
    </Flex>
  );
};

export default PipelineRunStatus;
