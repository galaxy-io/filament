import { styled } from "@linaria/react";
import type { Icon as PhosphorIcon } from "@phosphor-icons/react";
import { CpuIcon, MemoryIcon } from "@phosphor-icons/react";

import { InputSize } from "@galaxy-io/dls/inputs/Input";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import type {
  WorkerResourcesDraft,
  WorkerResourcesState,
} from "@/pages/pipelines/components/worker/types";
import { trimWorkerResourceValue } from "@/pages/pipelines/components/worker/utils";

type WorkerResourceField = keyof WorkerResourcesState;

interface WorkerResourcesFieldConfig {
  field: WorkerResourceField;
  label: string;
  placeholder: string;
  icon: PhosphorIcon;
}

interface WorkerResourcesFieldsProps {
  state: WorkerResourcesState;
  onChange: (partial: Partial<WorkerResourcesState>) => void;
  defaults?: WorkerResourcesDraft;
  emptyPlaceholder?: string;
  isDisabled?: boolean;
  size?: InputSize;
}

const WORKER_RESOURCE_FIELDS: WorkerResourcesFieldConfig[] = [
  {
    field: "cpuRequest",
    label: "CPU request",
    placeholder: "500m",
    icon: CpuIcon,
  },
  {
    field: "cpuLimit",
    label: "CPU limit",
    placeholder: "1",
    icon: CpuIcon,
  },
  {
    field: "memoryRequest",
    label: "Memory request",
    placeholder: "1Gi",
    icon: MemoryIcon,
  },
  {
    field: "memoryLimit",
    label: "Memory limit",
    placeholder: "2Gi",
    icon: MemoryIcon,
  },
];

const FieldsGrid = styled.div`
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  width: 100%;

  @media (max-width: 640px) {
    grid-template-columns: 1fr;
  }
`;

const getPlaceholder = (
  field: WorkerResourcesFieldConfig,
  defaults: WorkerResourcesFieldsProps["defaults"],
  emptyPlaceholder: string | undefined,
) => {
  const defaultValue = trimWorkerResourceValue(defaults?.[field.field]);
  if (defaultValue) return `Default: ${defaultValue}`;
  return emptyPlaceholder ?? field.placeholder;
};

const WorkerResourcesFields = ({
  state,
  onChange,
  defaults,
  emptyPlaceholder,
  isDisabled = false,
  size = InputSize.LARGE,
}: WorkerResourcesFieldsProps) => (
  <FieldsGrid>
    {WORKER_RESOURCE_FIELDS.map((field) => (
      <TextInput
        key={field.field}
        value={state[field.field]}
        onChange={(value) => onChange({ [field.field]: value })}
        size={size}
        placeholder={getPlaceholder(field, defaults, emptyPlaceholder)}
        label={field.label}
        leading={{ icon: field.icon }}
        isDisabled={isDisabled}
        fillWidth
      />
    ))}
  </FieldsGrid>
);

export default WorkerResourcesFields;
