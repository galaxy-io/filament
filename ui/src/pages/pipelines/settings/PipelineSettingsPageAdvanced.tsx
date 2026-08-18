import { useState } from "react";

import { create } from "@bufbuild/protobuf";
import { useParams } from "@tanstack/react-router";

import Accordion from "@galaxy-io/dls/accordion/Accordion";
import Button from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, {
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import {
  WorkerConfigurationSchema,
  type WorkerResources,
  WorkerResourcesSchema,
} from "@/gen/ingestion/v1/common_pb";
import {
  GetPipelineRequestSchema,
  UpdatePipelineRequestSchema,
} from "@/gen/ingestion/v1/pipelines_pb";

import PipelineWorkerResourcesFields from "@/pages/pipelines/components/worker/PipelineWorkerResourcesFields";

import { useSuspenseGetPipelineQuery, useUpdatePipelineMutation } from "@/api/queries/pipelines";

import { getErrorMessage } from "@/utils/errors";

const workerResourcesEqual = (left: WorkerResources, right: WorkerResources | undefined) =>
  left.cpuRequest.trim() === (right?.cpuRequest ?? "") &&
  left.cpuLimit.trim() === (right?.cpuLimit ?? "") &&
  left.memoryRequest.trim() === (right?.memoryRequest ?? "") &&
  left.memoryLimit.trim() === (right?.memoryLimit ?? "");

const PipelineSettingsPageAdvanced = () => {
  const { showToast } = useToast();
  const { id } = useParams({ from: "/pipelines/$id" });
  const { data } = useSuspenseGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id }),
  });
  const pipeline = data.pipeline;
  const { mutate: updatePipeline, isPending: isSaving } = useUpdatePipelineMutation();
  const [workerResources, setWorkerResources] = useState(() =>
    create(WorkerResourcesSchema, pipeline?.workerConfiguration?.resources),
  );

  if (!pipeline) return null;

  const hasChanges = !workerResourcesEqual(
    workerResources,
    pipeline.workerConfiguration?.resources,
  );

  const handleSave = () => {
    updatePipeline(
      create(UpdatePipelineRequestSchema, {
        tenantId: pipeline.tenantId,
        pipelineId: pipeline.id,
        name: pipeline.name,
        description: pipeline.description,
        workerConfiguration: create(WorkerConfigurationSchema, {
          resources: create(WorkerResourcesSchema, {
            cpuRequest: workerResources.cpuRequest.trim(),
            cpuLimit: workerResources.cpuLimit.trim(),
            memoryRequest: workerResources.memoryRequest.trim(),
            memoryLimit: workerResources.memoryLimit.trim(),
          }),
        }),
      }),
      {
        onSuccess: () => {
          showToast({
            header: "Configuration saved",
            subheader: "Your worker configuration has been saved successfully.",
            variant: ToastVariant.SUCCESS,
          });
        },
        onError: (error) => {
          showToast({
            header: "Save failed",
            subheader: getErrorMessage(error, "Failed to save worker configuration"),
            variant: ToastVariant.ERROR,
          });
        },
      },
    );
  };

  return (
    <Accordion header="Worker resources">
      <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} fillWidth>
        <PipelineWorkerResourcesFields
          state={workerResources}
          onChange={(partial) => setWorkerResources((current) => ({ ...current, ...partial }))}
        />
        <FlexWrapper justifyContent={JustifyContent.END} fillWidth>
          <Button label="Save" isDisabled={!hasChanges} isLoading={isSaving} onClick={handleSave} />
        </FlexWrapper>
      </FlexWrapper>
    </Accordion>
  );
};

export default PipelineSettingsPageAdvanced;
