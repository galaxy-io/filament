import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";

import {
  usePipelineTransformFieldsEnvironment,
  usePipelineTransformFieldsState,
} from "@/components/transform/PipelineTransformFieldsProvider";
import PipelineTransformFieldsResourceSteps from "@/components/transform/PipelineTransformFieldsResourceSteps";
import PipelineTransformFieldsStepCard from "@/components/transform/PipelineTransformFieldsStepCard";

const PipelineTransformFields = () => {
  const { stepsByResource, draft } = usePipelineTransformFieldsState();
  const { resources } = usePipelineTransformFieldsEnvironment();
  const hasSteps = [...stepsByResource.values()].some((steps) => steps.length > 0);
  let hasPrecedingSteps = false;

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} fillWidth>
      {resources.map((resource) => {
        const hasDivider = hasPrecedingSteps;
        if ((stepsByResource.get(resource)?.length ?? 0) > 0) hasPrecedingSteps = true;
        return (
          <PipelineTransformFieldsResourceSteps
            key={resource}
            resource={resource}
            hasDivider={hasDivider}
          />
        );
      })}
      {draft?.id === null && (
        <>
          {hasSteps && <HorizontalDivider />}
          <PipelineTransformFieldsStepCard draft={draft} />
        </>
      )}
    </FlexWrapper>
  );
};

export default PipelineTransformFields;
