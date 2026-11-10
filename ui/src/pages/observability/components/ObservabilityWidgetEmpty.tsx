import type { FC } from "react";

import type { Icon as PhosphorIcon } from "@phosphor-icons/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import EmptyLayout, { EmptyLayoutSize } from "@galaxy-io/dls/layout/EmptyLayout";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import { Radius } from "@galaxy-io/dls/theme/enums";

import {
  OBSERVABILITY_EMPTY_ICON_SIZE,
  OBSERVABILITY_EMPTY_TILE_SIZE,
} from "@/pages/observability/constants";

interface ObservabilityWidgetEmptyProps {
  icon: PhosphorIcon;
  header: string;
}

const ObservabilityWidgetEmpty: FC<ObservabilityWidgetEmptyProps> = ({ icon, header }) => (
  <EmptyLayout
    size={EmptyLayoutSize.SMALL}
    header={header}
    graphic={
      <Box
        variant={BoxVariant.SECONDARY}
        hasBorder
        radius={Radius.MD}
        width={OBSERVABILITY_EMPTY_TILE_SIZE}
        height={OBSERVABILITY_EMPTY_TILE_SIZE}
      >
        <Flex alignItems={AlignItems.CENTER} justifyContent={JustifyContent.CENTER} height="100%">
          <Icon
            component={icon}
            size={OBSERVABILITY_EMPTY_ICON_SIZE}
            variant={IconVariant.SECONDARY}
          />
        </Flex>
      </Box>
    }
  />
);

export default ObservabilityWidgetEmpty;
