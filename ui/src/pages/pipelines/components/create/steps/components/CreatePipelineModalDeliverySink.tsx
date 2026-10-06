import { styled } from "@linaria/react";

import SelectInput, { SelectInputSize, type SelectOption } from "@galaxy-io/dls/inputs/SelectInput";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Text from "@galaxy-io/dls/text/Text";

import type { WriteMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import ConnectorTile from "@/pages/connectors/components/ConnectorTile";
import {
  CREATE_PIPELINE_MODAL_SINK_SELECT_WIDTH,
  WRITE_MODE_TO_LABEL_MAP,
} from "@/pages/pipelines/components/create/constants";
import type { CreatePipelineModalSinkRow } from "@/pages/pipelines/components/create/types";

const SinkNameWrapper = styled.div`
  min-width: 0;
  overflow: hidden;
`;

const CreatePipelineModalDeliverySink = ({
  sink,
  onChange,
}: {
  sink: CreatePipelineModalSinkRow;
  onChange: (sinkId: Connection["id"], writeMode: WriteMode) => void;
}) => {
  const options: SelectOption[] = sink.writeModeOptions.map((mode) => ({
    id: String(mode),
    label: WRITE_MODE_TO_LABEL_MAP[mode],
    value: mode,
  }));

  return (
    <Flex
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.SPACE_BETWEEN}
      gap={16}
      padding={[12, 16]}
      fillWidth
    >
      <Flex
        alignItems={
          AlignItems.CENTER
        } /* @dls-migrate layout.off-scale: Pick a value on the space scale (or a CSS-order tuple of them). */
        gap={10}
      >
        <ConnectorTile connector={sink.connection.connector} kind={sink.connection.kind} />
        <SinkNameWrapper>
          <Text lineClamp={1}>{sink.connection.name}</Text>
        </SinkNameWrapper>
      </Flex>
      <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} grow={0} shrink={0}>
        <Box width={CREATE_PIPELINE_MODAL_SINK_SELECT_WIDTH}>
          <SelectInput
            fillWidth
            options={options}
            /* @dls-migrate selectinput.value: `value` and `onChange` now carry option ids, not option objects. */ value={
              options.find((option) => option.value === sink.writeMode) ?? null
            }
            onChange={(option) => onChange(sink.connection.id, option.value as WriteMode)}
            size={SelectInputSize.LARGE}
          />
        </Box>
      </Flex>
    </Flex>
  );
};

export default CreatePipelineModalDeliverySink;
