import Widget, { WidgetSize } from "@galaxy-io/dls/widget/Widget";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import PipelineNodeConfigFields, {
  usePipelineNodeConfig,
} from "@/pages/pipelines/components/node/PipelineNodeConfigFields";

import { normalizeIdentifier } from "@/utils/naming";

interface CreatePipelineModalDeliveryNodeConfigProps {
  header: string;
  connection: Connection;
  kind: ConnectorKind;
}

const CreatePipelineModalDeliveryNodeConfig = ({
  header,
  connection,
  kind,
}: CreatePipelineModalDeliveryNodeConfigProps) => {
  const { nodeConfigs, sourceConnection } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();
  const config = nodeConfigs[connection.id] ?? {};
  const defaultSchema =
    kind === ConnectorKind.SINK
      ? normalizeIdentifier(sourceConnection?.name ?? "") || undefined
      : undefined;
  const nodeConfig = usePipelineNodeConfig(connection, kind, config, defaultSchema);

  if (nodeConfig.fields.length === 0) return null;

  return (
    <Widget
      isCollapsible
      header={
        header
      } /* @dls-migrate accordion.padding-other: The body inset follows `size`: remove `padding` (use `isFlush` for 0). */
      padding="16px"
      size={WidgetSize.LARGE}
    >
      <PipelineNodeConfigFields
        {...nodeConfig}
        config={config}
        onChange={(payload) =>
          dispatch({
            type: CreatePipelineModalActionType.SET_NODE_CONFIG,
            payload: { connectionId: connection.id, config: payload },
          })
        }
      />
    </Widget>
  );
};

export default CreatePipelineModalDeliveryNodeConfig;
