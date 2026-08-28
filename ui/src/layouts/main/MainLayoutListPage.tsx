import { type PropsWithChildren, type ReactNode, useEffect, useState } from "react";

import { styled } from "@linaria/react";
import { MagnifyingGlassIcon } from "@phosphor-icons/react";
import { useNavigate, useSearch } from "@tanstack/react-router";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { useDebouncedValue } from "@galaxy-io/dls/inputs/hooks";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import BaseToolbar from "@/layouts/components/BaseToolbar";

import { LIST_SEARCH_DEBOUNCE_MS } from "@/api/utils";

const MainLayoutListPageScrollArea = styled.div<{ $noPadding?: boolean }>`
  flex: 1;
  width: 100%;
  min-height: 0;

  padding: ${({ $noPadding }) => ($noPadding ? 0 : 12)}px;

  display: flex;
  flex-direction: column;

  overflow-y: auto;
`;

interface MainLayoutListPage {
  actions: ReactNode[];
  noPadding?: boolean;
}

interface MainLayoutListPageState {
  search: string;
}

const DEFAULT_STATE: MainLayoutListPageState = {
  search: "",
};

const MainLayoutListPage = ({
  actions,
  noPadding = false,
  children,
}: PropsWithChildren<MainLayoutListPage>) => {
  const navigate = useNavigate();
  const { q = "" } = useSearch({ strict: false });

  const [state, setState] = useState<MainLayoutListPageState>(() => ({
    ...DEFAULT_STATE,
    search: q,
  }));
  const debouncedSearch = useDebouncedValue(state.search, LIST_SEARCH_DEBOUNCE_MS);

  const handleSearchChange = (search: string) => {
    setState((prev) => ({ ...prev, search }));
  };

  useEffect(() => {
    if (debouncedSearch === q) {
      return;
    }
    void navigate({
      to: ".",
      replace: true,
      search: (prev) => ({ ...prev, q: debouncedSearch || undefined }),
    });
  }, [debouncedSearch, q, navigate]);

  return (
    <FlexWrapper fillWidth fillHeight direction={FlexDirection.COLUMN}>
      <BaseToolbar
        leadingActions={[
          <TextInput
            key="search"
            value={state.search}
            onChange={handleSearchChange}
            placeholder="Search"
            leading={{ icon: MagnifyingGlassIcon }}
            fillWidth
          />,
        ]}
        trailingActions={actions}
      />
      <FlexItem grow={0} shrink={0} fillWidth>
        <HorizontalDivider />
      </FlexItem>
      <MainLayoutListPageScrollArea $noPadding={noPadding}>{children}</MainLayoutListPageScrollArea>
    </FlexWrapper>
  );
};

export default MainLayoutListPage;
