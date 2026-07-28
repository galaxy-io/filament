import type { PropsWithChildren, ReactNode } from "react";

import { styled } from "@linaria/react";
import { MagnifyingGlassIcon } from "@phosphor-icons/react";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import BaseToolbar from "@/layouts/components/BaseToolbar";

const ScrollArea = styled.div<{ $padding: number }>`
  flex: 1;
  width: 100%;
  min-height: 0;

  padding: ${({ $padding }) => $padding}px;

  display: flex;
  flex-direction: column;

  overflow-y: auto;
`;

interface ListPageShellProps {
  search: string;
  onSearchChange: (search: string) => void;
  searchPlaceholder: string;
  trailingActions: ReactNode[];
  contentPadding?: number;
}

const ListPageShell = ({
  search,
  onSearchChange,
  searchPlaceholder,
  trailingActions,
  contentPadding = 0,
  children,
}: PropsWithChildren<ListPageShellProps>) => {
  return (
    <FlexWrapper fillWidth fillHeight direction={FlexDirection.COLUMN}>
      <BaseToolbar
        leadingActions={[
          <TextInput
            key="search"
            value={search}
            onChange={onSearchChange}
            placeholder={searchPlaceholder}
            leading={{ icon: MagnifyingGlassIcon }}
            fillWidth
          />,
        ]}
        trailingActions={trailingActions}
      />
      <FlexItem grow={0} shrink={0} fillWidth>
        <HorizontalDivider />
      </FlexItem>
      <ScrollArea $padding={contentPadding}>{children}</ScrollArea>
    </FlexWrapper>
  );
};

export default ListPageShell;
