export const isSearchMatch = (search: string, ...fields: string[]) => {
  const query = search.trim().toLowerCase();
  if (!query) return true;
  return fields.some((field) => field.toLowerCase().includes(query));
};
