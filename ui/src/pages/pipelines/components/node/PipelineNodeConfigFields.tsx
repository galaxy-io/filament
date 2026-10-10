import type { FC } from "react";

import type { InputVariant } from "@galaxy-io/dls/inputs/Input";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";

import Field from "@/components/fields/Field";
import { updateConfigField } from "@/components/fields/utils";

import type {
  PipelineNodeConfig,
  PipelineNodeConfigState,
} from "@/pages/pipelines/components/node/types";

interface PipelineNodeConfigFieldsProps extends PipelineNodeConfigState {
  config: PipelineNodeConfig;
  onChange: (config: PipelineNodeConfig) => void;
  variant?: InputVariant;
  isDisabled?: boolean;
}

const PipelineNodeConfigFields: FC<PipelineNodeConfigFieldsProps> = ({
  scopedFields,
  fields,
  displayValue,
  config,
  onChange,
  variant,
  isDisabled,
}) => (
  <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={12} fillWidth>
    {fields.map((field) => (
      <Field
        key={field.name}
        field={field}
        value={displayValue[field.name] ?? null}
        variant={variant}
        onChange={(value) => onChange(updateConfigField(scopedFields, config, field.name, value))}
        isDisabled={isDisabled}
      />
    ))}
  </Flex>
);

export default PipelineNodeConfigFields;
