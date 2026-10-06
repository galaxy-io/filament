import type { SelectOption } from "@galaxy-io/dls/inputs/SelectInput";

export interface SelectAllOption {
  id: string;
  label: string;
  optionIds: string[];
}

export const isSelectAllChecked = (selectAll: SelectAllOption, selected: string[]) =>
  selectAll.optionIds.length > 0 && selectAll.optionIds.every((id) => selected.includes(id));

export const getSelectAllOptions = (
  selectAll: SelectAllOption,
  options: SelectOption[],
): SelectOption[] => [{ id: selectAll.id, label: selectAll.label }, ...options];

export const getSelectAllValue = (selectAll: SelectAllOption, selected: string[]) =>
  isSelectAllChecked(selectAll, selected) ? [selectAll.id, ...selected] : selected;

export const getSelectAllChange = (
  selectAll: SelectAllOption,
  next: string[],
  selected: string[],
): string[] => {
  const wasChecked = isSelectAllChecked(selectAll, selected);
  const isChecked = next.includes(selectAll.id);
  const rest = next.filter((id) => id !== selectAll.id);
  if (isChecked === wasChecked) return rest;
  if (isChecked) return [...rest, ...selectAll.optionIds.filter((id) => !rest.includes(id))];
  return rest.filter((id) => !selectAll.optionIds.includes(id));
};
