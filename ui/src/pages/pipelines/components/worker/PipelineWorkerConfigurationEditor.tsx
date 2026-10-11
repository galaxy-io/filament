import type { FC } from "react";

import CodeEditor, { CodeEditorLanguage } from "@galaxy-io/dls/editor/CodeEditor";
import Field from "@galaxy-io/dls/inputs/Field";

import { PIPELINE_WORKER_CONFIGURATION_DEFAULT_TEXT } from "@/pages/pipelines/components/worker/constants";

interface PipelineWorkerConfigurationEditorProps {
  value: string;
  onChange: (value: string) => void;
  error?: string;
  isPerRun?: boolean;
}

const PipelineWorkerConfigurationEditor: FC<PipelineWorkerConfigurationEditorProps> = ({
  value,
  onChange,
  error,
  isPerRun = false,
}) => (
  <Field
    label="Pod template"
    labelTooltip={
      isPerRun
        ? "Applied to the Kubernetes Job for this run only"
        : "Applied to the Kubernetes Job for every run of this pipeline"
    }
    error={error}
    fillWidth
  >
    <CodeEditor
      value={value}
      onChange={onChange}
      language={CodeEditorLanguage.JSON}
      placeholder={PIPELINE_WORKER_CONFIGURATION_DEFAULT_TEXT}
      hasLineNumbers={false}
    />
  </Field>
);

export default PipelineWorkerConfigurationEditor;
