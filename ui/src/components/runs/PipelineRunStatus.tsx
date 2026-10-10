import type { FC } from "react";

import { InfoIcon } from "@phosphor-icons/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import Square from "@galaxy-io/dls/shapes/Square";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { FontFamily, Placement } from "@galaxy-io/dls/theme/enums";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";

import type { RunInfo, RunStatus } from "@/gen/ingestion/v1/runs_pb";

import {
  PIPELINE_EXECUTION_OBSERVED_STATE_TO_HUE_MAP,
  PIPELINE_EXECUTION_OBSERVED_STATE_TO_LABEL_MAP,
  PIPELINE_RUN_STATUS_TO_HUE_MAP,
  PIPELINE_RUN_STATUS_TO_LABEL_MAP,
} from "@/components/runs/constants";

interface PipelineRunStatusProps {
  status: RunStatus;
  error?: RunInfo["error"];
  executionStatus?: RunInfo["executionStatus"];
}

const PipelineRunStatus: FC<PipelineRunStatusProps> = ({ status, error, executionStatus }) => {
  const observedState = executionStatus?.observedState;
  const display =
    observedState !== undefined
      ? {
          label: PIPELINE_EXECUTION_OBSERVED_STATE_TO_LABEL_MAP[observedState],
          hue: PIPELINE_EXECUTION_OBSERVED_STATE_TO_HUE_MAP[observedState],
        }
      : {
          label: PIPELINE_RUN_STATUS_TO_LABEL_MAP[status],
          hue: PIPELINE_RUN_STATUS_TO_HUE_MAP[status],
        };
  const reason = executionStatus?.reason || error;

  return (
    <Flex alignItems={AlignItems.CENTER} gap={8}>
      <Square {...(display.hue === null ? {} : { color: display.hue })} />
      <Text size={TextSize.BODY_SM} lineClamp={1}>
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
        >
          <Icon component={InfoIcon} variant={IconVariant.TERTIARY} size={14} />
        </Tooltip>
      )}
    </Flex>
  );
};

export default PipelineRunStatus;
