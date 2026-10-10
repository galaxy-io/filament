import type { FC } from "react";

import { WarningIcon } from "@phosphor-icons/react";

import Tabs, { type TabItem, TabsSize } from "@galaxy-io/dls/navigation/Tabs";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import ConnectorTile from "@/components/connections/ConnectorTile";
import { ConnectorTileSize } from "@/components/connections/types";

import {
  useCreatePipelineModalActions,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";

const CreatePipelineModalResourcesTabs: FC = () => {
  const { sinks, activeSinkId, selectedCountBySink, issuesBySink } = useCreatePipelineModalState();
  const { setActiveSink } = useCreatePipelineModalActions();

  const items: TabItem<Connection["id"]>[] = sinks.map((sink) => ({
    id: sink.connection.id,
    label: sink.connection.name,
    leading: (
      <ConnectorTile
        connector={sink.connection.connector}
        kind={sink.connection.kind}
        size={ConnectorTileSize.SMALL}
      />
    ),
    icon: issuesBySink[sink.connection.id]?.length ? WarningIcon : undefined,
    count: selectedCountBySink[sink.connection.id] ?? 0,
  }));

  return (
    <Tabs
      ariaLabel="Sinks"
      size={TabsSize.MEDIUM}
      items={items}
      value={activeSinkId}
      onChange={(sinkId) => setActiveSink(sinkId)}
      inset={8}
    />
  );
};

export default CreatePipelineModalResourcesTabs;
