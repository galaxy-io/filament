import { Fragment } from "react";

import FlexWrapper, { FlexDirection, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget from "@galaxy-io/dls/widget/Widget";

import type { WriteMode } from "@/gen/ingestion/v1/common_pb";

import CreatePipelineModalDeliverySink from "@/pages/pipelines/components/create/CreatePipelineModalDeliverySink";
import CreatePipelineModalSchedule from "@/pages/pipelines/components/create/CreatePipelineModalSchedule";
import type { CreatePipelineModalSinkRow } from "@/pages/pipelines/components/create/types";
import type { PipelineSettingsPageScheduleState } from "@/pages/pipelines/settings/types";

interface CreatePipelineModalDeliverySectionProps {
  header: string;
  subheader: string;
  children: React.ReactNode;
}

const CreatePipelineModalDeliverySection = ({
  header,
  subheader,
  children,
}: CreatePipelineModalDeliverySectionProps) => (
  <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} fillWidth>
    <FlexWrapper direction={FlexDirection.COLUMN} gap={2} fillWidth>
      <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM}>
        {header}
      </Text>
      <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
        {subheader}
      </Text>
    </FlexWrapper>
    {children}
  </FlexWrapper>
);

interface CreatePipelineModalDeliveryProps {
  sinks: CreatePipelineModalSinkRow[];
  isCdc: boolean;
  schedule: PipelineSettingsPageScheduleState;
  onSinkWriteModeChange: (sinkId: string, writeMode: WriteMode) => void;
  onScheduleChange: (partial: Partial<PipelineSettingsPageScheduleState>) => void;
}

const CreatePipelineModalDelivery = ({
  sinks,
  isCdc,
  schedule,
  onSinkWriteModeChange,
  onScheduleChange,
}: CreatePipelineModalDeliveryProps) => {
  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.LARGE} fillWidth>
      {!isCdc && sinks.length > 0 && (
        <CreatePipelineModalDeliverySection
          header="Destinations"
          subheader="How each destination lands the records it receives"
        >
          <Widget noPadding noHover fillWidth>
            <FlexWrapper direction={FlexDirection.COLUMN} fillWidth>
              {sinks.map((sink, index) => (
                <Fragment key={sink.connection.id}>
                  {index > 0 && <HorizontalDivider />}
                  <CreatePipelineModalDeliverySink sink={sink} onChange={onSinkWriteModeChange} />
                </Fragment>
              ))}
            </FlexWrapper>
          </Widget>
        </CreatePipelineModalDeliverySection>
      )}
      <CreatePipelineModalDeliverySection
        header="Schedule"
        subheader="When the pipeline runs on its own. Leave it off to run manually."
      >
        <CreatePipelineModalSchedule schedule={schedule} onScheduleChange={onScheduleChange} />
      </CreatePipelineModalDeliverySection>
    </FlexWrapper>
  );
};

export default CreatePipelineModalDelivery;
