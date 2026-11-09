import type { FC, ReactNode } from "react";

import { InfoIcon } from "@phosphor-icons/react";

import Skeleton, { SkeletonSize } from "@galaxy-io/dls/feedback/Skeleton";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";

import {
  OBSERVABILITY_BAND_HEADER_HEIGHT,
  OBSERVABILITY_BAND_INFO_ICON_SIZE,
  OBSERVABILITY_BAND_VALUE_HEIGHT,
  OBSERVABILITY_BAND_VALUE_SKELETON_WIDTH,
} from "@/pages/observability/components/band/constants";

interface ObservabilityBandTileProps {
  label: string;
  info: string;
  actions?: ReactNode;
  value: ReactNode;
  isLoading: boolean;
  chart: ReactNode;
  top: ReactNode;
}

const ObservabilityBandTile: FC<ObservabilityBandTileProps> = ({
  label,
  info,
  actions,
  value,
  isLoading,
  chart,
  top,
}) => (
  <FlexItem grow={1} basis={0} minWidth={0}>
    <Flex direction={FlexDirection.COLUMN} fillWidth>
      <Flex direction={FlexDirection.COLUMN} gap={12} padding={16} fillWidth>
        <Flex
          alignItems={AlignItems.CENTER}
          justifyContent={JustifyContent.SPACE_BETWEEN}
          gap={8}
          height={OBSERVABILITY_BAND_HEADER_HEIGHT}
          minWidth={0}
        >
          <Flex alignItems={AlignItems.CENTER} gap={4} minWidth={0}>
            <Text variant={TextVariant.SECONDARY} lineClamp={1}>
              {label}
            </Text>
            <Icon
              component={InfoIcon}
              variant={IconVariant.TERTIARY}
              size={OBSERVABILITY_BAND_INFO_ICON_SIZE}
              ariaLabel={`About ${label.toLowerCase()}`}
              tooltip={info}
            />
          </Flex>
          {actions}
        </Flex>
        <Flex alignItems={AlignItems.CENTER} height={OBSERVABILITY_BAND_VALUE_HEIGHT} minWidth={0}>
          {isLoading ? (
            <Box
              width={OBSERVABILITY_BAND_VALUE_SKELETON_WIDTH}
              height={OBSERVABILITY_BAND_VALUE_HEIGHT}
            >
              <Skeleton size={SkeletonSize.SMALL} />
            </Box>
          ) : (
            value
          )}
        </Flex>
        {chart}
      </Flex>
      <Divider />
      <Box padding={16} fillWidth>
        {top}
      </Box>
    </Flex>
  </FlexItem>
);

export default ObservabilityBandTile;
