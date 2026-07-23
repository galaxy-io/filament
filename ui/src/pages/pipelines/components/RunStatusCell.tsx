import { InfoIcon } from "@phosphor-icons/react";

import Beacon from "@galaxy-io/dls/beacons/Beacon";
import FlexWrapper, { AlignItems } from "@galaxy-io/dls/containers/FlexWrapper";
import Wrapper from "@galaxy-io/dls/containers/Wrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import Tooltip, { TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";

import {
  RUN_ERROR_TOOLTIP_MAX_WIDTH,
  RUN_STATUS_TO_BEACON_VARIANT_MAP,
  RUN_STATUS_TO_LABEL_MAP,
  RUN_STATUS_TO_TEXT_VARIANT_MAP,
} from "@/pages/pipelines/constants";

import { RunStatus } from "@/gen/ingestion/v1/runs_pb";

interface RunStatusCellProps {
  status: RunStatus;
  error?: string;
}

const RunStatusCell = ({ status, error }: RunStatusCellProps) => {
  return (
    <FlexWrapper alignItems={AlignItems.CENTER} gap={8}>
      <Beacon
        variant={RUN_STATUS_TO_BEACON_VARIANT_MAP[status]}
        isPulse={status === RunStatus.RUNNING}
      />
      <Text size={TextSize.BODY_SM} variant={RUN_STATUS_TO_TEXT_VARIANT_MAP[status]}>
        {RUN_STATUS_TO_LABEL_MAP[status]}
      </Text>
      {error && (
        <Tooltip
          body={
            <Wrapper maxWidth={RUN_ERROR_TOOLTIP_MAX_WIDTH}>
              <Text size={TextSize.CAPTION} isMonospace isSelectable>
                {error}
              </Text>
            </Wrapper>
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

export default RunStatusCell;
