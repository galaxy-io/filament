import type { FC } from "react";

import SearchInput from "@galaxy-io/dls/inputs/SearchInput";

import { useFilamentSearchUpdate, useListSearch } from "@/module/hooks";
import type { ListSearchParams } from "@/module/schemas";

import { LIST_SEARCH_DEBOUNCE_MS } from "@/constants";

interface ListSearchProps {
  placeholder: string;
}

const ListSearch: FC<ListSearchProps> = ({ placeholder }) => {
  const updateSearch = useFilamentSearchUpdate<ListSearchParams>();
  const { q = "" } = useListSearch();

  const handleSearch = (term: string) => {
    if (term === q) return;
    void updateSearch((prev) => ({ ...prev, q: term || undefined }), { replace: true });
  };

  return (
    <SearchInput
      ariaLabel={placeholder}
      placeholder={placeholder}
      defaultValue={q}
      debounceMs={LIST_SEARCH_DEBOUNCE_MS}
      onSearch={handleSearch}
      fillWidth
    />
  );
};

export default ListSearch;
