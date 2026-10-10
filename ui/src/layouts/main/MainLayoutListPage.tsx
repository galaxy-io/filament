import type { FC, PropsWithChildren, ReactNode } from "react";

import SearchInput from "@galaxy-io/dls/inputs/SearchInput";
import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";

import BaseToolbar from "@/layouts/components/BaseToolbar";

import { useConnectionsSearch, useFilamentSearchUpdate } from "@/module/hooks";
import type { ConnectionsSearch } from "@/module/schemas";

import { LIST_SEARCH_DEBOUNCE_MS } from "@/constants";

interface MainLayoutListPage {
  actions: ReactNode[];
  noPadding?: boolean;
  isScrollable?: boolean;
}

const MainLayoutListPage: FC<PropsWithChildren<MainLayoutListPage>> = ({
  actions,
  noPadding = false,
  isScrollable = true,
  children,
}) => {
  const updateSearch = useFilamentSearchUpdate<ConnectionsSearch>();
  const { q = "" } = useConnectionsSearch();

  const handleSearch = (term: string) => {
    if (term === q) {
      return;
    }
    void updateSearch((prev) => ({ ...prev, q: term || undefined }), { replace: true });
  };

  return (
    <Flex alignItems={AlignItems.START} fillWidth height="100%" direction={FlexDirection.COLUMN}>
      <BaseToolbar
        leadingActions={[
          <SearchInput
            key="search"
            defaultValue={q}
            debounceMs={LIST_SEARCH_DEBOUNCE_MS}
            onSearch={handleSearch}
            fillWidth
          />,
        ]}
        trailingActions={actions}
      />
      <FlexItem grow={0} shrink={0} fillWidth>
        <Divider />
      </FlexItem>
      {isScrollable ? (
        <FlexItem grow={1} fillWidth minHeight={0}>
          <ScrollArea>
            <Box padding={noPadding ? 0 : 12}>{children}</Box>
          </ScrollArea>
        </FlexItem>
      ) : (
        <Flex
          direction={FlexDirection.COLUMN}
          grow={1}
          basis={0}
          fillWidth
          minHeight={0}
          padding={noPadding ? 0 : 12}
        >
          {children}
        </Flex>
      )}
    </Flex>
  );
};

export default MainLayoutListPage;
