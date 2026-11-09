import type { FC } from "react";

import type { RoleColor } from "@galaxy-io/dls/theme/tokens/types";

import type { QueryAggregateRequest } from "@/gen/metrics/v1/metrics_pb";

import ObservabilityBandList from "@/pages/observability/components/band/ObservabilityBandList";
import { useObservabilityBandRanking } from "@/pages/observability/hooks/useObservabilityBandRanking";

interface ObservabilityBandTopListProps {
  title: string;
  emptyText: string;
  input: QueryAggregateRequest;
  colors: (RoleColor | undefined)[];
  format: (value: number) => string;
}

const ObservabilityBandTopList: FC<ObservabilityBandTopListProps> = ({
  title,
  emptyText,
  input,
  colors,
  format,
}) => {
  const { ranking, isLoading } = useObservabilityBandRanking(input);

  return (
    <ObservabilityBandList
      title={title}
      emptyText={emptyText}
      items={ranking.map(({ pipeline, value }) => ({ pipeline, value: format(value) }))}
      isLoading={isLoading}
      colors={colors}
    />
  );
};

export default ObservabilityBandTopList;
