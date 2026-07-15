import { useState } from "react";

import { CaretRightIcon } from "@phosphor-icons/react";
import { styled } from "@linaria/react";
import { match } from "ts-pattern";

import RotateWithTransition from "@galaxy-io/dls/animations/RotateWithTransition";
import Badge, { BadgeSize, BadgeVariant } from "@galaxy-io/dls/badge/Badge";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import PipelineCard from "@/pages/pipelines/components/PipelineCard";
import {
  PIPELINE_GROUP_BAND_HEIGHT,
  PIPELINE_GROUP_TO_LABEL_MAP,
  PIPELINE_INDICATOR_WIDTH,
} from "@/pages/pipelines/constants";
import { PipelineGroup, PipelineResource } from "@/pages/pipelines/types";

const IndicatorWrapper = styled.div`
  width: ${PIPELINE_INDICATOR_WIDTH}px;

  display: flex;
  align-items: center;
  justify-content: center;
`;

const GroupBandWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: ${PIPELINE_GROUP_BAND_HEIGHT}px;

  padding: 0 16px;

  display: flex;
  align-items: center;

  background-color: ${({ theme }) => theme.color.background.tertiary};

  border-bottom: 0.5px solid ${({ theme }) => theme.color.border.primary};

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

interface PipelineCardGroupProps {
  group: PipelineGroup;
  pipelines: PipelineResource[];
  defaultExpanded?: boolean;
}

const PipelineCardGroup = ({
  group,
  pipelines,
  defaultExpanded = true,
}: PipelineCardGroupProps) => {
  const [isExpanded, setIsExpanded] = useState(defaultExpanded);

  if (!pipelines.length) {
    return null;
  }

  return (
    <FlexWrapper fillWidth direction={FlexDirection.COLUMN}>
      <GroupBandWrapper onClick={() => setIsExpanded((prev) => !prev)}>
        <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
          <IndicatorWrapper>
            <RotateWithTransition isRotated={isExpanded} deg={90}>
              <Icon
                component={CaretRightIcon}
                size={12}
                variant={IconVariant.SECONDARY}
              />
            </RotateWithTransition>
          </IndicatorWrapper>
          <Text>{PIPELINE_GROUP_TO_LABEL_MAP[group]}</Text>
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

export default PipelineCardGroup;
