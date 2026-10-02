import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";

import {
  usePipelineTransformFieldsEnvironment,
  usePipelineTransformFieldsState,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import PipelineTransformFieldsResourceSteps from "@/pages/pipelines/components/transform/PipelineTransformFieldsResourceSteps";
import PipelineTransformFieldsStepCard from "@/pages/pipelines/components/transform/PipelineTransformFieldsStepCard";

const PipelineTransformFields = () => {
  const { stepsByResource, draft } = usePipelineTransformFieldsState();
  const { resources } = usePipelineTransformFieldsEnvironment();
  const hasSteps = (resource: string) => (stepsByResource.get(resource)?.length ?? 0) > 0;

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} fillWidth>
      {resources.map((resource, index) => (
        <PipelineTransformFieldsResourceSteps
          key={resource}
          resource={resource}
          hasDivider={resources.slice(0, index).some(hasSteps)}
        />
      ))}
      {draft?.id === null && (
        <>
          {resources.some(hasSteps) && <HorizontalDivider />}
          <PipelineTransformFieldsStepCard draft={draft} />
        </>
      )}
    </FlexWrapper>
  );
};

export default PipelineTransformFields;
