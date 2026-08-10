import { Fragment } from "react";

import FlexWrapper, { FlexDirection, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget from "@galaxy-io/dls/widget/Widget";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import CreatePipelineModalDeliverySink from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliverySink";
import PipelineScheduleFields from "@/pages/pipelines/components/schedule/PipelineScheduleFields";
import { formatPipelineScheduleSummary } from "@/pages/pipelines/settings/utils";

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

const CreatePipelineModalDeliverySchedule = () => {
  const { schedule } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  const summary = formatPipelineScheduleSummary(schedule);

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} fillWidth>
      <PipelineScheduleFields
        state={schedule}
        onChange={(partial) =>
          dispatch({ type: CreatePipelineModalActionType.SET_SCHEDULE, payload: partial })
        }
      />
      {schedule.isEnabled && summary && (
        <Text size={TextSize.BODY_SM} variant={TextVariant.SUCCESS}>
          {summary}
        </Text>
      )}
    </FlexWrapper>
  );
};

const CreatePipelineModalDelivery = () => {
  const { sinks, isCdc } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

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
                  <CreatePipelineModalDeliverySink
                    sink={sink}
                    onChange={(sinkId, writeMode) =>
                      dispatch({
                        type: CreatePipelineModalActionType.SET_SINK_WRITE_MODE,
                        payload: { sinkId, writeMode },
                      })
                    }
                  />
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
        <CreatePipelineModalDeliverySchedule />
      </CreatePipelineModalDeliverySection>
    </FlexWrapper>
  );
};

export default CreatePipelineModalDelivery;
