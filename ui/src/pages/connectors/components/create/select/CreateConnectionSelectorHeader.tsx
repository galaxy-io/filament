import { useCallback } from "react";

import { MagnifyingGlassIcon, XIcon } from "@phosphor-icons/react";

import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import BaseHeader from "@/layouts/components/BaseHeader";
import { BaseHeaderSize } from "@/layouts/components/types";

interface CreateConnectionSelectorHeaderProps {
  search: string;
  onSearchChange: (search: string) => void;
  onClose: () => void;
}

const CreateConnectionSelectorHeader = ({
  search,
  onSearchChange,
  onClose,
}: CreateConnectionSelectorHeaderProps) => {
  const handleSearchClear = useCallback(() => {
    onSearchChange("");
  }, [onSearchChange]);

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} fillWidth>
      <FlexWrapper padding="16px" fillWidth>
        <BaseHeader
          size={BaseHeaderSize.LARGE}
          title="New connection"
          description="Connect to a data source or sink"
          onClose={onClose}
        />
      </FlexWrapper>
      <HorizontalDivider />
      <FlexWrapper padding="8px 16px" fillWidth>
        <TextInput
          key="search"
          placeholder="Search connectors..."
          leading={{ icon: MagnifyingGlassIcon }}
          trailing={search ? { icon: XIcon, onClick: handleSearchClear } : undefined}
          onChange={onSearchChange}
          value={search}
          size={InputSize.LARGE}
          fillWidth
        />
      </FlexWrapper>
    </FlexWrapper>
  );
};

export default CreateConnectionSelectorHeader;
