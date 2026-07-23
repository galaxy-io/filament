import { styled } from "@linaria/react";
import {
  ArrowUUpLeftIcon,
  FloppyDiskIcon,
  PlayIcon,
} from "@phosphor-icons/react";

import Button, {
  ButtonSize,
  ButtonVariant,
} from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, {
  AlignItems,
  FlexGap,
} from "@galaxy-io/dls/containers/FlexWrapper";
import SelectInput, {
  type SelectInputOption,
  SelectInputSize,
  SelectInputVariant,
} from "@galaxy-io/dls/inputs/SelectInput";
import ToggleInput from "@galaxy-io/dls/inputs/ToggleInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { PIPELINE_NAVBAR_HEIGHT } from "@/layouts/pipeline/constants";

import PipelineFlow, {
  type PipelineFlowConnection,
  PipelineFlowSize,
} from "@/pages/pipelines/components/PipelineFlow";
import { formatPipelineName } from "@/pages/pipelines/utils";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

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
  source?: PipelineFlowConnection;
  sinks: PipelineFlowConnection[];
  hasEdges: boolean;
  isEnabled: boolean;
  onToggleEnabled: (enabled: boolean) => void;
  hasChanges: boolean;
  isSaving: boolean;
  onSave: () => void;
  isRunning: boolean;
  isRunDisabled: boolean;
  onRun: () => void;
  isPreview: boolean;
  versionOptions: SelectInputOption[];
  selectedVersionOption: SelectInputOption | null;
  onVersionChange: (option: SelectInputOption) => void;
  onBackToLatest: () => void;
}

const PipelineLayoutNavbar = ({
  name,
  source,
  sinks,
  hasEdges,
  isEnabled,
  onToggleEnabled,
  hasChanges,
  isSaving,
  onSave,
  isRunning,
  isRunDisabled,
  onRun,
  isPreview,
  versionOptions,
  selectedVersionOption,
  onVersionChange,
  onBackToLatest,
}: PipelineLayoutNavbarProps) => {
  return (
    <PipelineLayoutNavbarWrapper>
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
        <PipelineFlow
          source={source}
          sinks={sinks}
          hasEdges={hasEdges}
          size={PipelineFlowSize.SMALL}
        />
        <Text size={TextSize.BODY_LG}>{formatPipelineName(name)}</Text>
      </FlexWrapper>

      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
        {/* Save first: previewing remounts the canvas and would discard edits */}
        {versionOptions.length > 0 && (
          <SelectInput
            options={versionOptions}
            value={selectedVersionOption}
            onChange={onVersionChange}
            size={SelectInputSize.SMALL}
            variant={SelectInputVariant.SECONDARY}
            dropdownWidth={200}
            isDisabled={hasChanges}
          />
        )}
        {isPreview && (
          <Button
            label="Back to latest"
            icon={ArrowUUpLeftIcon}
            variant={ButtonVariant.TERTIARY}
            size={ButtonSize.SMALL}
            onClick={onBackToLatest}
          />
        )}
        {!isPreview &&
          (hasChanges ? (
            <Text size={TextSize.BODY_SM} variant={TextVariant.ERROR}>
              Unsaved changes
            </Text>
          ) : (
            <ToggleInput value={isEnabled} onChange={onToggleEnabled} />
          ))}
        {!isPreview &&
          (hasChanges ? (
            <Button
              label="Save"
              icon={FloppyDiskIcon}
              variant={ButtonVariant.SECONDARY}
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
              isDisabled={isRunDisabled}
              onClick={onRun}
            />
          ))}
      </FlexWrapper>
    </PipelineLayoutNavbarWrapper>
  );
};

export default PipelineLayoutNavbar;
