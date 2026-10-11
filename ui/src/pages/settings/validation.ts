export const getRequiredFieldError = (
  value: string,
  label: string,
  shouldShowErrors: boolean,
): string | null => {
  return shouldShowErrors && value.trim() === "" ? `${label} is required` : null;
};
