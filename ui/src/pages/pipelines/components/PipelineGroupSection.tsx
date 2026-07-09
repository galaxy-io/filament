import { useState } from "react";

import { CaretDownIcon, CaretRightIcon } from "@phosphor-icons/react";
import { styled } from "@linaria/react";
import { match } from "ts-pattern";

import Badge, { BadgeSize, BadgeVariant } from "@galaxy-io/dls/badge/Badge";
import FlexWrapper, { AlignItems, FlexDirection, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import PipelineCard from "@/pages/pipelines/components/PipelineCard";
import { PipelineGroup, PipelineListItem } from "@/pages/pipelines/types";

const GROUP_BAND_HEIGHT = 40;

const GROUP_LABELS: Record<PipelineGroup, string> = {
  [PipelineGroup.ACTIVE]: "Active",
  [PipelineGroup.NEEDS_ATTENTION]: "Needs attention",
  [PipelineGroup.PAUSED]: "Paused",
};

const GroupBandWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: ${GROUP_BAND_HEIGHT}px;

  padding: 0 16px 0 13px;

  display: flex;
  align-items: center;

  background-color: ${({ theme }) => theme.color.background.tertiary};

  cursor: pointer;
  user-select: none;
`);

const getGroupBadgeVariant = (group: PipelineGroup): BadgeVariant => {
  return match(group)
    .with(PipelineGroup.ACTIVE, () => BadgeVariant.SUCCESS)
    .with(PipelineGroup.NEEDS_ATTENTION, () => BadgeVariant.WARNING)
    .with(PipelineGroup.PAUSED, () => BadgeVariant.SECONDARY)
    .exhaustive();
};

interface PipelineGroupSectionProps {
  group: PipelineGroup;
  pipelines: PipelineListItem[];
  defaultExpanded?: boolean;
}

const PipelineGroupSection = ({
  group,
  pipelines,
  defaultExpanded = true,
}: PipelineGroupSectionProps) => {
  const [isExpanded, setIsExpanded] = useState(defaultExpanded);

  if (!pipelines.length) {
    return null;
  }

  return (
    <FlexWrapper fillWidth direction={FlexDirection.COLUMN}>
      <GroupBandWrapper onClick={() => setIsExpanded((prev) => !prev)}>
        <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
          <Icon
            component={isExpanded ? CaretDownIcon : CaretRightIcon}
            size={12}
            variant={IconVariant.SECONDARY}
          />
          <Text size={TextSize.BODY_MD}>{GROUP_LABELS[group]}</Text>
          <Badge
            count={pipelines.length}
            size={BadgeSize.SMALL}
            variant={getGroupBadgeVariant(group)}
          />
        </FlexWrapper>
      </GroupBandWrapper>
      {isExpanded && (
        <FlexWrapper fillWidth direction={FlexDirection.COLUMN}>
          {pipelines.map((pipeline) => (
            <PipelineCard key={pipeline.id} pipeline={pipeline} />
          ))}
        </FlexWrapper>
      )}
    </FlexWrapper>
  );
};

export default PipelineGroupSection;
