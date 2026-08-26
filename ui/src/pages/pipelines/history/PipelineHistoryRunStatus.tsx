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

interface PipelineHistoryRunStatusProps {
  status: RunStatus;
  error?: RunInfo["error"];
}

const PipelineHistoryRunStatus = ({ status, error }: PipelineHistoryRunStatusProps) => {
  return (
    <FlexWrapper alignItems={AlignItems.CENTER} gap={8}>
      <Beacon
        variant={PIPELINE_RUN_STATUS_TO_BEACON_VARIANT_MAP[status]}
        isPulse={status === RunStatus.RUNNING}
      />
      <Text size={TextSize.BODY_SM} variant={PIPELINE_RUN_STATUS_TO_TEXT_VARIANT_MAP[status]}>
        {PIPELINE_RUN_STATUS_TO_LABEL_MAP[status]}
      </Text>
      {error && (
        <Tooltip
          body={
            <Text size={TextSize.CAPTION} isMonospace isSelectable>
              {error}
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
