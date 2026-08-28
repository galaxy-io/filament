import CodeEditor from "@galaxy-io/dls/editor/CodeEditor";

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
      content={value}
      onChange={onChange}
      lang="json"
      placeholder={DEFAULT_WORKER_CONFIGURATION_TEXT}
      borderRadius={4}
      noLineNumbers
    />
  </FieldWrapper>
);

export default PipelineWorkerConfigurationEditor;
