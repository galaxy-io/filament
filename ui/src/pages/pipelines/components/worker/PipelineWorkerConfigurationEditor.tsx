import CodeEditor, { CodeEditorLanguage } from "@galaxy-io/dls/editor/CodeEditor";

import FieldWrapper from "@/components/fields/FieldWrapper";

import { DEFAULT_WORKER_CONFIGURATION_TEXT } from "@/pages/pipelines/components/worker/utils";

interface PipelineWorkerConfigurationEditorProps {
  value: string;
  onChange: (value: string) => void;
  error?: string;
  label?: string;
  help?: string;
}

const PipelineWorkerConfigurationEditor = ({
  value,
  onChange,
  error,
  label = "Pod template",
  help = "Applied to the Kubernetes Job for every run of this pipeline",
}: PipelineWorkerConfigurationEditorProps) => (
  <FieldWrapper label={label} help={help} error={error}>
    <CodeEditor
      value={value}
      onChange={onChange}
      language={CodeEditorLanguage.JSON}
      placeholder={DEFAULT_WORKER_CONFIGURATION_TEXT}
      hasLineNumbers={false}
    />
  </FieldWrapper>
);

export default PipelineWorkerConfigurationEditor;
