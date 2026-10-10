import type { FC } from "react";

import SelectInput from "@galaxy-io/dls/inputs/SelectInput";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text from "@galaxy-io/dls/text/Text";

import { WriteMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import ConnectorTile from "@/pages/connectors/components/ConnectorTile";
import { CREATE_PIPELINE_MODAL_SINK_SELECT_WIDTH } from "@/pages/pipelines/components/create/constants";
import type { CreatePipelineModalSinkRow } from "@/pages/pipelines/components/create/types";
import { getWriteModeSelectOptions } from "@/pages/pipelines/components/resource/utils";

import { mapOptionIdToEnum } from "@/utils/select";

const CreatePipelineModalDeliverySink: FC<{
  sink: CreatePipelineModalSinkRow;
  onChange: (sinkId: Connection["id"], writeMode: WriteMode) => void;
}> = ({ sink, onChange }) => {
  const options = getWriteModeSelectOptions(sink.writeModeOptions);

  const handleWriteModeChange = (id: string | null) => {
    if (id === null) return;
    onChange(sink.connection.id, mapOptionIdToEnum(WriteMode, id));
  };

  return (
    <Flex
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.SPACE_BETWEEN}
      gap={16}
      padding={[12, 16]}
      fillWidth
    >
      <Flex alignItems={AlignItems.CENTER} gap={8}>
        <ConnectorTile connector={sink.connection.connector} kind={sink.connection.kind} />
        <FlexItem minWidth={0} overflow="hidden">
          <Text lineClamp={1}>{sink.connection.name}</Text>
        </FlexItem>
      </Flex>
      <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} grow={0} shrink={0}>
        <Box width={CREATE_PIPELINE_MODAL_SINK_SELECT_WIDTH}>
          <SelectInput
            fillWidth
            options={options}
            value={String(sink.writeMode)}
            onChange={handleWriteModeChange}
          />
        </Box>
      </Flex>
    </Flex>
  );
};

export default CreatePipelineModalDeliverySink;
