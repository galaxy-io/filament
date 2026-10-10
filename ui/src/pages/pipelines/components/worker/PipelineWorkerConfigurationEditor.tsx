import type { FC } from "react";

import CodeEditor, { CodeEditorLanguage } from "@galaxy-io/dls/editor/CodeEditor";
import Field from "@galaxy-io/dls/inputs/Field";

import { DEFAULT_WORKER_CONFIGURATION_TEXT } from "@/pages/pipelines/components/worker/utils";

interface PipelineWorkerConfigurationEditorProps {
  value: string;
  onChange: (value: string) => void;
  error?: string;
  label?: string;
  help?: string;
}

const PipelineWorkerConfigurationEditor: FC<PipelineWorkerConfigurationEditorProps> = ({
  value,
  onChange,
  error,
  label = "Pod template",
  help = "Applied to the Kubernetes Job for every run of this pipeline",
}) => (
  <Field label={label} labelTooltip={help} error={error} fillWidth>
    <CodeEditor
      value={value}
      onChange={onChange}
      language={CodeEditorLanguage.JSON}
      placeholder={DEFAULT_WORKER_CONFIGURATION_TEXT}
      hasLineNumbers={false}
    />
  </Field>
);

export default PipelineWorkerConfigurationEditor;
