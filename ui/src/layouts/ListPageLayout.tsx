import type { PropsWithChildren, ReactNode } from "react";

import { styled } from "@linaria/react";
import { MagnifyingGlassIcon } from "@phosphor-icons/react";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import BaseToolbar from "@/layouts/components/BaseToolbar";

const ScrollArea = styled.div<{ $noPadding?: boolean }>`
  flex: 1;
  width: 100%;
  min-height: 0;

  padding: ${({ $noPadding }) => ($noPadding ? 0 : 12)}px;

  display: flex;
  flex-direction: column;

  overflow-y: auto;
`;

interface ListPageLayoutProps {
  search: string;
  onSearchChange: (search: string) => void;
  actions: ReactNode[];
  noPadding?: boolean;
}

const ListPageLayout = ({
  search,
  onSearchChange,
  actions,
  noPadding = false,
  children,
}: PropsWithChildren<ListPageLayoutProps>) => {
  return (
    <FlexWrapper fillWidth fillHeight direction={FlexDirection.COLUMN}>
      <BaseToolbar
        leadingActions={[
          <TextInput
            key="search"
            value={search}
            onChange={onSearchChange}
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
      <ScrollArea $noPadding={noPadding}>{children}</ScrollArea>
    </FlexWrapper>
  );
};

export default ListPageLayout;
