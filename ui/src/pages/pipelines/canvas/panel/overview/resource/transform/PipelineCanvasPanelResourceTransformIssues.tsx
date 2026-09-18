import { create } from "@bufbuild/protobuf";

import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { Resource } from "@/gen/ingestion/v1/connectors_pb";
import { ValidateTransformRequestSchema } from "@/gen/ingestion/v1/transformations_pb";

import type { TransformStep } from "@/pages/pipelines/canvas/panel/overview/resource/transform/types";
import {
  formatTransformStepSummary,
  getTransformIssueStepId,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/utils";
import type { PipelineCanvasEdgeTransform } from "@/pages/pipelines/canvas/types";

import { useValidateTransformQuery } from "@/api/queries/transforms";

interface PipelineCanvasPanelResourceTransformIssuesProps {
  sourceConnectionId: Connection["id"];
  resource: Resource["name"];
  definition: PipelineCanvasEdgeTransform;
  steps: TransformStep[];
}

/**
 * Compiles one resource's definition against its live schema and lists what
 * the compiler rejects, each issue labelled with the step it sits on.
 */
const PipelineCanvasPanelResourceTransformIssues = ({
  sourceConnectionId,
  resource,
  definition,
  steps,
}: PipelineCanvasPanelResourceTransformIssuesProps) => {
  const { data } = useValidateTransformQuery({
    input: create(ValidateTransformRequestSchema, {
      sourceConnectionId,
      resource,
      transform: definition,
    }),
    options: { enabled: sourceConnectionId !== "" },
  });
  const issues = data?.issues ?? [];
  if (issues.length === 0) return null;

  const stepsById = new Map(steps.map((step) => [step.id, step]));

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={4} padding="8px 12px" fillWidth>
      {issues.map((issue) => {
        const stepId = getTransformIssueStepId(issue.field);
        const step = stepId ? stepsById.get(stepId) : undefined;
        const label = step ? formatTransformStepSummary(step) : issue.field;
        return (
          <Text
            key={`${issue.field}:${issue.message}`}
            size={TextSize.BODY_SM}
            variant={TextVariant.ERROR}
          >
            {label}: {issue.message}
          </Text>
        );
      })}
    </FlexWrapper>
  );
};

export default PipelineCanvasPanelResourceTransformIssues;
