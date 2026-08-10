import { Fragment } from "react";

import FlexWrapper, {
  FlexDirection,
  FlexGap,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Text, {
  TextSize,
  TextVariant,
  TextWeight,
} from "@galaxy-io/dls/text/Text";
import Widget from "@galaxy-io/dls/widget/Widget";

import {
  useCreatePipelineModalActions,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import CreatePipelineModalDeliverySink from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliverySink";
import CreatePipelineModalSchedule from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalSchedule";

interface CreatePipelineModalDeliverySectionProps {
  header: string;
  children: React.ReactNode;
}

const CreatePipelineModalDeliverySection = ({
  header,
  children,
}: CreatePipelineModalDeliverySectionProps) => (
  <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} fillWidth>
    <FlexWrapper direction={FlexDirection.COLUMN} gap={2} fillWidth>
      <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM}>
        {header}
      </Text>
    </FlexWrapper>
    {children}
  </FlexWrapper>
);

const CreatePipelineModalDelivery = () => {
  const { sinks, isCdc } = useCreatePipelineModalState();
  const { setSinkWriteMode } = useCreatePipelineModalActions();

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.LARGE} fillWidth>
      {!isCdc && sinks.length > 0 && (
        <CreatePipelineModalDeliverySection header="Destinations">
          <Widget noPadding noHover fillWidth>
            <FlexWrapper direction={FlexDirection.COLUMN} fillWidth>
              {sinks.map((sink, index) => (
                <Fragment key={sink.connection.id}>
                  {index > 0 && <HorizontalDivider />}
                  <CreatePipelineModalDeliverySink
                    sink={sink}
                    onChange={setSinkWriteMode}
                  />
                </Fragment>
              ))}
            </FlexWrapper>
          </Widget>
        </CreatePipelineModalDeliverySection>
      )}
      <CreatePipelineModalDeliverySection header="Schedule">
        <CreatePipelineModalSchedule />
      </CreatePipelineModalDeliverySection>
    </FlexWrapper>
  );
};

export default CreatePipelineModalDelivery;
