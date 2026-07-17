import { styled } from "@linaria/react";
import { FloppyDiskIcon, PlayIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Chip from "@galaxy-io/dls/chips/Chip";
import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import ToggleInput from "@galaxy-io/dls/inputs/ToggleInput";
import Text, { TextSize, TextWeight } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import {
  PIPELINE_NAVBAR_HEIGHT,
  PIPELINE_STATUS_TO_CHIP_VARIANT_MAP,
  PIPELINE_STATUS_TO_LABEL_MAP,
} from "@/layouts/pipeline/constants";
import type { PipelineStatus } from "@/layouts/pipeline/types";

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
  isEnabled: boolean;
  onToggleEnabled: (enabled: boolean) => void;
  hasChanges: boolean;
  isSaving: boolean;
  onSave: () => void;
  isRunning: boolean;
  onRun: () => void;
}

const PipelineLayoutNavbar = ({
  name,
  status,
  isEnabled,
  onToggleEnabled,
  hasChanges,
  isSaving,
  onSave,
  isRunning,
  onRun,
}: PipelineLayoutNavbarProps) => {
  return (
    <PipelineLayoutNavbarWrapper>
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
        <Chip
          label={PIPELINE_STATUS_TO_LABEL_MAP[status]}
          variant={PIPELINE_STATUS_TO_CHIP_VARIANT_MAP[status]}
        />
        <Text size={TextSize.BODY_LG}>{name}</Text>
      </FlexWrapper>

      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
        <ToggleInput value={isEnabled} onChange={onToggleEnabled} />
        {hasChanges ? (
          <Button
            label="Save"
            icon={FloppyDiskIcon}
            variant={ButtonVariant.PRIMARY}
            size={ButtonSize.SMALL}
            isLoading={isSaving}
            onClick={onSave}
          />
        ) : (
          <Button
            label="Run"
            icon={PlayIcon}
            variant={ButtonVariant.PRIMARY}
            size={ButtonSize.SMALL}
            isLoading={isRunning}
            onClick={onRun}
          />
        )}
      </FlexWrapper>
    </PipelineLayoutNavbarWrapper>
  );
};

export default PipelineLayoutNavbar;
