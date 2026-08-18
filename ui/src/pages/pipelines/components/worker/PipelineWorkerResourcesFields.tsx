import { styled } from "@linaria/react";

import { InputSize } from "@galaxy-io/dls/inputs/Input";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import type { WorkerResources } from "@/gen/ingestion/v1/common_pb";

interface PipelineWorkerResourcesFieldsProps {
  state: WorkerResources;
  onChange: (partial: Partial<WorkerResources>) => void;
}

const FieldsGrid = styled.div`
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
  width: 100%;

  @media (max-width: 640px) {
    grid-template-columns: 1fr;
  }
`;

const PipelineWorkerResourcesFields = ({ state, onChange }: PipelineWorkerResourcesFieldsProps) => (
  <FieldsGrid>
    <TextInput
      value={state.cpuRequest}
      onChange={(value) => onChange({ cpuRequest: value })}
      size={InputSize.LARGE}
      placeholder="500m"
      label="CPU request"
      fillWidth
    />
    <TextInput
      value={state.cpuLimit}
      onChange={(value) => onChange({ cpuLimit: value })}
      size={InputSize.LARGE}
      placeholder="1"
      label="CPU limit"
      fillWidth
    />
    <TextInput
      value={state.memoryRequest}
      onChange={(value) => onChange({ memoryRequest: value })}
      size={InputSize.LARGE}
      placeholder="1Gi"
      label="Memory request"
      fillWidth
    />
    <TextInput
      value={state.memoryLimit}
      onChange={(value) => onChange({ memoryLimit: value })}
      size={InputSize.LARGE}
      placeholder="2Gi"
      label="Memory limit"
      fillWidth
    />
  </FieldsGrid>
);

export default PipelineWorkerResourcesFields;
