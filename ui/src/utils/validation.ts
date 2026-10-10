export const isNameValid = (name: string | undefined): boolean => {
  return (name ?? "").trim().length > 0;
};

export const getNameError = (
  name: string | undefined,
  shouldShowErrors: boolean,
): string | null => {
  return shouldShowErrors && !isNameValid(name) ? "Name is required" : null;
};
