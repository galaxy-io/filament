import { useState } from "react";

import { ProviderKind } from "@/gen/ingestion/v1/common_pb";

export const useProvidersPageState = () => {
  const [search, setSearch] = useState("");
  const [kindFilter, setKindFilter] = useState<ProviderKind>(ProviderKind.UNSPECIFIED);
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);

  const handleSelectKindFilter = (kind: ProviderKind) => {
    setKindFilter(kind);
    setIsFiltersOpen(false);
  };

  return {
    search,
    setSearch,
    kindFilter,
    setKindFilter,
    isFiltersOpen,
    setIsFiltersOpen,
    handleSelectKindFilter,
  };
};
