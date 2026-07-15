import { styled } from "@linaria/react";
import { PlayIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Chip from "@galaxy-io/dls/chips/Chip";
import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import ToggleInput from "@galaxy-io/dls/inputs/ToggleInput";
import Text, { TextWeight } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import {
  PIPELINE_NAVBAR_HEIGHT,
  PIPELINE_STATUS_TO_CHIP_VARIANT_MAP,
  PIPELINE_STATUS_TO_LABEL_MAP,
} from "@/layouts/pipeline/constants";
import type { PipelineStatus } from "@/layouts/pipeline/types";

import PipelineFlow from "@/pages/pipelines/components/PipelineFlow";

const PipelineLayoutNavbarWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: ${PIPELINE_NAVBAR_HEIGHT}px;

  padding: 0 12px;

  display: flex;
  align-items: center;
  justify-content: space-between;

  flex-shrink: 0;

  background-color: ${({ theme }) => theme.color.background.base};
`);

interface PipelineLayoutNavbarProps {
  name: string;
  status: PipelineStatus;
  source: string;
  sinks: string[];
  isEnabled: boolean;
  onToggleEnabled: (enabled: boolean) => void;
  onRun: () => void;
}

const PipelineLayoutNavbar = ({
  name,
  status,
  source,
  sinks,
  isEnabled,
  onToggleEnabled,
  onRun,
}: PipelineLayoutNavbarProps) => {
  return (
    <PipelineLayoutNavbarWrapper>
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
        <Chip
          label={PIPELINE_STATUS_TO_LABEL_MAP[status]}
          variant={PIPELINE_STATUS_TO_CHIP_VARIANT_MAP[status]}
        />
        <Text weight={TextWeight.MEDIUM}>{name}</Text>
        <PipelineFlow source={source} sinks={sinks} />
      </FlexWrapper>

      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
        <ToggleInput value={isEnabled} onChange={onToggleEnabled} />
        <Button
          label="Run"
          icon={PlayIcon}
          variant={ButtonVariant.PRIMARY}
          size={ButtonSize.SMALL}
          onClick={onRun}
        />
      </FlexWrapper>
    </PipelineLayoutNavbarWrapper>
  );
};

export default PipelineLayoutNavbar;
