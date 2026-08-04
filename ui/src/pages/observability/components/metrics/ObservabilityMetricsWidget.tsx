import { ArrowsClockwiseIcon, GlobeIcon, HardDrivesIcon } from "@phosphor-icons/react";

import Beacon, { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import FlexWrapper, { AlignItems } from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import MetricCard from "@/components/metrics/MetricCard";
import MetricGroup from "@/components/metrics/MetricGroup";

import { OBSERVABILITY_METRICS_MOCK } from "@/pages/observability/components/metrics/constants";

import { formatBytes, formatCount } from "@/utils/format";

const ObservabilityMetricsWidget = () => (
  <MetricGroup
    primary={
      <MetricCard
        label="Source status"
        value={
          <FlexWrapper alignItems={AlignItems.CENTER} gap={8}>
            <Beacon variant={BeaconVariant.SUCCESS} />
            <Text size={TextSize.BODY_SM} variant={TextVariant.SUCCESS}>
              Complete
            </Text>
          </FlexWrapper>
        }
        noBorder
      />
    }
  >
    <MetricCard
      label="Num projects"
      value={formatCount(OBSERVABILITY_METRICS_MOCK.numProjects)}
      icon={GlobeIcon}
    />
    <MetricCard
      label="Num syncs"
      value={formatCount(OBSERVABILITY_METRICS_MOCK.numSyncs)}
      icon={ArrowsClockwiseIcon}
    />
    <MetricCard
      label="Total volume"
      value={formatBytes(OBSERVABILITY_METRICS_MOCK.totalVolumeBytes)}
      icon={HardDrivesIcon}
    />
  </MetricGroup>
);

export default ObservabilityMetricsWidget;
