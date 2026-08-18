import CodeEditor from "@galaxy-io/dls/editor/CodeEditor";

import FieldWrapper from "@/components/fields/FieldWrapper";

import { DEFAULT_WORKER_CONFIGURATION_TEXT } from "@/pages/pipelines/components/worker/utils";

interface PipelineWorkerConfigurationEditorProps {
  value: string;
  onChange: (value: string) => void;
  error?: string;
}

const PipelineWorkerConfigurationEditor = ({
  value,
  onChange,
  error,
}: PipelineWorkerConfigurationEditorProps) => (
  <FieldWrapper
    label="Pod template"
    help="Applied to the Kubernetes Job for every run of this pipeline"
    error={error}
  >
    <CodeEditor
      content={value}
      onChange={onChange}
      placeholder={DEFAULT_WORKER_CONFIGURATION_TEXT}
      lang="json"
      noLineNumbers
    />
  </FieldWrapper>
);

export default PipelineWorkerConfigurationEditor;
