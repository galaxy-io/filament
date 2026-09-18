import { useMemo, useState } from "react";

import { create } from "@bufbuild/protobuf";
import { PlusIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";

import { ReadMode, WriteMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { Resource, ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";
import { ListTransformFunctionsRequestSchema } from "@/gen/ingestion/v1/transformations_pb";

import PipelineCanvasPanelResourceTransformIssues from "@/pages/pipelines/canvas/panel/overview/resource/transform/PipelineCanvasPanelResourceTransformIssues";
import PipelineCanvasPanelResourceTransformTable from "@/pages/pipelines/canvas/panel/overview/resource/transform/PipelineCanvasPanelResourceTransformTable";
import type {
  TransformStep,
  TransformStepState,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/types";
import {
  mapDefinitionToTransformSteps,
  mapTransformStepsToDefinition,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/utils";
import PipelineCanvasPanelSection from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelSection";
import {
  usePipelineCanvasActions,
  usePipelineCanvasReadOnly,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import type { CanvasEdge } from "@/pages/pipelines/canvas/types";

import { useListTransformFunctionsQuery } from "@/api/queries/transforms";

interface PipelineCanvasPanelResourceTransformSectionProps {
  edge: CanvasEdge;
  sourceConnectionId: Connection["id"];
  resources: Resource["name"][];
  columnsByResource: Map<Resource["name"], ResourceColumn[]>;
}

/**
 * The edge's transform as a list of steps. Rules are a view over the
 * definition stored on the edge: every edit rewrites the definition, and the
 * steps are read back from it, so the canvas diff and save see one thing.
 */
const PipelineCanvasPanelResourceTransformSection = ({
  edge,
  sourceConnectionId,
  resources,
  columnsByResource,
}: PipelineCanvasPanelResourceTransformSectionProps) => {
  const isReadOnly = usePipelineCanvasReadOnly();
  const { setEdgeConfig } = usePipelineCanvasActions();
  const [isCreating, setIsCreating] = useState(false);

  const { data: catalog } = useListTransformFunctionsQuery({
    input: create(ListTransformFunctionsRequestSchema, {}),
  });
  const functionsByName = useMemo(
    () => new Map((catalog?.functions ?? []).map((fn) => [fn.name, fn])),
    [catalog?.functions],
  );

  const definition = edge.data?.transform;
  const steps = useMemo(
    () => mapDefinitionToTransformSteps(definition, resources),
    [definition, resources],
  );
  // With nothing to show, the header's Add step is the whole section: an open
  // body would render empty and its edge would read as a stray line.
  const [isOpen, setIsOpen] = useState(true);
  const hasContent = steps.length > 0 || isCreating;

  const commit = (next: TransformStep[]) =>
    setEdgeConfig(edge.id, {
      readMode: edge.data?.readMode ?? ReadMode.UNSPECIFIED,
      writeMode: edge.data?.writeMode ?? WriteMode.UNSPECIFIED,
      cursors: edge.data?.cursors ?? [],
      transform: mapTransformStepsToDefinition(next),
    });

  const handleCreate = (state: TransformStepState) =>
    commit([...steps, { ...state, id: "", index: steps.length }]);

  const handleUpdate = (step: TransformStep, state: TransformStepState) =>
    commit(
      steps.map((candidate) => (candidate.id === step.id ? { ...step, ...state } : candidate)),
    );

  const handleDelete = (step: TransformStep) =>
    commit(steps.filter((candidate) => candidate.id !== step.id));

  const transformedResources = resources.filter((resource) =>
    steps.some((step) => step.resource === resource),
  );

  return (
    <PipelineCanvasPanelSection
      header="Transformations"
      isEmpty={false}
      emptyHeader="No steps"
      emptyMessage="Columns arrive at the sink as they leave the source."
      isOpen={isOpen && hasContent}
      onToggle={() => setIsOpen((prev) => !prev)}
      headerAction={
        isReadOnly || steps.length > 0 ? undefined : (
          <Button
            label="Add step"
            icon={PlusIcon}
            variant={ButtonVariant.SECONDARY}
            size={ButtonSize.SMALL}
            onClick={(e) => {
              // The header toggles the section; the button opens it instead.
              e.stopPropagation();
              setIsOpen(true);
              setIsCreating(true);
            }}
            isDisabled={isCreating}
          />
        )
      }
    >
      <PipelineCanvasPanelResourceTransformTable
        steps={steps}
        resources={resources}
        columnsByResource={columnsByResource}
        functionsByName={functionsByName}
        onCreate={handleCreate}
        onUpdate={handleUpdate}
        onDelete={handleDelete}
        isCreating={isCreating}
        onCreatingChange={setIsCreating}
        isReadOnly={isReadOnly}
      />
      {definition &&
        transformedResources.map((resource) => (
          <PipelineCanvasPanelResourceTransformIssues
            key={resource}
            sourceConnectionId={sourceConnectionId}
            resource={resource}
            definition={definition}
            steps={steps}
          />
        ))}
    </PipelineCanvasPanelSection>
  );
};

export default PipelineCanvasPanelResourceTransformSection;
