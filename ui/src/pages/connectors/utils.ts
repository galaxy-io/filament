import { ACRONYMS_TO_CAPITALIZE } from "@/pages/connectors/constants";

export function formatFieldName(fieldName: string): string {
  return (
    fieldName
      // Split on underscores or camelCase boundaries
      .replace(/_/g, " ")
      .replace(/([a-z])([A-Z])/g, "$1 $2")
      // Capitalize first letter of each word
      .split(" ")
      .map((word) => {
        if (ACRONYMS_TO_CAPITALIZE.includes(word.toLowerCase())) {
          return word.toUpperCase();
        }
        return word.charAt(0).toUpperCase() + word.slice(1).toLowerCase();
      })
      .join(" ")
  );
}
