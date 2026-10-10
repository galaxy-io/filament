import type { FC } from "react";

import SelectInput from "@galaxy-io/dls/inputs/SelectInput";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextWeight } from "@galaxy-io/dls/text/Text";
import Widget from "@galaxy-io/dls/widget/Widget";

import { ConnectorKind, WriteMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import ConnectorTile from "@/components/connections/ConnectorTile";
import { CONNECTOR_KIND_TO_LABEL_MAP } from "@/components/connections/constants";
import { ConnectorTileSize } from "@/components/connections/types";

import {
  useCreatePipelineModalActions,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import { CREATE_PIPELINE_MODAL_SINK_SELECT_WIDTH } from "@/pages/pipelines/components/create/constants";
import type { CreatePipelineModalSinkRow } from "@/pages/pipelines/components/create/types";
import PipelineNodeConfigFields, {
  usePipelineNodeConfig,
} from "@/pages/pipelines/components/node/PipelineNodeConfigFields";
import { getWriteModeSelectOptions } from "@/pages/pipelines/components/resource/utils";

import { formatIdentifier } from "@/utils/naming";
import { mapOptionIdToEnum } from "@/utils/select";

interface CreatePipelineModalDeliveryConnectionProps {
  connection: Connection;
  kind: ConnectorKind;
  sink?: CreatePipelineModalSinkRow;
}

const CreatePipelineModalDeliveryConnection: FC<CreatePipelineModalDeliveryConnectionProps> = ({
  connection,
  kind,
  sink,
}) => {
  const { nodeConfigs, sourceConnection } = useCreatePipelineModalState();
  const { setNodeConfig, setSinkWriteMode } = useCreatePipelineModalActions();

  const config = nodeConfigs[connection.id] ?? {};
  const defaultSchema =
    kind === ConnectorKind.SINK
      ? formatIdentifier(sourceConnection?.name ?? "") || undefined
      : undefined;
  const nodeConfig = usePipelineNodeConfig(connection, kind, config, defaultSchema);

  const handleWriteModeChange = (id: string | null) => {
    if (id === null) return;
    setSinkWriteMode({ sinkId: connection.id, writeMode: mapOptionIdToEnum(WriteMode, id) });
  };

  const header = (
    <Flex alignItems={AlignItems.CENTER} gap={8} minWidth={0}>
      <ConnectorTile connector={connection.connector} kind={kind} size={ConnectorTileSize.SMALL} />
      <FlexItem minWidth={0}>
        <Text weight={TextWeight.MEDIUM} lineClamp={1}>
          {connection.name}
        </Text>
      </FlexItem>
    </Flex>
  );

  const actions = sink && (
    <Box width={CREATE_PIPELINE_MODAL_SINK_SELECT_WIDTH}>
      <SelectInput
        ariaLabel={`Write mode for ${connection.name}`}
        options={getWriteModeSelectOptions(sink.writeModeOptions)}
        value={String(sink.writeMode)}
        onChange={handleWriteModeChange}
        fillWidth
      />
    </Box>
  );

  if (nodeConfig.fields.length === 0) {
    return (
      <Widget header={header} subheader={CONNECTOR_KIND_TO_LABEL_MAP[kind]} actions={actions} />
    );
  }

  return (
    <Widget
      isCollapsible
      header={header}
      subheader={CONNECTOR_KIND_TO_LABEL_MAP[kind]}
      actions={actions}
    >
      <PipelineNodeConfigFields
        {...nodeConfig}
        config={config}
        onChange={(payload) => setNodeConfig({ connectionId: connection.id, config: payload })}
      />
    </Widget>
  );
};

export default CreatePipelineModalDeliveryConnection;
