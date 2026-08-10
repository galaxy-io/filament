import { styled } from "@linaria/react";

import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import Text from "@galaxy-io/dls/text/Text";

import type { WriteMode } from "@/gen/ingestion/v1/common_pb";

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

interface CreatePipelineModalDeliverySinkProps {
  sink: CreatePipelineModalSinkRow;
  onChange: (sinkId: string, writeMode: WriteMode) => void;
}

const CreatePipelineModalDeliverySink = ({
  sink,
  onChange,
}: CreatePipelineModalDeliverySinkProps) => {
  const options: SelectInputOption[] = sink.writeModeOptions.map((mode) => ({
    id: String(mode),
    label: WRITE_MODE_TO_LABEL_MAP[mode],
    value: mode,
  }));

  const selectedOption = options.find((option) => option.value === sink.writeMode) ?? null;

  return (
    <FlexWrapper
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.SPACE_BETWEEN}
      gap={16}
      padding="12px"
      fillWidth
    >
      <FlexWrapper alignItems={AlignItems.CENTER} gap={10}>
        <ConnectorTile connector={sink.connection.connector} />
        <SinkNameWrapper>
          <Text isEllipsis>{sink.connection.name}</Text>
        </SinkNameWrapper>
      </FlexWrapper>
      <FlexWrapper direction={FlexDirection.COLUMN} grow={0} shrink={0}>
        <SelectInput
          options={options}
          value={selectedOption}
          onChange={(option) => onChange(sink.connection.id, option.value as WriteMode)}
          size={InputSize.LARGE}
          width={CREATE_PIPELINE_MODAL_SINK_SELECT_WIDTH}
        />
      </FlexWrapper>
    </FlexWrapper>
  );
};

export default CreatePipelineModalDeliverySink;
