import type { FC } from "react";

import { ChipSize } from "@galaxy-io/dls/chips/Chip";
import SelectInput from "@galaxy-io/dls/inputs/SelectInput";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextWeight } from "@galaxy-io/dls/text/Text";
import Widget from "@galaxy-io/dls/widget/Widget";

import { ConnectorKind, WriteMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import ConnectionKindChip from "@/components/connections/ConnectionKindChip";
import ConnectorTile from "@/components/connections/ConnectorTile";
import { ConnectorTileSize } from "@/components/connections/types";

import {
  useCreatePipelineModalActions,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import {
  CREATE_PIPELINE_MODAL_CONNECTION_CARET_WIDTH,
  CREATE_PIPELINE_MODAL_CONNECTION_ROW_HEIGHT,
} from "@/pages/pipelines/components/create/constants";
import type { CreatePipelineModalSinkRow } from "@/pages/pipelines/components/create/types";
import { usePipelineNodeConfig } from "@/pages/pipelines/components/node/hooks/usePipelineNodeConfig";
import PipelineNodeConfigFields from "@/pages/pipelines/components/node/PipelineNodeConfigFields";
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

  const title = (
    <Flex alignItems={AlignItems.CENTER} gap={8} minWidth={0}>
      <ConnectorTile connector={connection.connector} kind={kind} size={ConnectorTileSize.SMALL} />
      <FlexItem minWidth={0}>
        <Text weight={TextWeight.MEDIUM} lineClamp={1}>
          {connection.name}
        </Text>
      </FlexItem>
    </Flex>
  );

  const kindChip = <ConnectionKindChip kind={kind} size={ChipSize.SMALL} />;

  if (!sink && nodeConfig.fields.length === 0) {
    return (
      <Widget isFlush>
        <Flex
          alignItems={AlignItems.CENTER}
          gap={8}
          padding={[0, 12]}
          minHeight={CREATE_PIPELINE_MODAL_CONNECTION_ROW_HEIGHT}
          fillWidth
        >
          <FlexItem grow={1} minWidth={0}>
            {title}
          </FlexItem>
          {kindChip}
          <Box width={CREATE_PIPELINE_MODAL_CONNECTION_CARET_WIDTH} />
        </Flex>
      </Widget>
    );
  }

  return (
    <Widget isCollapsible header={title} actions={kindChip} defaultIsOpen>
      {sink && (
        <SelectInput
          label="Write mode"
          options={getWriteModeSelectOptions(sink.writeModeOptions)}
          value={String(sink.writeMode)}
          onChange={handleWriteModeChange}
          fillWidth
        />
      )}
      {nodeConfig.fields.length > 0 && (
        <PipelineNodeConfigFields
          {...nodeConfig}
          config={config}
          onChange={(payload) => setNodeConfig({ connectionId: connection.id, config: payload })}
        />
      )}
    </Widget>
  );
};

export default CreatePipelineModalDeliveryConnection;
