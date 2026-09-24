import { InfoIcon } from "@phosphor-icons/react";

import Beacon from "@galaxy-io/dls/beacons/Beacon";
import FlexWrapper, { AlignItems } from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import Tooltip, { TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";

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
    <FlexWrapper alignItems={AlignItems.CENTER} gap={8}>
      <Beacon variant={display.beacon} isPulse={display.isPulse} />
      <Text size={TextSize.BODY_SM} variant={display.text}>
        {display.label}
      </Text>
      {reason && (
        <Tooltip
          body={
            <Text size={TextSize.CAPTION} isMonospace isSelectable>
              {reason}
            </Text>
          }
          position={TooltipPosition.RIGHT}
          isInteractive
        >
          <Icon component={InfoIcon} variant={IconVariant.TERTIARY} size={14} />
        </Tooltip>
      )}
    </FlexWrapper>
  );
};

export default PipelineHistoryRunStatus;
