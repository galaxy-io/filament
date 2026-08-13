import { useState } from "react";

import { create } from "@bufbuild/protobuf";
import { SlidersIcon } from "@phosphor-icons/react";
import { useParams } from "@tanstack/react-router";

import Accordion from "@galaxy-io/dls/accordion/Accordion";
import Button from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, {
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import TextAreaInput from "@galaxy-io/dls/inputs/TextAreaInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import {
  GetPipelineRequestSchema,
  type Pipeline,
  UpdatePipelineRequestSchema,
} from "@/gen/ingestion/v1/pipelines_pb";

import { formatPipelineName } from "@/pages/pipelines/utils";

import { useSuspenseGetPipelineQuery, useUpdatePipelineMutation } from "@/api/queries/pipelines";

import { getErrorMessage } from "@/utils/errors";
import { stripDeletedName } from "@/utils/format";

interface PipelineSettingsPageGeneralState {
  name: Pipeline["name"];
  description: Pipeline["description"];
}

const DEFAULT_STATE: PipelineSettingsPageGeneralState = {
  name: "",
  description: "",
};

const PipelineSettingsPageGeneral = () => {
  const { showToast } = useToast();
  const { id } = useParams({ from: "/pipelines/$id" });

  const { data } = useSuspenseGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id }),
  });
  const pipeline = data.pipeline;

  const { mutate: updatePipeline, isPending: isSaving } = useUpdatePipelineMutation();

  const [state, setState] = useState<PipelineSettingsPageGeneralState>(() => ({
    ...DEFAULT_STATE,
    name: stripDeletedName(pipeline?.name ?? ""),
    description: pipeline?.description ?? "",
  }));

  if (!pipeline) return null;

  const pipelineName = stripDeletedName(pipeline.name);

  const handleNameChange = (name: string) => {
    setState((prev) => ({ ...prev, name }));
  };

  const handleDescriptionChange = (description: string) => {
    setState((prev) => ({ ...prev, description }));
  };

  const hasChanges =
    state.name.trim() !== pipelineName || state.description.trim() !== pipeline.description;

  const canSave = hasChanges && state.name.trim().length > 0;

  const handleSave = () => {
    const request = create(UpdatePipelineRequestSchema, {
      pipeline: {
        ...pipeline,
        name: state.name.trim(),
        description: state.description.trim(),
      },
    });

    updatePipeline(request, {
      onSuccess: () => {
        showToast({
          header: "Pipeline saved",
          subheader: "Your pipeline has been saved successfully.",
          variant: ToastVariant.SUCCESS,
        });
      },
      onError: (error) => {
        showToast({
          header: "Save failed",
          subheader: getErrorMessage(error, "Failed to save pipeline"),
          variant: ToastVariant.ERROR,
        });
      },
    });
  };

  return (
    <Accordion header="General" icon={SlidersIcon} isOpenInitial>
      <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} fillWidth>
        <TextInput
          value={state.name}
          onChange={handleNameChange}
          size={InputSize.LARGE}
          placeholder={formatPipelineName(pipeline)}
          label="Name"
          fillWidth
        />
        <TextAreaInput
          value={state.description}
          onChange={handleDescriptionChange}
          size={InputSize.LARGE}
          placeholder="Optional description"
          label="Description"
          fillWidth
        />
        <FlexWrapper justifyContent={JustifyContent.END} fillWidth>
          <Button label="Save" isDisabled={!canSave} isLoading={isSaving} onClick={handleSave} />
        </FlexWrapper>
      </FlexWrapper>
    </Accordion>
  );
};

export default PipelineSettingsPageGeneral;
