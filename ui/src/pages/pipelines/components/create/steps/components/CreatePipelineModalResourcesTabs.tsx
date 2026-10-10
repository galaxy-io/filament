import type { FC } from "react";

import { WarningIcon } from "@phosphor-icons/react";

import Box from "@galaxy-io/dls/layout/Box";
import Tabs, { type TabItem, TabsSize } from "@galaxy-io/dls/navigation/Tabs";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";

const CreatePipelineModalResourcesTabs: FC = () => {
  const { sinks, activeSinkId, selectedCountBySink, issuesBySink } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  const items: TabItem<Connection["id"]>[] = sinks.map((sink) => ({
    id: sink.connection.id,
    label: sink.connection.name,
    icon: issuesBySink[sink.connection.id]?.length ? WarningIcon : undefined,
    count: selectedCountBySink[sink.connection.id] ?? 0,
  }));

  return (
    <Box padding={[0, 8]} fillWidth>
      <Tabs
        ariaLabel="Sinks"
        size={TabsSize.MEDIUM}
        items={items}
        value={activeSinkId}
        onChange={(sinkId) =>
          dispatch({ type: CreatePipelineModalActionType.SET_ACTIVE_SINK, payload: sinkId })
        }
      />
    </Box>
  );
};

export default CreatePipelineModalResourcesTabs;
