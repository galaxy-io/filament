# Recipes

Complete Galaxy screens built only from `@galaxy-io/dls`. Each one type-checks against the package. Data fetching, routing and persistence are stubbed with `declare` so the UI is what you read; replace them with the app's own.

The examples are written for `exactOptionalPropertyTypes`, so an optional prop that may be `undefined` is passed with a conditional spread (`{...(error ? { error } : {})}`). Without that compiler flag, `error={error}` works too.

## Settings page

A settings page with a vertical `Tabs` column, horizontal `Field`s, switches that save immediately, and the theme switcher.

```tsx
import { useState } from "react";
import { BellIcon, PaletteIcon, UserIcon } from "@phosphor-icons/react";
import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Field from "@galaxy-io/dls/inputs/Field";
import Fieldset from "@galaxy-io/dls/inputs/Fieldset";
import SelectInput from "@galaxy-io/dls/inputs/SelectInput";
import SwitchInput from "@galaxy-io/dls/inputs/SwitchInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Tabs from "@galaxy-io/dls/navigation/Tabs";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { Orientation } from "@galaxy-io/dls/theme/enums";
import ThemeSwitcher from "@galaxy-io/dls/theme/ThemeSwitcher";
import { useToast } from "@galaxy-io/dls/toast/useToast";

declare function saveProfile(profile: { name: string; timezone: string | null }): Promise<void>;
declare function saveNotificationSetting(key: string, isOn: boolean): Promise<void>;

function ProfileSection() {
  const { toast } = useToast();
  const [name, setName] = useState("Ada Lovelace");
  const [timezone, setTimezone] = useState<string | null>("Europe/London");
  const [isSaving, setIsSaving] = useState(false);
  const nameError = name.trim() === "" ? "Enter your name." : undefined;
  return (
    <Fieldset label="Profile" description="Shown to everyone in the workspace.">
      <Field label="Name" orientation={Orientation.HORIZONTAL} isRequired {...(nameError ? { error: nameError } : {})}>
        <TextInput value={name} onChange={setName} fillWidth />
      </Field>
      <Field label="Time zone" orientation={Orientation.HORIZONTAL}>
        <SelectInput
          options={[
            { id: "Europe/London", label: "London", description: "GMT+1" },
            { id: "America/New_York", label: "New York", description: "GMT-4" },
            { id: "Asia/Tokyo", label: "Tokyo", description: "GMT+9" },
          ]}
          value={timezone}
          onChange={setTimezone}
          isSearchable
          fillWidth
        />
      </Field>
      <Flex justifyContent={JustifyContent.END}>
        <Button
          label="Save profile"
          isLoading={isSaving}
          isDisabled={nameError !== undefined}
          onClick={async () => {
            setIsSaving(true);
            try {
              await saveProfile({ name, timezone });
              toast("Profile saved");
            } finally {
              setIsSaving(false);
            }
          }}
        />
      </Flex>
    </Fieldset>
  );
}

function NotificationSetting({ id, label, description }: { id: string; label: string; description: string }) {
  const [isOn, setIsOn] = useState(true);
  const [isPending, setIsPending] = useState(false);
  return (
    <SwitchInput
      label={label}
      description={description}
      isChecked={isOn}
      isLoading={isPending}
      fillWidth
      onChange={async (next) => {
        setIsPending(true);
        try {
          await saveNotificationSetting(id, next);
          setIsOn(next);
        } finally {
          setIsPending(false);
        }
      }}
    />
  );
}

export function SettingsPage() {
  return (
    <Flex direction={FlexDirection.COLUMN} gap={24} padding={24} maxWidth={960}>
      <Text as="h1" size={TextSize.HEADING_MD} weight={TextWeight.MEDIUM}>
        Settings
      </Text>
      <Tabs
        ariaLabel="Settings sections"
        orientation={Orientation.VERTICAL}
        items={[
          { id: "profile", label: "Profile", icon: UserIcon, panel: <ProfileSection /> },
          {
            id: "notifications",
            label: "Notifications",
            icon: BellIcon,
            panel: (
              <Fieldset label="Email me when">
                <NotificationSetting id="run.failed" label="A run fails" description="One email per pipeline per hour." />
                <Divider />
                <NotificationSetting id="schema.changed" label="A schema changes" description="Columns added, removed or retyped." />
              </Fieldset>
            ),
          },
          {
            id: "appearance",
            label: "Appearance",
            icon: PaletteIcon,
            panel: (
              <Fieldset label="Appearance">
                <ThemeSwitcher label="Theme" />
                <Text variant={TextVariant.SECONDARY}>System follows your operating system.</Text>
              </Fieldset>
            ),
          },
        ]}
      />
      <Flex>
        <Button label="Sign out" variant={ButtonVariant.SECONDARY} />
      </Flex>
    </Flex>
  );
}
```

## Table page with filters and row actions

A page header, a toolbar (search, a filter dropdown, bulk actions while rows are selected), and an `InfiniteTable` with sorting, selection, row actions, empty, loading and error states.

```tsx
import { useMemo, useState } from "react";
import { ArrowsClockwiseIcon, FunnelIcon, PauseIcon, PlugsIcon, PlusIcon, TrashIcon } from "@phosphor-icons/react";
import Beacon, { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Chip, { ChipSize } from "@galaxy-io/dls/chips/Chip";
import Dropdown from "@galaxy-io/dls/dropdown/Dropdown";
import EmptyState, { EmptyStateSize } from "@galaxy-io/dls/feedback/EmptyState";
import CheckboxGroup from "@galaxy-io/dls/inputs/CheckboxGroup";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import SearchInput from "@galaxy-io/dls/inputs/SearchInput";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import { MenuItem, MenuItemVariant, MenuSeparator } from "@galaxy-io/dls/menu/Menu";
import ConfirmDialog from "@galaxy-io/dls/modal/ConfirmDialog";
import InfiniteTable from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn } from "@galaxy-io/dls/table/types";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { formatNumber, formatRelativeTime } from "@galaxy-io/dls/utils/format";

type SourceStatus = "healthy" | "syncing" | "failing";

interface Source {
  id: string;
  name: string;
  kind: string;
  status: SourceStatus;
  rows: number;
  lastSyncedAt: number;
}

declare function useSources(): { sources: Source[]; isLoading: boolean; error: string | undefined; reload: () => void };
declare function deleteSources(ids: string[]): Promise<void>;

const STATUS_TO_BEACON_MAP: Record<SourceStatus, { label: string; variant: BeaconVariant; isPulse: boolean }> = {
  healthy: { label: "Healthy", variant: BeaconVariant.SUCCESS, isPulse: false },
  syncing: { label: "Syncing", variant: BeaconVariant.PRIMARY, isPulse: true },
  failing: { label: "Failing", variant: BeaconVariant.ERROR, isPulse: false },
};

const COLUMNS: TableColumn<Source>[] = [
  {
    id: "name",
    header: "Name",
    accessor: (row) => row.name,
    isRowHeader: true,
    canSort: true,
    minWidth: 200,
    cell: ({ row }) => (
      <Flex gap={8} alignItems={AlignItems.CENTER} minWidth={0}>
        <Text lineClamp={1} shouldTooltipOnOverflow>
          {row.name}
        </Text>
        <Chip label={row.kind} size={ChipSize.SMALL} />
      </Flex>
    ),
  } satisfies TableColumn<Source, string>,
  {
    id: "status",
    header: "Status",
    accessor: (row) => row.status,
    width: 140,
    cell: ({ row }) => {
      const status = STATUS_TO_BEACON_MAP[row.status];
      return <Beacon label={status.label} variant={status.variant} isPulse={status.isPulse} />;
    },
  } satisfies TableColumn<Source, SourceStatus>,
  {
    id: "rows",
    header: "Rows",
    accessor: (row) => row.rows,
    align: "right",
    canSort: true,
    width: 120,
    cell: ({ value }) => <Text family={FontFamily.MONO}>{formatNumber(value)}</Text>,
  } satisfies TableColumn<Source, number>,
  {
    id: "lastSyncedAt",
    header: "Last sync",
    accessor: (row) => row.lastSyncedAt,
    canSort: true,
    width: 140,
    cell: ({ value }) => <Text variant={TextVariant.SECONDARY}>{formatRelativeTime(value)}</Text>,
  } satisfies TableColumn<Source, number>,
];

const STATUS_OPTIONS = [
  { id: "healthy", label: "Healthy" },
  { id: "syncing", label: "Syncing" },
  { id: "failing", label: "Failing" },
] satisfies { id: SourceStatus; label: string }[];

export function SourcesPage() {
  const { sources, isLoading, error, reload } = useSources();
  const [term, setTerm] = useState("");
  const [statuses, setStatuses] = useState<SourceStatus[]>([]);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [pendingDelete, setPendingDelete] = useState<string[]>([]);

  const visible = useMemo(
    () =>
      sources.filter(
        (source) =>
          source.name.toLowerCase().includes(term.toLowerCase()) &&
          (statuses.length === 0 || statuses.includes(source.status)),
      ),
    [sources, term, statuses],
  );
  const isFiltered = term !== "" || statuses.length > 0;

  return (
    <Flex direction={FlexDirection.COLUMN} gap={16} padding={24} height="100%">
      <Flex justifyContent={JustifyContent.SPACE_BETWEEN} alignItems={AlignItems.CENTER}>
        <Text as="h1" size={TextSize.HEADING_MD} weight={TextWeight.MEDIUM}>
          Sources
        </Text>
        <Button label="New source" icon={PlusIcon} />
      </Flex>

      <Flex gap={8} alignItems={AlignItems.CENTER}>
        <SearchInput size={InputSize.SMALL} placeholder="Search sources" onSearch={setTerm} />
        <Dropdown
          header="Status"
          body={<CheckboxGroup ariaLabel="Status" options={STATUS_OPTIONS} value={statuses} onChange={setStatuses} />}
        >
          <Button
            label="Status"
            icon={FunnelIcon}
            variant={ButtonVariant.SECONDARY}
            size={ButtonSize.SMALL}
            trailing={statuses.length > 0 ? <Chip count={statuses.length} size={ChipSize.X_SMALL} /> : undefined}
          />
        </Dropdown>
        {selectedIds.length > 0 && (
          <Flex gap={8} alignItems={AlignItems.CENTER}>
            <Text variant={TextVariant.SECONDARY}>{selectedIds.length} selected</Text>
            <Button
              label="Delete"
              icon={TrashIcon}
              variant={ButtonVariant.SECONDARY}
              size={ButtonSize.SMALL}
              onClick={() => setPendingDelete(selectedIds)}
            />
          </Flex>
        )}
      </Flex>

      <Flex direction={FlexDirection.COLUMN} grow={1} minHeight={0}>
        <InfiniteTable
          ariaLabel="Sources"
          columns={COLUMNS}
          data={visible}
          getRowId={(row) => row.id}
          isLoading={isLoading}
          {...(error ? { error } : {})}
          isSelectable
          value={selectedIds}
          onChange={setSelectedIds}
          defaultSort={{ columnId: "name", isDescending: false }}
          rowActions={(row) => (
            <>
              <MenuItem label="Sync now" icon={ArrowsClockwiseIcon} onSelect={reload} />
              <MenuItem label="Pause" icon={PauseIcon} onSelect={() => {}} />
              <MenuSeparator />
              <MenuItem
                label="Delete"
                icon={TrashIcon}
                variant={MenuItemVariant.ERROR}
                onSelect={() => setPendingDelete([row.id])}
              />
            </>
          )}
          emptyState={
            isFiltered ? (
              <EmptyState
                role="status"
                size={EmptyStateSize.SMALL}
                header="No matching sources"
                description="Try another search or clear the status filter."
                actions={
                  <Button
                    label="Clear filters"
                    variant={ButtonVariant.SECONDARY}
                    size={ButtonSize.SMALL}
                    onClick={() => setStatuses([])}
                  />
                }
              />
            ) : (
              <EmptyState
                icon={PlugsIcon}
                header="No sources yet"
                description="Connect a database or an app to start syncing data."
                actions={<Button label="Connect source" />}
              />
            )
          }
        />
      </Flex>

      <ConfirmDialog
        isOpen={pendingDelete.length > 0}
        onOpenChange={(isOpen) => {
          if (!isOpen) setPendingDelete([]);
        }}
        header={pendingDelete.length === 1 ? "Delete this source?" : `Delete ${pendingDelete.length} sources?`}
        description="They stop syncing. Data already in the warehouse stays."
        label={pendingDelete.length === 1 ? "Delete source" : "Delete sources"}
        isDestructive
        onConfirm={async () => {
          await deleteSources(pendingDelete);
          setSelectedIds([]);
          reload();
        }}
      />
    </Flex>
  );
}
```

## Create / edit form in a modal

One `Modal` for both create and edit: validation on submit, then live once an error shows; `isLoading` on the submit; a toast on success.

```tsx
import { useState } from "react";
import Alert, { AlertVariant } from "@galaxy-io/dls/feedback/Alert";
import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Field from "@galaxy-io/dls/inputs/Field";
import NumberInput from "@galaxy-io/dls/inputs/NumberInput";
import RadioGroup from "@galaxy-io/dls/inputs/RadioGroup";
import TextAreaInput from "@galaxy-io/dls/inputs/TextAreaInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Flex, { FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Modal, { ModalSize } from "@galaxy-io/dls/modal/Modal";
import { useToast } from "@galaxy-io/dls/toast/useToast";

interface Pipeline {
  name: string;
  description: string;
  mode: "incremental" | "full";
  retries: number | null;
}

declare function savePipeline(pipeline: Pipeline): Promise<void>;

const EMPTY_PIPELINE: Pipeline = { name: "", description: "", mode: "incremental", retries: 3 };

function validate(pipeline: Pipeline): Partial<Record<keyof Pipeline, string>> {
  const errors: Partial<Record<keyof Pipeline, string>> = {};
  if (pipeline.name.trim() === "") errors.name = "Enter a pipeline name.";
  else if (pipeline.name.length > 80) errors.name = "Use 80 characters or fewer.";
  if (pipeline.description.length > 280) errors.description = "Use 280 characters or fewer.";
  return errors;
}

export function PipelineDialog({
  isOpen,
  onOpenChange,
  initial,
}: {
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  initial?: Pipeline;
}) {
  const { toast } = useToast();
  const isEditing = initial !== undefined;
  const [pipeline, setPipeline] = useState<Pipeline>(initial ?? EMPTY_PIPELINE);
  const [hasSubmitted, setHasSubmitted] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | undefined>(undefined);
  const errors = hasSubmitted ? validate(pipeline) : {};
  const update = (patch: Partial<Pipeline>) => setPipeline((current) => ({ ...current, ...patch }));

  const submit = async () => {
    setHasSubmitted(true);
    if (Object.keys(validate(pipeline)).length > 0) return;
    setIsSaving(true);
    setSaveError(undefined);
    try {
      await savePipeline(pipeline);
      toast(isEditing ? "Pipeline updated" : "Pipeline created");
      onOpenChange(false);
    } catch {
      setSaveError("Could not save the pipeline. Check your connection and try again.");
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <Modal
      header={isEditing ? "Edit pipeline" : "New pipeline"}
      size={ModalSize.MEDIUM}
      isOpen={isOpen}
      onOpenChange={onOpenChange}
      isDismissable={!isSaving}
      footer={
        <>
          <Button label="Cancel" variant={ButtonVariant.SECONDARY} isDisabled={isSaving} onClick={() => onOpenChange(false)} />
          <Button label={isEditing ? "Save changes" : "Create pipeline"} isLoading={isSaving} onClick={submit} />
        </>
      }
    >
      <Flex direction={FlexDirection.COLUMN} gap={16}>
        {saveError && <Alert variant={AlertVariant.ERROR}>{saveError}</Alert>}
        <TextInput
          label="Name"
          value={pipeline.name}
          onChange={(name) => update({ name })}
          isRequired
          autoFocus
          fillWidth
          {...(errors.name ? { error: errors.name } : {})}
        />
        <Field
          label="Description"
          isOptional
          maxLength={280}
          count={pipeline.description.length}
          {...(errors.description ? { error: errors.description } : {})}
        >
          <TextAreaInput value={pipeline.description} onChange={(description) => update({ description })} minRows={2} isAutoGrow fillWidth />
        </Field>
        <RadioGroup
          label="Sync mode"
          options={[
            { id: "incremental", label: "Incremental", description: "Only rows that changed since the last run." },
            { id: "full", label: "Full refresh", description: "Every row, every run." },
          ]}
          value={pipeline.mode}
          onChange={(mode) => update({ mode })}
        />
        <NumberInput
          label="Retries"
          value={pipeline.retries}
          onChange={(retries) => update({ retries })}
          min={0}
          max={10}
          hasSteppers
        />
      </Flex>
    </Modal>
  );
}
```

## Dashboard

KPI tiles, charts in `Widget`s with `isFlush`, linked hover across charts that share the time axis, and a list card.

```tsx
import { ArrowRightIcon } from "@phosphor-icons/react";
import Beacon, { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import AreaChart from "@galaxy-io/dls/charts/AreaChart";
import BarChart from "@galaxy-io/dls/charts/BarChart";
import ChartGroupProvider from "@galaxy-io/dls/charts/ChartGroupProvider";
import Sparkline, { SparklineVariant } from "@galaxy-io/dls/charts/Sparkline";
import StatChart from "@galaxy-io/dls/charts/StatChart";
import { ChartPalette } from "@galaxy-io/dls/charts/types";
import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Grid from "@galaxy-io/dls/layout/Grid";
import ToggleInput, { ToggleInputSize } from "@galaxy-io/dls/inputs/ToggleInput";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget from "@galaxy-io/dls/widget/Widget";
import { formatNumber } from "@galaxy-io/dls/utils/format";

const DAYS = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];
const ROWS = [120, 180, 160, 240, 210, 90, 70];
const FAILED = [2, 0, 1, 4, 1, 0, 0];

export function Dashboard({ isLoading }: { isLoading: boolean }) {
  return (
    <Flex direction={FlexDirection.COLUMN} gap={16} padding={24}>
      <Flex justifyContent={JustifyContent.SPACE_BETWEEN} alignItems={AlignItems.CENTER}>
        <Text as="h1" size={TextSize.HEADING_MD} weight={TextWeight.MEDIUM}>
          Overview
        </Text>
        <ToggleInput
          ariaLabel="Period"
          size={ToggleInputSize.SMALL}
          options={[
            { id: "7d", label: "7 days" },
            { id: "30d", label: "30 days" },
            { id: "90d", label: "90 days" },
          ]}
          defaultValue="7d"
        />
      </Flex>

      <Grid columns={4} gap={12}>
        <StatChart label="Rows synced" value="1.07M" description="+4% vs last week" trailing={<Sparkline data={ROWS} />} hasBorder isLoading={isLoading} />
        <StatChart label="Runs" value={formatNumber(312)} description="44 per day" hasBorder isLoading={isLoading} />
        <StatChart
          label="Failed runs"
          value={8}
          trailing={<Chip label="3 new" variant={ChipVariant.ERROR} size={ChipSize.SMALL} />}
          hasBorder
          isLoading={isLoading}
        />
        <StatChart
          label="Median duration"
          value="6m 12s"
          trailing={<Sparkline data={[7, 6.5, 6.8, 6.1, 6.2]} variant={SparklineVariant.SUCCESS} />}
          hasBorder
          isLoading={isLoading}
        />
      </Grid>

      <ChartGroupProvider>
        <Grid columns="2fr 1fr" gap={12}>
          <Widget header="Rows synced" subheader="By source type" isFlush isLoading={isLoading}>
            <Box height={240} padding={[12, 16]}>
              <AreaChart
                ariaLabel="Rows synced by source type, last 7 days"
                isStacked
                series={{ databases: { label: "Databases" }, apps: { label: "Apps", color: ChartPalette.TEAL } }}
                areas={[
                  { metric: "databases", points: DAYS.map((x, index) => ({ x, y: Math.round((ROWS[index] ?? 0) * 0.7) })) },
                  { metric: "apps", points: DAYS.map((x, index) => ({ x, y: Math.round((ROWS[index] ?? 0) * 0.3) })) },
                ]}
                valueFormatter={(value) => `${formatNumber(value, { compact: true })}k`}
              />
            </Box>
          </Widget>
          <Widget header="Failed runs" isFlush isLoading={isLoading}>
            <Box height={240} padding={[12, 16]}>
              <BarChart
                ariaLabel="Failed runs per day, last 7 days"
                hasLegend={false}
                series={{ failed: { label: "Failed", color: ChartPalette.ERROR } }}
                groups={DAYS.map((label, index) => ({
                  label,
                  bars: [{ metric: "failed" as const, components: [{ key: "failed", label: "Failed", value: FAILED[index] ?? 0 }] }],
                }))}
              />
            </Box>
          </Widget>
        </Grid>
      </ChartGroupProvider>

      <Widget
        header="Sources needing attention"
        actions={<Button label="View all" icon={ArrowRightIcon} isIconTrailing variant={ButtonVariant.TERTIARY} size={ButtonSize.SMALL} href="/sources" />}
      >
        <Flex direction={FlexDirection.COLUMN} gap={8}>
          <Flex justifyContent={JustifyContent.SPACE_BETWEEN} alignItems={AlignItems.CENTER}>
            <Text>orders_db</Text>
            <Beacon label="Failing" variant={BeaconVariant.ERROR} />
          </Flex>
          <Flex justifyContent={JustifyContent.SPACE_BETWEEN} alignItems={AlignItems.CENTER}>
            <Text>salesforce</Text>
            <Beacon label="Degraded" variant={BeaconVariant.WARNING} />
          </Flex>
          <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
            Updated 2 min ago
          </Text>
        </Flex>
      </Widget>
    </Flex>
  );
}
```

## Detail page with tabs

Breadcrumbs in the `Topbar`, a header with the object's actions, `Tabs` for peer views, a `DescriptionList` and a `Timeline`.

```tsx
import { ArrowsClockwiseIcon, DatabaseIcon, DotsThreeIcon, PencilSimpleIcon, TrashIcon } from "@phosphor-icons/react";
import Avatar, { AvatarSize } from "@galaxy-io/dls/avatar/Avatar";
import Beacon, { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Icon from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Grid from "@galaxy-io/dls/layout/Grid";
import DescriptionList from "@galaxy-io/dls/lists/DescriptionList";
import Menu, { MenuItem, MenuItemVariant, MenuSeparator } from "@galaxy-io/dls/menu/Menu";
import Breadcrumbs from "@galaxy-io/dls/navigation/Breadcrumbs";
import Tabs from "@galaxy-io/dls/navigation/Tabs";
import Topbar from "@galaxy-io/dls/navigation/Topbar";
import CodeBlock, { CodeBlockLanguage } from "@galaxy-io/dls/text/CodeBlock";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import Timeline, { TimelineVariant } from "@galaxy-io/dls/timeline/Timeline";
import Widget from "@galaxy-io/dls/widget/Widget";

function OverviewTab() {
  return (
    <Grid columns="minmax(0, 2fr) minmax(0, 1fr)" gap={16}>
      <Widget header="Details">
        <DescriptionList
          columns={2}
          items={[
            { label: "Status", value: <Beacon label="Healthy" variant={BeaconVariant.SUCCESS} /> },
            { label: "Source id", value: "src_8f2c91d0", family: FontFamily.MONO, isCopyable: true },
            { label: "Host", value: "orders.internal:5432", family: FontFamily.MONO },
            { label: "Tables", value: 12 },
            { label: "Owner", value: <Avatar name="Ada Lovelace" size={AvatarSize.X_SMALL} /> },
            { label: "Last error", value: null },
          ]}
        />
      </Widget>
      <Widget header="Recent activity">
        <Timeline
          items={[
            { id: "r3", label: "Run 4812 finished", time: "14:09", variant: TimelineVariant.SUCCESS },
            { id: "r2", label: "Schema changed", description: "orders.discount_code added", time: "14:03", variant: TimelineVariant.WARNING },
            { id: "r1", label: "Run 4812 started", time: "14:02" },
          ]}
        />
      </Widget>
    </Grid>
  );
}

export function SourceDetailPage() {
  return (
    <Flex direction={FlexDirection.COLUMN} height="100%">
      <Topbar>
        <Breadcrumbs
          items={[
            { label: "Sources", href: "/sources" },
            { label: "orders_db" },
          ]}
        />
      </Topbar>
      <Flex direction={FlexDirection.COLUMN} gap={16} padding={24} grow={1} minHeight={0} overflow="auto">
        <Flex justifyContent={JustifyContent.SPACE_BETWEEN} alignItems={AlignItems.CENTER}>
          <Flex gap={8} alignItems={AlignItems.CENTER}>
            <Icon component={DatabaseIcon} size={16} />
            <Text as="h1" size={TextSize.HEADING_MD} weight={TextWeight.MEDIUM}>
              orders_db
            </Text>
            <Text variant={TextVariant.TERTIARY}>Postgres</Text>
          </Flex>
          <Flex gap={8}>
            <Button label="Edit" icon={PencilSimpleIcon} variant={ButtonVariant.SECONDARY} />
            <Button label="Sync now" icon={ArrowsClockwiseIcon} />
            <Menu trigger={<Button icon={DotsThreeIcon} ariaLabel="More actions" tooltip="More actions" variant={ButtonVariant.TERTIARY} />}>
              <MenuItem label="Duplicate" onSelect={() => {}} />
              <MenuSeparator />
              <MenuItem label="Delete source" icon={TrashIcon} variant={MenuItemVariant.ERROR} onSelect={() => {}} />
            </Menu>
          </Flex>
        </Flex>
        <Tabs
          ariaLabel="Source"
          items={[
            { id: "overview", label: "Overview", panel: <OverviewTab /> },
            { id: "runs", label: "Runs", count: 48, panel: <Text>Runs</Text> },
            {
              id: "config",
              label: "Configuration",
              panel: <CodeBlock language={CodeBlockLanguage.YAML} label="source.yaml" content={"host: orders.internal\nport: 5432\nmode: incremental"} canCopy />,
            },
          ]}
        />
      </Flex>
    </Flex>
  );
}
```

## Empty state

A page that has nothing yet, and the same region after a search with no results.

```tsx
import { MagnifyingGlassIcon, PlugsIcon } from "@phosphor-icons/react";
import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import EmptyState, { EmptyStateSize } from "@galaxy-io/dls/feedback/EmptyState";
import Flex from "@galaxy-io/dls/layout/Flex";
import Link from "@galaxy-io/dls/links/Link";

export const NoSources = ({ onConnect }: { onConnect: () => void }) => (
  <Flex padding={48} fillWidth>
    <EmptyState
      size={EmptyStateSize.LARGE}
      icon={PlugsIcon}
      header="No sources yet"
      description="Connect a database or an app. Galaxy keeps it in sync with your warehouse."
      actions={
        <>
          <Button label="Connect source" onClick={onConnect} />
          <Link href="https://docs.getgalaxy.io/sources" isExternal>
            Read the guide
          </Link>
        </>
      }
    />
  </Flex>
);

export const NoResults = ({ term, onClear }: { term: string; onClear: () => void }) => (
  <EmptyState
    role="status"
    icon={MagnifyingGlassIcon}
    header={`No results for “${term}”`}
    description="Check the spelling or search by host or owner."
    actions={<Button label="Clear search" variant={ButtonVariant.SECONDARY} onClick={onClear} />}
  />
);
```

## Command palette

One palette at the app root, bound to ⌘K / Ctrl+K, with navigation and actions, recents, and a matching hint in the `Topbar`.

```tsx
import type { ReactNode } from "react";
import { DatabaseIcon, FlowArrowIcon, GearIcon, HouseIcon, MoonIcon, PlusIcon } from "@phosphor-icons/react";
import { useDisclosure } from "@galaxy-io/dls/hooks/useDisclosure";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import CommandPalette, { type CommandPaletteItem } from "@galaxy-io/dls/navigation/CommandPalette";
import Topbar from "@galaxy-io/dls/navigation/Topbar";
import { GalaxyTheme } from "@galaxy-io/dls/theme/enums";
import { useGalaxyTheme } from "@galaxy-io/dls/theme/useGalaxyTheme";

declare function navigate(path: string): void;
declare function openNewSourceDialog(): void;

export function AppFrame({ title, children }: { title: string; children: ReactNode }) {
  const palette = useDisclosure();
  const { activeTheme, setTheme } = useGalaxyTheme();

  const items: CommandPaletteItem[] = [
    { id: "new-source", label: "New source", icon: PlusIcon, group: "Create", onSelect: openNewSourceDialog },
    { id: "go-overview", label: "Overview", icon: HouseIcon, group: "Go to", onSelect: () => navigate("/") },
    { id: "go-sources", label: "Sources", icon: DatabaseIcon, group: "Go to", onSelect: () => navigate("/sources") },
    { id: "go-pipelines", label: "Pipelines", icon: FlowArrowIcon, group: "Go to", onSelect: () => navigate("/pipelines") },
    {
      id: "go-settings",
      label: "Settings",
      icon: GearIcon,
      group: "Go to",
      keywords: ["preferences", "account"],
      hotKeys: ["mod", ","],
      onSelect: () => navigate("/settings"),
    },
    {
      id: "toggle-theme",
      label: activeTheme === GalaxyTheme.DARK ? "Switch to light theme" : "Switch to dark theme",
      icon: MoonIcon,
      group: "Preferences",
      onSelect: () => setTheme(activeTheme === GalaxyTheme.DARK ? GalaxyTheme.LIGHT : GalaxyTheme.DARK),
    },
  ];

  return (
    <>
      <Topbar
        actions={
          <Button
            label="Search"
            variant={ButtonVariant.SECONDARY}
            size={ButtonSize.SMALL}
            hotKeys={["mod", "k"]}
            onClick={palette.open}
          />
        }
      >
        {title}
      </Topbar>
      {children}
      <CommandPalette
        items={items}
        isOpen={palette.isOpen}
        onOpenChange={palette.setIsOpen}
        shouldBindHotKey
        storageKey="galaxy:command-palette"
      />
    </>
  );
}
```
