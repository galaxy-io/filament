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
  GetPipelineRequestSchema,
  UpdatePipelineRequestSchema,
} from "@/gen/ingestion/v1/pipelines_pb";

import PipelineWorkerConfigurationEditor from "@/pages/pipelines/components/worker/PipelineWorkerConfigurationEditor";
import {
  formatWorkerConfiguration,
  parseWorkerConfiguration,
  workerConfigurationEquals,
} from "@/pages/pipelines/components/worker/utils";

import { useSuspenseGetPipelineQuery, useUpdatePipelineMutation } from "@/api/queries/pipelines";

import { getErrorMessage } from "@/utils/errors";

const PipelineSettingsPageAdvanced = () => {
  const { showToast } = useToast();
  const { id } = useParams({ from: "/pipelines/$id" });
  const { data } = useSuspenseGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id }),
  });
  const pipeline = data.pipeline;
  const { mutate: updatePipeline, isPending: isSaving } = useUpdatePipelineMutation();
  const [workerConfiguration, setWorkerConfiguration] = useState(() =>
    formatWorkerConfiguration(pipeline?.workerConfiguration),
  );

  if (!pipeline) return null;

  const parsed = parseWorkerConfiguration(workerConfiguration);
  const hasChanges = !workerConfigurationEquals(parsed.configuration, pipeline.workerConfiguration);
  const canSave = hasChanges && !parsed.error;

  const handleSave = () => {
    updatePipeline(
      create(UpdatePipelineRequestSchema, {
        tenantId: pipeline.tenantId,
        pipelineId: pipeline.id,
        name: pipeline.name,
        description: pipeline.description,
        workerConfiguration: parsed.configuration,
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
    <Accordion header="Worker configuration">
      <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} fillWidth>
        <PipelineWorkerConfigurationEditor
          value={workerConfiguration}
          onChange={setWorkerConfiguration}
          error={parsed.error}
        />
        <FlexWrapper justifyContent={JustifyContent.END} fillWidth>
          <Button label="Save" isDisabled={!canSave} isLoading={isSaving} onClick={handleSave} />
        </FlexWrapper>
      </FlexWrapper>
    </Accordion>
  );
};

export default PipelineSettingsPageAdvanced;
