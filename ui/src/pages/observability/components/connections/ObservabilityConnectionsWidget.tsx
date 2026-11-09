import { type FC, useMemo } from "react";

import { ArrowRightIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";
import Widget from "@galaxy-io/dls/widget/Widget";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import {
  CONNECTOR_KIND_TO_PATH_MAP,
  CONNECTOR_KIND_TO_PLURAL_LABEL_MAP,
} from "@/components/connections/constants";
import RouterLink from "@/components/RouterLink";

import {
  OBSERVABILITY_CONNECTOR_KIND_TO_CONNECTIONS_INPUT_MAP,
  OBSERVABILITY_CONNECTOR_KIND_TO_ICON_MAP,
} from "@/pages/observability/components/connections/constants";
import ObservabilityConnectionRow from "@/pages/observability/components/connections/ObservabilityConnectionRow";
import { createObservabilityConnectionStats } from "@/pages/observability/components/connections/utils";
import ObservabilityWidgetEmpty from "@/pages/observability/components/ObservabilityWidgetEmpty";
import { useObservabilityPipelines } from "@/pages/observability/hooks/useObservabilityPipelines";

import { createFilamentHref } from "@/module/paths";

import { useSuspenseListConnectionsQuery } from "@/api/queries/connections";

interface ObservabilityConnectionsWidgetProps {
  kind: ConnectorKind;
}

const ObservabilityConnectionsWidget: FC<ObservabilityConnectionsWidgetProps> = ({ kind }) => {
  const { entries } = useObservabilityPipelines();
  const { data } = useSuspenseListConnectionsQuery({
    input: OBSERVABILITY_CONNECTOR_KIND_TO_CONNECTIONS_INPUT_MAP[kind],
  });

  const stats = useMemo(() => createObservabilityConnectionStats(entries), [entries]);

  return (
    <Widget
      header={CONNECTOR_KIND_TO_PLURAL_LABEL_MAP[kind]}
      icon={OBSERVABILITY_CONNECTOR_KIND_TO_ICON_MAP[kind]}
      isFlush
      gap={0}
      actions={
        <Button
          label="View all"
          size={ButtonSize.SMALL}
          variant={ButtonVariant.TERTIARY}
          icon={ArrowRightIcon}
          isIconTrailing
          href={createFilamentHref(CONNECTOR_KIND_TO_PATH_MAP[kind])}
          as={RouterLink}
        />
      }
    >
      <FlexItem grow={1} basis={0} minHeight={0}>
        {data.connections.length === 0 ? (
          <ObservabilityWidgetEmpty
            icon={OBSERVABILITY_CONNECTOR_KIND_TO_ICON_MAP[kind]}
            header={`No ${CONNECTOR_KIND_TO_PLURAL_LABEL_MAP[kind].toLowerCase()}`}
          />
        ) : (
          <ScrollArea>
            <Box variant={BoxVariant.PRIMARY} fillWidth>
              {data.connections.map((connection) => (
                <ObservabilityConnectionRow
                  key={connection.id}
                  connection={connection}
                  stats={stats.get(connection.id)}
                />
              ))}
            </Box>
          </ScrollArea>
        )}
      </FlexItem>
    </Widget>
  );
};

export default ObservabilityConnectionsWidget;
