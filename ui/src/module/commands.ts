import { useMemo } from "react";

import type { Transport } from "@connectrpc/connect";
import { PlusIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";

import type { CommandPaletteItem } from "@galaxy-io/dls/navigation/CommandPalette";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { CONNECTOR_KIND_TO_PATH_MAP } from "@/components/connections/constants";
import { PIPELINE_CREATE_TITLE } from "@/components/pipelines/constants";
import { formatPipelineName } from "@/components/pipelines/utils";

import { CONNECTOR_KIND_TO_CREATE_TITLE_MAP } from "@/pages/connections/constants";

import {
  FILAMENT_COMMAND_GROUP_CONNECTIONS,
  FILAMENT_COMMAND_GROUP_CREATE,
  FILAMENT_COMMAND_GROUP_GO_TO,
  FILAMENT_COMMAND_GROUP_PIPELINES,
  FILAMENT_CONNECTOR_KIND_TO_CREATE_COMMAND_ID_MAP,
  FILAMENT_CONNECTOR_KIND_TO_NAV_ITEM_MAP,
} from "@/module/constants";
import {
  useFilamentFlowOpen,
  useFilamentMountedBase,
  useFilamentSearchUpdate,
} from "@/module/hooks";
import {
  FILAMENT_NAV_ITEM_TO_COMMAND_ID_MAP,
  FILAMENT_NAV_ITEM_TO_ICON_MAP,
  FILAMENT_NAV_ITEM_TO_KEYWORDS_MAP,
  FILAMENT_NAV_ITEM_TO_LABEL_MAP,
  FILAMENT_NAV_ITEM_TO_PATH_MAP,
  FILAMENT_NAV_ITEMS,
} from "@/module/nav";
import { FilamentPath } from "@/module/paths";
import type { FilamentLayoutSearch } from "@/module/schemas";
import { FilamentNavItem, Flow } from "@/module/types";

import { useListConnectionsQuery } from "@/api/queries/connections";
import { useListPipelinesQuery } from "@/api/queries/pipelines";

const ROOT_PATH = "/";

const CREATE_CONNECTOR_KINDS = [ConnectorKind.SOURCE, ConnectorKind.SINK];

export interface UseFilamentCommandItemsInput {
  base?: string;
  transport?: Transport;
}

export const useFilamentCommandItems = ({
  base,
  transport,
}: UseFilamentCommandItemsInput = {}): CommandPaletteItem[] => {
  const navigate = useNavigate();
  const mountedBase = useFilamentMountedBase();
  const openFlow = useFilamentFlowOpen();
  const updateSearch = useFilamentSearchUpdate<FilamentLayoutSearch>();

  const { data: pipelinesData } = useListPipelinesQuery({
    options: { refetchInterval: false, transport },
  });
  const { data: connectionsData } = useListConnectionsQuery({
    options: { refetchInterval: false, transport },
  });

  return useMemo(() => {
    const from = mountedBase ?? base ?? ROOT_PATH;

    const navigateFromBase = (
      to: FilamentPath,
      { params, search }: { params?: Record<string, string>; search?: FilamentLayoutSearch } = {},
    ) => void navigate({ from, to, params, search } as Parameters<typeof navigate>[0]);

    const openCreateFlow = (to: FilamentPath, flow: Flow, connectorKind?: ConnectorKind) => {
      if (mountedBase) {
        openFlow(flow, connectorKind);
        return;
      }
      navigateFromBase(to, { search: { flow, connectorKind } });
    };

    const openConnection = (connectionId: string, kind: ConnectorKind) => {
      if (mountedBase) {
        void updateSearch((prev) => ({ ...prev, connectionId }));
        return;
      }
      navigateFromBase(CONNECTOR_KIND_TO_PATH_MAP[kind], { search: { connectionId } });
    };

    return [
      ...FILAMENT_NAV_ITEMS.map((item) => ({
        id: FILAMENT_NAV_ITEM_TO_COMMAND_ID_MAP[item],
        label: FILAMENT_NAV_ITEM_TO_LABEL_MAP[item],
        icon: FILAMENT_NAV_ITEM_TO_ICON_MAP[item],
        group: FILAMENT_COMMAND_GROUP_GO_TO,
        keywords: FILAMENT_NAV_ITEM_TO_KEYWORDS_MAP[item],
        onSelect: () => navigateFromBase(FILAMENT_NAV_ITEM_TO_PATH_MAP[item]),
      })),
      {
        id: "new-pipeline",
        label: PIPELINE_CREATE_TITLE,
        icon: PlusIcon,
        group: FILAMENT_COMMAND_GROUP_CREATE,
        onSelect: () => openCreateFlow(FilamentPath.PIPELINES, Flow.CREATE_PIPELINE),
      },
      ...CREATE_CONNECTOR_KINDS.map((kind) => ({
        id: FILAMENT_CONNECTOR_KIND_TO_CREATE_COMMAND_ID_MAP[kind],
        label: CONNECTOR_KIND_TO_CREATE_TITLE_MAP[kind],
        icon: PlusIcon,
        group: FILAMENT_COMMAND_GROUP_CREATE,
        onSelect: () =>
          openCreateFlow(CONNECTOR_KIND_TO_PATH_MAP[kind], Flow.CREATE_CONNECTION, kind),
      })),
      ...(pipelinesData?.pipelines ?? []).map((pipeline) => ({
        id: `pipeline-${pipeline.id}`,
        label: formatPipelineName(pipeline),
        description: pipeline.description || undefined,
        icon: FILAMENT_NAV_ITEM_TO_ICON_MAP[FilamentNavItem.PIPELINES],
        group: FILAMENT_COMMAND_GROUP_PIPELINES,
        onSelect: () => navigateFromBase(FilamentPath.PIPELINE, { params: { id: pipeline.id } }),
      })),
      ...(connectionsData?.connections ?? []).map((connection) => ({
        id: `connection-${connection.id}`,
        label: connection.name,
        description: connection.connector,
        icon: FILAMENT_NAV_ITEM_TO_ICON_MAP[
          FILAMENT_CONNECTOR_KIND_TO_NAV_ITEM_MAP[connection.kind]
        ],
        group: FILAMENT_COMMAND_GROUP_CONNECTIONS,
        onSelect: () => openConnection(connection.id, connection.kind),
      })),
    ];
  }, [
    base,
    mountedBase,
    navigate,
    openFlow,
    updateSearch,
    pipelinesData?.pipelines,
    connectionsData?.connections,
  ]);
};
