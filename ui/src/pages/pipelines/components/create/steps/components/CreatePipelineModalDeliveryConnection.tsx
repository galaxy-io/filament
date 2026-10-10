import { type FC, useState } from "react";

import { CaretDownIcon, CaretUpIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { ChipSize } from "@galaxy-io/dls/chips/Chip";
import SelectInput from "@galaxy-io/dls/inputs/SelectInput";
import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextWeight } from "@galaxy-io/dls/text/Text";

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
  CREATE_PIPELINE_MODAL_CONNECTION_ROW_HEIGHT,
  CREATE_PIPELINE_MODAL_CONNECTION_TOGGLE_WIDTH,
  CREATE_PIPELINE_MODAL_SINK_SELECT_WIDTH,
} from "@/pages/pipelines/components/create/constants";
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

interface CreatePipelineModalDeliveryConnectionState {
  isOpen: boolean;
}

const DEFAULT_STATE: CreatePipelineModalDeliveryConnectionState = {
  isOpen: false,
};

const CreatePipelineModalDeliveryConnection: FC<CreatePipelineModalDeliveryConnectionProps> = ({
  connection,
  kind,
  sink,
}) => {
  const { nodeConfigs, sourceConnection } = useCreatePipelineModalState();
  const { setNodeConfig, setSinkWriteMode } = useCreatePipelineModalActions();
  const [state, setState] = useState<CreatePipelineModalDeliveryConnectionState>(DEFAULT_STATE);

  const config = nodeConfigs[connection.id] ?? {};
  const defaultSchema =
    kind === ConnectorKind.SINK
      ? formatIdentifier(sourceConnection?.name ?? "") || undefined
      : undefined;
  const nodeConfig = usePipelineNodeConfig(connection, kind, config, defaultSchema);
  const hasOptions = nodeConfig.fields.length > 0;
  const isOpen = hasOptions && state.isOpen;

  const handleToggle = () => {
    setState((prev) => ({ ...prev, isOpen: !prev.isOpen }));
  };

  const handleWriteModeChange = (id: string | null) => {
    if (id === null) return;
    setSinkWriteMode({ sinkId: connection.id, writeMode: mapOptionIdToEnum(WriteMode, id) });
  };

  return (
    <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} fillWidth>
      <Flex
        alignItems={AlignItems.CENTER}
        gap={8}
        padding={[0, 12]}
        minHeight={CREATE_PIPELINE_MODAL_CONNECTION_ROW_HEIGHT}
        fillWidth
      >
        <ConnectorTile
          connector={connection.connector}
          kind={kind}
          size={ConnectorTileSize.SMALL}
        />
        <ConnectionKindChip kind={kind} size={ChipSize.SMALL} />
        <FlexItem grow={1} minWidth={0}>
          <Text weight={TextWeight.MEDIUM} lineClamp={1}>
            {connection.name}
          </Text>
        </FlexItem>
        {sink && (
          <Box width={CREATE_PIPELINE_MODAL_SINK_SELECT_WIDTH}>
            <SelectInput
              ariaLabel={`Write mode for ${connection.name}`}
              options={getWriteModeSelectOptions(sink.writeModeOptions)}
              value={String(sink.writeMode)}
              onChange={handleWriteModeChange}
              fillWidth
            />
          </Box>
        )}
        <Box width={CREATE_PIPELINE_MODAL_CONNECTION_TOGGLE_WIDTH}>
          {hasOptions && (
            <Button
              icon={isOpen ? CaretUpIcon : CaretDownIcon}
              ariaLabel={`${isOpen ? "Hide" : "Show"} ${connection.name} options`}
              tooltip={isOpen ? "Hide options" : "Show options"}
              variant={ButtonVariant.TERTIARY}
              size={ButtonSize.SMALL}
              onClick={handleToggle}
            />
          )}
        </Box>
      </Flex>
      {isOpen && (
        <>
          <Divider />
          <Box padding={12} fillWidth>
            <PipelineNodeConfigFields
              {...nodeConfig}
              config={config}
              onChange={(payload) =>
                setNodeConfig({ connectionId: connection.id, config: payload })
              }
            />
          </Box>
        </>
      )}
    </Flex>
  );
};

export default CreatePipelineModalDeliveryConnection;
