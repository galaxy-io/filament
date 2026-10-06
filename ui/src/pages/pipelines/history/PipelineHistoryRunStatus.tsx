import { InfoIcon } from "@phosphor-icons/react";

import Beacon from "@galaxy-io/dls/beacons/Beacon";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { FontFamily, Placement } from "@galaxy-io/dls/theme/enums";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";

import { type RunInfo, RunStatus } from "@/gen/ingestion/v1/runs_pb";

import {
  PIPELINE_EXECUTION_OBSERVED_STATE_PULSING,
  PIPELINE_EXECUTION_OBSERVED_STATE_TO_BEACON_VARIANT_MAP,
  PIPELINE_EXECUTION_OBSERVED_STATE_TO_LABEL_MAP,
  PIPELINE_EXECUTION_OBSERVED_STATE_TO_TEXT_VARIANT_MAP,
  PIPELINE_RUN_STATUS_TO_BEACON_VARIANT_MAP,
  PIPELINE_RUN_STATUS_TO_LABEL_MAP,
  PIPELINE_RUN_STATUS_TO_TEXT_VARIANT_MAP,
} from "@/pages/pipelines/history/constants";

interface PipelineHistoryRunStatusProps {
  status: RunStatus;
  error?: RunInfo["error"];
  executionStatus?: RunInfo["executionStatus"];
}

const PipelineHistoryRunStatus = ({
  status,
  error,
  executionStatus,
}: PipelineHistoryRunStatusProps) => {
  const observedState = executionStatus?.observedState;
  const display =
    observedState !== undefined
      ? {
          label: PIPELINE_EXECUTION_OBSERVED_STATE_TO_LABEL_MAP[observedState],
          beacon: PIPELINE_EXECUTION_OBSERVED_STATE_TO_BEACON_VARIANT_MAP[observedState],
          text: PIPELINE_EXECUTION_OBSERVED_STATE_TO_TEXT_VARIANT_MAP[observedState],
          isPulse: PIPELINE_EXECUTION_OBSERVED_STATE_PULSING.has(observedState),
        }
      : {
          label: PIPELINE_RUN_STATUS_TO_LABEL_MAP[status],
          beacon: PIPELINE_RUN_STATUS_TO_BEACON_VARIANT_MAP[status],
          text: PIPELINE_RUN_STATUS_TO_TEXT_VARIANT_MAP[status],
          isPulse: status === RunStatus.RUNNING,
        };
  const reason = executionStatus?.reason || error;

  return (
    <Flex alignItems={AlignItems.CENTER} gap={8}>
      <Beacon variant={display.beacon} isPulse={display.isPulse} />
      <Text size={TextSize.BODY_SM} variant={display.text}>
        {display.label}
      </Text>
      {reason && (
        <Tooltip
          body={
            <Text size={TextSize.CAPTION} family={FontFamily.MONO} isSelectable>
              {reason}
            </Text>
          }
          placement={Placement.RIGHT}
          /* @dls-migrate tooltip.isInteractive: Every tooltip is hoverable now; a bubble with its own layout is a `Popover`. */ isInteractive
        >
          <Icon component={InfoIcon} variant={IconVariant.TERTIARY} size={14} />
        </Tooltip>
      )}
    </Flex>
  );
};

export default PipelineHistoryRunStatus;
