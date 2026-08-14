import { Fragment } from "react";

import { SlidersHorizontalIcon } from "@phosphor-icons/react";

import Accordion from "@galaxy-io/dls/accordion/Accordion";
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
import WorkerResourcesFields from "@/pages/pipelines/components/worker/WorkerResourcesFields";
import { formatPipelineScheduleSummary } from "@/pages/pipelines/settings/utils";

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

const CreatePipelineModalDeliverySchedule = () => {
  const { schedule } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  const summary = formatPipelineScheduleSummary(schedule);

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} fillWidth>
      <PipelineScheduleFields
        state={schedule}
        onChange={(partial) =>
          dispatch({
            type: CreatePipelineModalActionType.SET_SCHEDULE,
            payload: partial,
          })
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

const CreatePipelineModalDeliveryAdvanced = () => {
  const { workerResources } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  return (
    <Accordion header="Advanced configuration" icon={SlidersHorizontalIcon}>
      <WorkerResourcesFields
        state={workerResources}
        onChange={(payload) =>
          dispatch({
            type: CreatePipelineModalActionType.SET_WORKER_RESOURCES,
            payload,
          })
        }
      />
    </Accordion>
  );
};

const CreatePipelineModalDelivery = () => {
  const { sinks, isCdc } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

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
      <CreatePipelineModalDeliverySection header="Schedule">
        <CreatePipelineModalDeliverySchedule />
      </CreatePipelineModalDeliverySection>
      <CreatePipelineModalDeliveryAdvanced />
    </FlexWrapper>
  );
};

export default CreatePipelineModalDelivery;
