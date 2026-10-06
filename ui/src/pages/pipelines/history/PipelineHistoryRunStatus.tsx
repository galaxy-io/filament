import { InfoIcon } from "@phosphor-icons/react";

import Beacon, { BeaconSize } from "@galaxy-io/dls/beacons/Beacon";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { FontFamily, Placement } from "@galaxy-io/dls/theme/enums";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";

import type { RunInfo, RunStatus } from "@/gen/ingestion/v1/runs_pb";

import {
  PIPELINE_EXECUTION_OBSERVED_STATE_PULSING,
  PIPELINE_EXECUTION_OBSERVED_STATE_TO_BEACON_VARIANT_MAP,
  PIPELINE_EXECUTION_OBSERVED_STATE_TO_LABEL_MAP,
  PIPELINE_RUN_STATUS_PULSING,
  PIPELINE_RUN_STATUS_TO_BEACON_VARIANT_MAP,
  PIPELINE_RUN_STATUS_TO_LABEL_MAP,
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
          variant: PIPELINE_EXECUTION_OBSERVED_STATE_TO_BEACON_VARIANT_MAP[observedState],
          isPulse: PIPELINE_EXECUTION_OBSERVED_STATE_PULSING.has(observedState),
        }
      : {
          label: PIPELINE_RUN_STATUS_TO_LABEL_MAP[status],
          variant: PIPELINE_RUN_STATUS_TO_BEACON_VARIANT_MAP[status],
          isPulse: PIPELINE_RUN_STATUS_PULSING.has(status),
        };
  const reason = executionStatus?.reason || error;

  return (
    <Flex alignItems={AlignItems.CENTER} gap={8}>
      <Beacon
        label={display.label}
        variant={display.variant}
        size={BeaconSize.SMALL}
        isPulse={display.isPulse}
      />
      {reason && (
        <Tooltip
          body={
            <Text size={TextSize.CAPTION} family={FontFamily.MONO} isSelectable>
              {reason}
            </Text>
          }
          placement={Placement.RIGHT}
        >
          <Icon component={InfoIcon} variant={IconVariant.TERTIARY} size={14} />
        </Tooltip>
      )}
    </Flex>
  );
};

export default PipelineHistoryRunStatus;
