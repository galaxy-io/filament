import { InfoIcon } from "@phosphor-icons/react";

import Beacon from "@galaxy-io/dls/beacons/Beacon";
import FlexWrapper, { AlignItems } from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import Tooltip, { TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";

import { type RunInfo, RunStatus } from "@/gen/ingestion/v1/runs_pb";

import {
  PIPELINE_RUN_STATUS_TO_BEACON_VARIANT_MAP,
  PIPELINE_RUN_STATUS_TO_LABEL_MAP,
  PIPELINE_RUN_STATUS_TO_TEXT_VARIANT_MAP,
} from "@/pages/pipelines/history/constants";
import { executionStateLabels, executionStateStatus } from "@/pages/pipelines/streaming";

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
  const displayStatus = executionStatus
    ? (executionStateStatus[executionStatus.observedState] ?? RunStatus.UNSPECIFIED)
    : status;
  const label = executionStatus
    ? (executionStateLabels[executionStatus.observedState] ?? "Unknown")
    : PIPELINE_RUN_STATUS_TO_LABEL_MAP[status];
  const reason = executionStatus?.reason || error;
  return (
    <FlexWrapper alignItems={AlignItems.CENTER} gap={8}>
      <Beacon
        variant={PIPELINE_RUN_STATUS_TO_BEACON_VARIANT_MAP[displayStatus]}
        isPulse={displayStatus === RunStatus.RUNNING || displayStatus === RunStatus.REQUESTED}
      />
      <Text
        size={TextSize.BODY_SM}
        variant={PIPELINE_RUN_STATUS_TO_TEXT_VARIANT_MAP[displayStatus]}
      >
        {label}
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
