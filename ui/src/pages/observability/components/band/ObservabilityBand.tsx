import type { FC } from "react";

import Divider from "@galaxy-io/dls/layout/Divider";
import Flex from "@galaxy-io/dls/layout/Flex";
import { Orientation } from "@galaxy-io/dls/theme/enums";

import ObservabilityBandRunsTile from "@/pages/observability/components/band/ObservabilityBandRunsTile";
import ObservabilityBandThroughputTile from "@/pages/observability/components/band/ObservabilityBandThroughputTile";
import ObservabilityBandUsageTile from "@/pages/observability/components/band/ObservabilityBandUsageTile";

const ObservabilityBand: FC = () => (
  <Flex fillWidth>
    <ObservabilityBandRunsTile />
    <Divider orientation={Orientation.VERTICAL} />
    <ObservabilityBandThroughputTile />
    <Divider orientation={Orientation.VERTICAL} />
    <ObservabilityBandUsageTile />
  </Flex>
);

export default ObservabilityBand;
