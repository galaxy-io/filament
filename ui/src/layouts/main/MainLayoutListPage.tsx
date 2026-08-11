import type { PropsWithChildren, ReactNode } from "react";

import { styled } from "@linaria/react";
import { MagnifyingGlassIcon } from "@phosphor-icons/react";
import { useNavigate, useSearch } from "@tanstack/react-router";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import BaseToolbar from "@/layouts/components/BaseToolbar";

const MAIN_LIST_PAGE_SEARCH_WIDTH = 300;

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

const MainLayoutListPage = ({
  actions,
  noPadding = false,
  children,
}: PropsWithChildren<MainLayoutListPage>) => {
  const navigate = useNavigate();
  const { q = "" } = useSearch({ strict: false });

  const handleSearchChange = (value: string) => {
    void navigate({
      to: ".",
      replace: true,
      search: (prev) => ({ ...prev, q: value || undefined }),
    });
  };

  return (
    <FlexWrapper fillWidth fillHeight direction={FlexDirection.COLUMN}>
      <BaseToolbar
        leadingActions={[
          <TextInput
            key="search"
            value={q}
            onChange={handleSearchChange}
            placeholder="Search"
            leading={{ icon: MagnifyingGlassIcon }}
            width={MAIN_LIST_PAGE_SEARCH_WIDTH}
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
