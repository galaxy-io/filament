import type { FC } from "react";

import Skeleton, { SkeletonSize } from "@galaxy-io/dls/feedback/Skeleton";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Link, { LinkUnderline, LinkVariant } from "@galaxy-io/dls/links/Link";
import Square, { SquareSize } from "@galaxy-io/dls/shapes/Square";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import type { RoleColor } from "@galaxy-io/dls/theme/tokens/types";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";

import { formatPipelineName } from "@/components/pipelines/utils";
import RouterLink from "@/components/RouterLink";

import {
  OBSERVABILITY_BAND_TOP_COUNT,
  OBSERVABILITY_BAND_TOP_ROW_HEIGHT,
} from "@/pages/observability/components/band/constants";
import type { ObservabilityBandListItem } from "@/pages/observability/components/band/types";

import { createFilamentHref, FilamentPath } from "@/module/paths";

interface ObservabilityBandListProps {
  title: string;
  emptyText: string;
  items: ObservabilityBandListItem[];
  isLoading: boolean;
  colors: (RoleColor | undefined)[];
}

const ObservabilityBandList: FC<ObservabilityBandListProps> = ({
  title,
  emptyText,
  items,
  isLoading,
  colors,
}) => {
  const renderRows = () => {
    if (isLoading) {
      return Array.from({ length: OBSERVABILITY_BAND_TOP_COUNT }, (_, index) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: static placeholder rows with no identity
        <Box key={index} height={OBSERVABILITY_BAND_TOP_ROW_HEIGHT} fillWidth>
          <Skeleton size={SkeletonSize.X_SMALL} />
        </Box>
      ));
    }
    if (items.length === 0) {
      return (
        <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
          {emptyText}
        </Text>
      );
    }
    return items
      .slice(0, OBSERVABILITY_BAND_TOP_COUNT)
      .map(({ pipeline, value, tooltip }, index) => {
        const valueText = (
          <Text size={TextSize.BODY_SM} family={FontFamily.MONO}>
            {value}
          </Text>
        );
        return (
          <Flex key={pipeline.id} alignItems={AlignItems.CENTER} gap={8} minWidth={0}>
            <Square size={SquareSize.SMALL} color={colors[index % colors.length]} />
            <FlexItem grow={1} minWidth={0}>
              <Text size={TextSize.BODY_SM} lineClamp={1}>
                <Link
                  href={createFilamentHref(FilamentPath.PIPELINE, { id: pipeline.id })}
                  as={RouterLink}
                  variant={LinkVariant.INHERIT}
                  underline={LinkUnderline.HOVER}
                >
                  {formatPipelineName(pipeline)}
                </Link>
              </Text>
            </FlexItem>
            <FlexItem shrink={0}>
              {tooltip ? <Tooltip body={tooltip}>{valueText}</Tooltip> : valueText}
            </FlexItem>
          </Flex>
        );
      });
  };

  return (
    <Flex direction={FlexDirection.COLUMN} gap={8} fillWidth>
      <Text size={TextSize.CAPTION} weight={TextWeight.MEDIUM} variant={TextVariant.TERTIARY}>
        {title}
      </Text>
      {renderRows()}
    </Flex>
  );
};

export default ObservabilityBandList;
