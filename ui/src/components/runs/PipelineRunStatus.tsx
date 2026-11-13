import type { FC } from "react";

import { InfoIcon } from "@phosphor-icons/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import Square from "@galaxy-io/dls/shapes/Square";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import type { RunInfo, RunStatus } from "@/gen/ingestion/v1/runs_pb";

import {
  PIPELINE_EXECUTION_OBSERVED_STATE_TO_HUE_MAP,
  PIPELINE_EXECUTION_OBSERVED_STATE_TO_LABEL_MAP,
} from "@/components/runs/constants";
import {
  formatPipelineRunFailedResources,
  getPipelineRunStatusHue,
  getPipelineRunStatusLabel,
  isPipelineRunCompletedWithFailures,
} from "@/components/runs/utils";

import { ACTIVE_RUN_STATUSES } from "@/constants";

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
  const observedState = ACTIVE_RUN_STATUSES.has(status)
    ? executionStatus?.observedState
    : undefined;
  const display =
    observedState !== undefined
      ? {
          label: PIPELINE_EXECUTION_OBSERVED_STATE_TO_LABEL_MAP[observedState],
          hue: PIPELINE_EXECUTION_OBSERVED_STATE_TO_HUE_MAP[observedState],
        }
      : {
          label: getPipelineRunStatusLabel(status, failedResources),
          hue: getPipelineRunStatusHue(status, failedResources),
        };
  const failuresReason = isPipelineRunCompletedWithFailures(status, failedResources)
    ? formatPipelineRunFailedResources(failedResources)
    : undefined;
  const reason = executionStatus?.reason || error || failuresReason;

  return (
    <Flex alignItems={AlignItems.CENTER} gap={8}>
      <Square color={display.hue} />
      <Text size={TextSize.BODY_SM} lineClamp={1}>
        {display.label}
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
