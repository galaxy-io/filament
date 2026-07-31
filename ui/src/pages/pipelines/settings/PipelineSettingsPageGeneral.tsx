import { useState } from "react";

import { create } from "@bufbuild/protobuf";
import { SlidersIcon } from "@phosphor-icons/react";

import Accordion from "@galaxy-io/dls/accordion/Accordion";
import Button from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, {
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import TextAreaInput, { TextAreaSize } from "@galaxy-io/dls/inputs/TextAreaInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import { type Pipeline, UpdatePipelineRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";

import { formatPipelineName } from "@/pages/pipelines/utils";

import { useUpdatePipelineMutation } from "@/api/queries/pipelines";

import { getErrorMessage } from "@/utils/errors";

interface PipelineSettingsPageGeneralState {
  name: Pipeline["name"];
  description: Pipeline["description"];
}

const DEFAULT_STATE: PipelineSettingsPageGeneralState = {
  name: "",
  description: "",
};

interface PipelineSettingsPageGeneralProps {
  pipeline: Pipeline;
}

const PipelineSettingsPageGeneral = ({ pipeline }: PipelineSettingsPageGeneralProps) => {
  const { showToast } = useToast();

  const { mutate: updatePipeline, isPending: isSaving } = useUpdatePipelineMutation();

  const [state, setState] = useState<PipelineSettingsPageGeneralState>(() => ({
    ...DEFAULT_STATE,
    name: pipeline.name,
    description: pipeline.description,
  }));

  const handleNameChange = (name: string) => {
    setState((prev) => ({ ...prev, name }));
  };

  const handleDescriptionChange = (description: string) => {
    setState((prev) => ({ ...prev, description }));
  };

  const hasChanges =
    state.name.trim() !== pipeline.name || state.description.trim() !== pipeline.description;

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
          size={TextAreaSize.LARGE}
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
