import { FlowArrowIcon } from "@phosphor-icons/react";
import { styled } from "@linaria/react";
import pluralize from "pluralize";

import Bold from "@galaxy-io/dls/text/Bold";
import Chip, { ChipVariant } from "@galaxy-io/dls/chips/Chip";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Text, {
  TextSize,
  TextVariant,
  TextWeight,
} from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ProviderKind } from "@/gen/ingestion/v1/common_pb";
import type { ProviderSpec } from "@/gen/ingestion/v1/providers_pb";

import ProviderTile from "@/pages/providers/components/ProviderTile";

const CardWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;

  padding: 16px;

  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 12px;

  background-color: ${({ theme }) => theme.color.background.primary};

  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 5px;

  transition: border-color 100ms ease;

  &:hover {
    border-color: ${({ theme }) => theme.color.border.secondary};
  }
`);

interface ProviderCardProps {
  provider: ProviderSpec;
  pipelineCount?: number;
}

const ProviderCard = ({ provider, pipelineCount = 0 }: ProviderCardProps) => {
  const isSource = provider.kind === ProviderKind.SOURCE;
  const kindLabel = isSource ? "Source" : "Sink";
  const pipelineLabel = pluralize("pipeline", pipelineCount, true);

  return (
    <CardWrapper>
      <FlexWrapper
        fillWidth
        alignItems={AlignItems.CENTER}
        justifyContent={JustifyContent.SPACE_BETWEEN}
      >
        <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
          <ProviderTile provider={provider.name} />
          <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM}>
            {provider.displayName || provider.name}
          </Text>
        </FlexWrapper>
        <Chip
          label={kindLabel}
          variant={isSource ? ChipVariant.LIME : ChipVariant.PINK}
        />
      </FlexWrapper>

      <Chip
        icon={FlowArrowIcon}
        label={pipelineLabel}
        variant={ChipVariant.TERTIARY}
      />

      <FlexWrapper
        fillWidth
        direction={FlexDirection.COLUMN}
        gap={FlexGap.MEDIUM}
      >
        <HorizontalDivider />
        <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.XSMALL}>
          <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
            Created on <Bold>Jan 1, 2025</Bold>
          </Text>
        </FlexWrapper>
      </FlexWrapper>
    </CardWrapper>
  );
};

export default ProviderCard;
