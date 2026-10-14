import { CheckboxInputVariant } from "@galaxy-io/dls/inputs/CheckboxInput";
import { InputVariant } from "@galaxy-io/dls/inputs/Input";
import { MultiSelectInputVariant } from "@galaxy-io/dls/inputs/MultiSelectInput";
import { SelectInputVariant } from "@galaxy-io/dls/inputs/SelectInput";
import { TagInputVariant } from "@galaxy-io/dls/inputs/TagInput";

export const INPUT_VARIANT_TO_CHECKBOX_INPUT_VARIANT_MAP: Record<
  InputVariant,
  CheckboxInputVariant
> = {
  [InputVariant.BASE]: CheckboxInputVariant.BASE,
  [InputVariant.PRIMARY]: CheckboxInputVariant.PRIMARY,
  [InputVariant.SECONDARY]: CheckboxInputVariant.SECONDARY,
  [InputVariant.TERTIARY]: CheckboxInputVariant.TERTIARY,
};

export const INPUT_VARIANT_TO_SELECT_INPUT_VARIANT_MAP: Record<InputVariant, SelectInputVariant> = {
  [InputVariant.BASE]: SelectInputVariant.BASE,
  [InputVariant.PRIMARY]: SelectInputVariant.PRIMARY,
  [InputVariant.SECONDARY]: SelectInputVariant.SECONDARY,
  [InputVariant.TERTIARY]: SelectInputVariant.TERTIARY,
};

export const INPUT_VARIANT_TO_MULTI_SELECT_INPUT_VARIANT_MAP: Record<
  InputVariant,
  MultiSelectInputVariant
> = {
  [InputVariant.BASE]: MultiSelectInputVariant.BASE,
  [InputVariant.PRIMARY]: MultiSelectInputVariant.PRIMARY,
  [InputVariant.SECONDARY]: MultiSelectInputVariant.SECONDARY,
  [InputVariant.TERTIARY]: MultiSelectInputVariant.TERTIARY,
};

export const INPUT_VARIANT_TO_TAG_INPUT_VARIANT_MAP: Record<InputVariant, TagInputVariant> = {
  [InputVariant.BASE]: TagInputVariant.BASE,
  [InputVariant.PRIMARY]: TagInputVariant.PRIMARY,
  [InputVariant.SECONDARY]: TagInputVariant.SECONDARY,
  [InputVariant.TERTIARY]: TagInputVariant.TERTIARY,
};
