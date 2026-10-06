import { type PropsWithChildren, type ReactNode, useEffect, useState } from "react";

import { styled } from "@linaria/react";
import { MagnifyingGlassIcon } from "@phosphor-icons/react";
import { useNavigate, useSearch } from "@tanstack/react-router";

import { useDebouncedValue } from "@galaxy-io/dls/hooks/useDebouncedValue";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";

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
    <Flex alignItems={AlignItems.START} fillWidth height="100%" direction={FlexDirection.COLUMN}>
      <BaseToolbar
        leadingActions={[
          <TextInput
            key="search"
            value={state.search}
            onChange={handleSearchChange}
            placeholder="Search"
            icon={MagnifyingGlassIcon}
            fillWidth
          />,
        ]}
        trailingActions={actions}
      />
      <FlexItem grow={0} shrink={0} fillWidth>
        <Divider />
      </FlexItem>
      <MainLayoutListPageScrollArea $noPadding={noPadding}>{children}</MainLayoutListPageScrollArea>
    </Flex>
  );
};

export default MainLayoutListPage;
