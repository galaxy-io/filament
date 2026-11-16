import { create } from "@bufbuild/protobuf";
import { DatabaseIcon, type Icon, TrayArrowDownIcon } from "@phosphor-icons/react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import {
  type ListConnectionsRequest,
  ListConnectionsRequestSchema,
} from "@/gen/ingestion/v1/connections_pb";

import { OBSERVABILITY_LIST_PAGE_SIZE } from "@/pages/observability/constants";

export const OBSERVABILITY_CONNECTOR_KIND_TO_CONNECTIONS_INPUT_MAP: Record<
  ConnectorKind,
  ListConnectionsRequest
> = {
  [ConnectorKind.UNSPECIFIED]: create(ListConnectionsRequestSchema, {
    pagination: { pageSize: OBSERVABILITY_LIST_PAGE_SIZE },
  }),
  [ConnectorKind.SOURCE]: create(ListConnectionsRequestSchema, {
    kind: ConnectorKind.SOURCE,
    pagination: { pageSize: OBSERVABILITY_LIST_PAGE_SIZE },
  }),
  [ConnectorKind.SINK]: create(ListConnectionsRequestSchema, {
    kind: ConnectorKind.SINK,
    pagination: { pageSize: OBSERVABILITY_LIST_PAGE_SIZE },
  }),
};

export const OBSERVABILITY_CONNECTOR_KIND_TO_ICON_MAP: Record<ConnectorKind, Icon> = {
  [ConnectorKind.UNSPECIFIED]: DatabaseIcon,
  [ConnectorKind.SOURCE]: DatabaseIcon,
  [ConnectorKind.SINK]: TrayArrowDownIcon,
};
