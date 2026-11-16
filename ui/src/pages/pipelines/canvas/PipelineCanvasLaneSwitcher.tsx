import type { FC } from "react";

import { styled } from "@linaria/react";
import { CalendarBlankIcon, WebhooksLogoIcon } from "@phosphor-icons/react";

import Beacon, { BeaconSize, BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import ToggleInput, {
  ToggleInputSize,
  ToggleInputVariant,
  type ToggleOption,
} from "@galaxy-io/dls/inputs/ToggleInput";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";

import { usePipelineExecutionLane } from "@/pages/pipelines/hooks/usePipelineExecutionLane";

import { usePipelineParams } from "@/module/hooks";

import { useListActivePipelineRunsQuery } from "@/api/queries/runs";

import { isContinuousRunActive } from "@/utils/runs";
import { mapOptionIdToEnum } from "@/utils/select";

const LANE_OPTIONS: ToggleOption<string>[] = [
  {
    id: String(ExecutionMode.BOUNDED),
    icon: CalendarBlankIcon,
    ariaLabel: "Batch resources",
  },
  {
    id: String(ExecutionMode.CONTINUOUS),
    icon: WebhooksLogoIcon,
    ariaLabel: "Webhook resources",
  },
];

const LaneSwitcherFrame = styled.div`
  position: relative;
  display: inline-flex;
`;

const LaneSwitcherLiveMark = styled.span`
  position: absolute;
  top: ${t.space[4]};
  right: ${t.space[4]};
  display: inline-flex;
  pointer-events: none;
`;

const PipelineCanvasLaneSwitcher: FC = () => {
  const { id } = usePipelineParams();
  const { isSplit, lane, setLane } = usePipelineExecutionLane();
  const { data } = useListActivePipelineRunsQuery(id);
  const isStreaming = (data?.runs ?? []).some(isContinuousRunActive);

  if (!isSplit) return null;

  return (
    <LaneSwitcherFrame>
      <ToggleInput
        ariaLabel="Resources"
        options={LANE_OPTIONS}
        value={String(lane)}
        onChange={(id) => setLane(mapOptionIdToEnum(ExecutionMode, id))}
        size={ToggleInputSize.MEDIUM}
        variant={ToggleInputVariant.PRIMARY}
      />
      {isStreaming && (
        <LaneSwitcherLiveMark>
          <Beacon
            variant={BeaconVariant.SUCCESS}
            size={BeaconSize.SMALL}
            ariaLabel="Stream running"
            isPulse
          />
        </LaneSwitcherLiveMark>
      )}
    </LaneSwitcherFrame>
  );
};

export default PipelineCanvasLaneSwitcher;
