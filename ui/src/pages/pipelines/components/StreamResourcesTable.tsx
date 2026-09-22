import { useMemo, useRef, useState } from "react";

import { MagnifyingGlassIcon, PlusIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import InfiniteTable, {
  type ColumnDef,
  type Row,
  TableVariant,
} from "@galaxy-io/dls/table/InfiniteTable";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

export interface StreamResourceRow {
  id: string;
  label: string;
  subject: string;
  selected: boolean;
}

// InfiniteTable uses this callback when memoizing its cell components. Keep
// its identity stable so editing a value does not remount the focused input.
const getResourceSelectAriaLabel = (row: Row<StreamResourceRow>) =>
  `Select ${row.original.label || row.original.subject || "resource"}`;

interface Props {
  rows: StreamResourceRow[];
  onChange: (row: StreamResourceRow) => void;
  onSelection: (selection: Record<string, boolean>) => void;
  onAdd: () => void;
  isDisabled?: boolean;
  fillHeight?: boolean;
}

export default function StreamResourcesTable({
  rows,
  onChange,
  onSelection,
  onAdd,
  isDisabled,
  fillHeight = false,
}: Props) {
  const [search, setSearch] = useState("");
  const filteredRows = useMemo(() => {
    const query = search.trim().toLowerCase();
    return rows.filter((row) => `${row.label} ${row.subject}`.toLowerCase().includes(query));
  }, [rows, search]);
  const changeRef = useRef(onChange);
  changeRef.current = onChange;
  const columns = useMemo<ColumnDef<StreamResourceRow>[]>(
    () => [
      {
        id: "label",
        header: "Destination table",
        cell: ({ row }) => (
          <TextInput
            aria-label="Destination table"
            value={row.original.label}
            onChange={(label) => changeRef.current({ ...row.original, label })}
            placeholder="orders"
            isDisabled={isDisabled}
            size={InputSize.SMALL}
            variant={InputVariant.TERTIARY}
            fillWidth
          />
        ),
      },
      {
        id: "subject",
        header: "Subject / topic",
        cell: ({ row }) => (
          <TextInput
            aria-label="Subject or topic"
            value={row.original.subject}
            onChange={(subject) => changeRef.current({ ...row.original, subject })}
            placeholder="orders.>"
            isDisabled={isDisabled}
            size={InputSize.SMALL}
            variant={InputVariant.TERTIARY}
            fillWidth
          />
        ),
      },
    ],
    [isDisabled],
  );
  const selection = Object.fromEntries(rows.map((row) => [row.id, row.selected]));
  return (
    <FlexWrapper
      direction={FlexDirection.COLUMN}
      fillWidth
      grow={fillHeight ? 1 : undefined}
      basis={fillHeight ? 0 : undefined}
      minHeight={0}
    >
      <FlexWrapper gap={8} padding="8px" alignItems={AlignItems.CENTER} fillWidth shrink={0}>
        <TextInput
          value={search}
          onChange={setSearch}
          placeholder="Search resources..."
          leading={{ icon: MagnifyingGlassIcon }}
          fillWidth
        />
        <FlexItem shrink={0}>
          <Button
            label="Add resource"
            icon={PlusIcon}
            variant={ButtonVariant.SECONDARY}
            onClick={() => {
              setSearch("");
              onAdd();
            }}
            isDisabled={isDisabled}
          />
        </FlexItem>
      </FlexWrapper>
      <HorizontalDivider />
      <FlexWrapper padding="8px 16px" fillWidth shrink={0}>
        <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
          Select subjects or topics and name their destination tables.
        </Text>
      </FlexWrapper>
      <FlexWrapper
        direction={FlexDirection.COLUMN}
        grow={fillHeight ? 1 : undefined}
        minHeight={0}
        fillWidth
      >
        <InfiniteTable<StreamResourceRow>
          variant={TableVariant.PRIMARY}
          columns={columns}
          data={filteredRows}
          getRowId={(row) => row.id}
          rowSelection={selection}
          onRowSelectionChange={(update) => {
            const next = typeof update === "function" ? update(selection) : update;
            onSelection({
              ...selection,
              ...Object.fromEntries(filteredRows.map((row) => [row.id, !!next[row.id]])),
            });
          }}
          enableRowSelection={() => !isDisabled}
          showSelectionColumn
          enableSelectAll
          enableSelectionRange
          fillWidth
          fillHeight={fillHeight}
          getRowSelectAriaLabel={getResourceSelectAriaLabel}
          contentWhenEmpty={
            <FlexWrapper padding="24px" fillWidth>
              <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                {search.trim()
                  ? "No resources match your search"
                  : "No resources discovered. Add a subject or topic to get started."}
              </Text>
            </FlexWrapper>
          }
        />
      </FlexWrapper>
    </FlexWrapper>
  );
}
