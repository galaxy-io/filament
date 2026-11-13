import { type FC, useState } from "react";

import { create } from "@bufbuild/protobuf";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import CheckboxInput from "@galaxy-io/dls/inputs/CheckboxInput";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";
import Widget from "@galaxy-io/dls/widget/Widget";

import { ValidatePipelineRequestSchema } from "@/gen/ingestion/v1/capabilities_pb";
import { ExecutionMode, ReplicationMode } from "@/gen/ingestion/v1/common_pb";
import { UpdatePipelineRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";

import { usePipelineParams } from "@/module/hooks";

import { useValidatePipelineQuery } from "@/api/queries/capabilities";
import {
  createGetPipelineInput,
  useSuspenseGetPipelineQuery,
  useUpdatePipelineMutation,
} from "@/api/queries/pipelines";

import { getErrorMessage } from "@/utils/errors";

const PipelineSettingsPageResourceFailures: FC = () => {
  const { id } = usePipelineParams();
  const { toast } = useToast();
  const { data } = useSuspenseGetPipelineQuery({ input: createGetPipelineInput(id) });
  const { mutate: updatePipeline, isPending: isSaving } = useUpdatePipelineMutation();
  const pipeline = data.pipeline;
  const { data: validation, isPending: isValidating } = useValidatePipelineQuery({
    input: create(ValidatePipelineRequestSchema, {
      graph: pipeline?.currentVersion?.graph,
      executionMode: pipeline?.executionMode,
    }),
    options: { enabled: !!pipeline?.currentVersion?.graph },
  });
  const [failOnError, setFailOnError] = useState(() => pipeline?.failOnError ?? false);

  if (
    !pipeline ||
    pipeline.executionMode === ExecutionMode.CONTINUOUS ||
    (pipeline.currentVersion?.graph && isValidating) ||
    validation?.edges.some((edge) => edge.replication === ReplicationMode.CDC)
  )
    return null;

  const hasChanges = failOnError !== (pipeline.failOnError ?? false);

  const handleSave = () => {
    updatePipeline(
      create(UpdatePipelineRequestSchema, {
        pipelineId: pipeline.id,
        name: pipeline.name,
        description: pipeline.description,
        failOnError,
      }),
      {
        onSuccess: () => toast({ header: "Run behavior saved", variant: ToastVariant.SUCCESS }),
        onError: (error) =>
          toast({
            header: "Save failed",
            description: getErrorMessage(error, "Failed to save run behavior"),
            variant: ToastVariant.ERROR,
          }),
      },
    );
  };

  return (
    <Widget isCollapsible header="Resource failures">
      <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={12} fillWidth>
        <CheckboxInput
          label="Fail the run when a resource fails"
          description="Stop every resource at the first failure. When off, the other resources finish and the run ends Partial."
          isChecked={failOnError}
          onChange={setFailOnError}
        />
        <Flex justifyContent={JustifyContent.END} gap={8} fillWidth>
          <Button
            label="Cancel"
            variant={ButtonVariant.SECONDARY}
            isDisabled={!hasChanges || isSaving}
            onClick={() => setFailOnError(pipeline.failOnError ?? false)}
          />
          <Button label="Save" isDisabled={!hasChanges} isLoading={isSaving} onClick={handleSave} />
        </Flex>
      </Flex>
    </Widget>
  );
};

export default PipelineSettingsPageResourceFailures;
