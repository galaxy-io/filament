import { useMemo, useState } from "react";

import BarChart from "@galaxy-io/dls/charts/BarChart";
import type { BarChartGroupDatum, ChartSeriesStyles } from "@galaxy-io/dls/charts/types";
import { ChartPalette } from "@galaxy-io/dls/charts/types";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import MultiSelectInput from "@galaxy-io/dls/inputs/MultiSelectInput";
import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import Switcher, { type SwitcherItem } from "@galaxy-io/dls/switcher/Switcher";
import InfiniteTable, { ColumnAlign, type ColumnDef } from "@galaxy-io/dls/table/InfiniteTable";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import Widget from "@galaxy-io/dls/widget/Widget";

import { RunStatus } from "@/gen/ingestion/v1/runs_pb";

import BaseToolbar from "@/layouts/components/BaseToolbar";

import { PIPELINE_RUN_STATUS_TO_LABEL_MAP } from "@/pages/pipelines/history/constants";
import PipelineHistoryRunStatus from "@/pages/pipelines/history/PipelineHistoryRunStatus";

import { formatBytes, formatCount, formatDuration, formatTimestamp } from "@/utils/format";

enum ObservabilityTimeframe {
  TWENTY_FOUR_HOURS = "24H",
  SEVEN_DAYS = "7D",
  THIRTY_DAYS = "30D",
}

type RunMetric = "runs";

const RUNS_SERIES: ChartSeriesStyles<RunMetric> = {
  runs: { label: "Runs" },
};

const STATUS_COLORS: Partial<Record<RunStatus, ChartPalette>> = {
  [RunStatus.COMPLETED]: ChartPalette.GREEN,
  [RunStatus.FAILED]: ChartPalette.RED,
  [RunStatus.RUNNING]: ChartPalette.BLUE,
  [RunStatus.REQUESTED]: ChartPalette.YELLOW,
  [RunStatus.CANCELED]: ChartPalette.PURPLE,
  [RunStatus.PAUSED]: ChartPalette.ORANGE,
  [RunStatus.PARTIAL]: ChartPalette.TEAL,
};

const ALL_STATUSES = [
  RunStatus.COMPLETED,
  RunStatus.FAILED,
  RunStatus.RUNNING,
  RunStatus.REQUESTED,
  RunStatus.CANCELED,
  RunStatus.PAUSED,
  RunStatus.PARTIAL,
];

const STATUS_OPTIONS: SelectInputOption[] = ALL_STATUSES.map((status) => ({
  id: String(status),
  label: PIPELINE_RUN_STATUS_TO_LABEL_MAP[status],
  value: status,
}));

const DEFAULT_SELECTED_STATUSES: SelectInputOption[] = [
  {
    id: String(RunStatus.COMPLETED),
    label: PIPELINE_RUN_STATUS_TO_LABEL_MAP[RunStatus.COMPLETED],
    value: RunStatus.COMPLETED,
  },
  {
    id: String(RunStatus.FAILED),
    label: PIPELINE_RUN_STATUS_TO_LABEL_MAP[RunStatus.FAILED],
    value: RunStatus.FAILED,
  },
];

const generateHourlyData = (selectedStatuses: RunStatus[]): BarChartGroupDatum<RunMetric>[] => {
  return Array.from({ length: 24 }, (_, i) => ({
    label: `${i.toString().padStart(2, "0")}:00`,
    bars: [
      {
        metric: "runs" as const,
        components: selectedStatuses.map((status) => ({
          key: String(status),
          label: PIPELINE_RUN_STATUS_TO_LABEL_MAP[status],
          value: Math.floor(Math.random() * 20) + 1,
          color: STATUS_COLORS[status],
        })),
      },
    ],
  }));
};

const generateDailyData = (
  days: number,
  selectedStatuses: RunStatus[],
): BarChartGroupDatum<RunMetric>[] => {
  const today = new Date();
  return Array.from({ length: days }, (_, i) => {
    const date = new Date(today);
    date.setDate(date.getDate() - (days - 1 - i));
    const label = `${(date.getMonth() + 1).toString().padStart(2, "0")}/${date.getDate().toString().padStart(2, "0")}`;
    return {
      label,
      bars: [
        {
          metric: "runs" as const,
          components: selectedStatuses.map((status) => ({
            key: String(status),
            label: PIPELINE_RUN_STATUS_TO_LABEL_MAP[status],
            value: Math.floor(Math.random() * 100) + 5,
            color: STATUS_COLORS[status],
          })),
        },
      ],
    };
  });
};

interface MockRunInfo {
  runId: string;
  pipelineId: string;
  pipelineName: string;
  status: RunStatus;
  startedAt: bigint;
  endedAt: bigint;
  records: bigint;
  bytes: bigint;
  error: string;
}

const MOCK_PIPELINE_NAMES = [
  "salesforce-to-snowflake",
  "hubspot-contacts-sync",
  "stripe-payments-etl",
  "postgres-to-bigquery",
  "mongodb-analytics",
];

const generateMockRuns = (): MockRunInfo[] => {
  const now = Date.now();
  const runs: MockRunInfo[] = [];

  for (let i = 0; i < 50; i++) {
    const startedAt = now - Math.floor(Math.random() * 24 * 60 * 60 * 1000);
    const duration = Math.floor(Math.random() * 30 * 60 * 1000) + 1000;
    const status = ALL_STATUSES[Math.floor(Math.random() * ALL_STATUSES.length)];
    const isTerminal = [RunStatus.COMPLETED, RunStatus.FAILED, RunStatus.CANCELED].includes(status);

    runs.push({
      runId: `run_${Math.random().toString(36).substring(2, 15)}`,
      pipelineId: `pipe_${Math.random().toString(36).substring(2, 10)}`,
      pipelineName: MOCK_PIPELINE_NAMES[Math.floor(Math.random() * MOCK_PIPELINE_NAMES.length)],
      status,
      startedAt: BigInt(startedAt),
      endedAt: isTerminal ? BigInt(startedAt + duration) : BigInt(0),
      records: BigInt(Math.floor(Math.random() * 100000)),
      bytes: BigInt(Math.floor(Math.random() * 100000000)),
      error: status === RunStatus.FAILED ? "Connection timeout" : "",
    });
  }

  return runs.sort((a, b) => Number(b.startedAt - a.startedAt));
};

const RUNS_TABLE_COLUMNS: ColumnDef<MockRunInfo>[] = [
  {
    id: "status",
    header: "Status",
    size: 110,
    cellLoading: () => <TextShimmer width={64} height={18} />,
    cell: ({ row }) => (
      <PipelineHistoryRunStatus status={row.original.status} error={row.original.error} />
    ),
  },
  {
    id: "pipeline",
    header: "Pipeline",
    cellLoading: () => <TextShimmer width={120} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM} isEllipsis>
        {row.original.pipelineName}
      </Text>
    ),
  },
  {
    id: "startedAt",
    header: "Started",
    size: 140,
    cellLoading: () => <TextShimmer width={100} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} isEllipsis>
        {formatTimestamp(row.original.startedAt)}
      </Text>
    ),
  },
  {
    id: "duration",
    header: "Duration",
    size: 100,
    cellLoading: () => <TextShimmer width={60} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} isEllipsis>
        {formatDuration(row.original.startedAt, row.original.endedAt)}
      </Text>
    ),
  },
  {
    id: "records",
    header: "Records",
    size: 90,
    cellLoading: () => <TextShimmer width={48} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} isMonospace>
        {formatCount(row.original.records)}
      </Text>
    ),
  },
  {
    id: "volume",
    header: "Volume",
    size: 100,
    align: ColumnAlign.RIGHT,
    cellLoading: () => <TextShimmer width={52} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} isMonospace>
        {formatBytes(row.original.bytes)}
      </Text>
    ),
  },
];

interface ObservabilityPageState {
  selectedTimeframe: ObservabilityTimeframe;
  selectedStatuses: SelectInputOption[];
}

const DEFAULT_STATE: ObservabilityPageState = {
  selectedTimeframe: ObservabilityTimeframe.TWENTY_FOUR_HOURS,
  selectedStatuses: DEFAULT_SELECTED_STATUSES,
};

const ObservabilityPage = () => {
  const [state, setState] = useState<ObservabilityPageState>(DEFAULT_STATE);

  const selectedStatusValues = useMemo(
    () => state.selectedStatuses.map((s) => Number(s.id) as RunStatus),
    [state.selectedStatuses],
  );

  const mockRuns = useMemo(() => generateMockRuns(), []);

  const filteredRuns = useMemo(
    () => mockRuns.filter((run) => selectedStatusValues.includes(run.status)),
    [mockRuns, selectedStatusValues],
  );

  const chartData = useMemo(() => {
    if (selectedStatusValues.length === 0) {
      return [];
    }
    switch (state.selectedTimeframe) {
      case ObservabilityTimeframe.TWENTY_FOUR_HOURS:
        return generateHourlyData(selectedStatusValues);
      case ObservabilityTimeframe.SEVEN_DAYS:
        return generateDailyData(7, selectedStatusValues);
      case ObservabilityTimeframe.THIRTY_DAYS:
        return generateDailyData(30, selectedStatusValues);
    }
  }, [state.selectedTimeframe, selectedStatusValues]);

  const handleStatusChange = (value: SelectInputOption[]) => {
    setState({ ...state, selectedStatuses: value });
  };

  const selectedTimeframeItems: SwitcherItem[] = [
    {
      id: ObservabilityTimeframe.TWENTY_FOUR_HOURS,
      label: ObservabilityTimeframe.TWENTY_FOUR_HOURS,
      onClick: () => handleTimeframeChange(ObservabilityTimeframe.TWENTY_FOUR_HOURS),
    },
    {
      id: ObservabilityTimeframe.SEVEN_DAYS,
      label: ObservabilityTimeframe.SEVEN_DAYS,
      onClick: () => handleTimeframeChange(ObservabilityTimeframe.SEVEN_DAYS),
    },
    {
      id: ObservabilityTimeframe.THIRTY_DAYS,
      label: ObservabilityTimeframe.THIRTY_DAYS,
      onClick: () => handleTimeframeChange(ObservabilityTimeframe.THIRTY_DAYS),
    },
  ];

  const handleTimeframeChange = (timeframe: ObservabilityTimeframe) => {
    setState({ ...state, selectedTimeframe: timeframe });
  };

  return (
    <FlexWrapper
      direction={FlexDirection.COLUMN}
      padding={"12px"}
      gap={24}
      fillWidth
      fillHeight
      overflow="auto"
    >
      <Widget fillWidth noPadding>
        <BaseToolbar
          leadingActions={[
            <Text key="title" variant={TextVariant.PRIMARY} weight={TextWeight.MEDIUM}>
              Runs
            </Text>,
          ]}
          trailingActions={[
            <MultiSelectInput
              key="status-selector"
              options={STATUS_OPTIONS}
              value={state.selectedStatuses}
              onChange={handleStatusChange}
              placeholder="Select statuses..."
              width={200}
            />,
            <Switcher
              key="timeframe-switcher"
              items={selectedTimeframeItems}
              selectedId={state.selectedTimeframe}
            />,
          ]}
        />
        <HorizontalDivider />
        <FlexWrapper direction={FlexDirection.COLUMN} padding={"24px 12px"} height={250} fillWidth>
          <BarChart series={RUNS_SERIES} groups={chartData} fillWidth fillHeight />
        </FlexWrapper>
        <HorizontalDivider />
        <InfiniteTable<MockRunInfo>
          columns={RUNS_TABLE_COLUMNS}
          data={filteredRuns}
          getRowId={(run) => run.runId}
          contentWhenEmpty={
            <Text variant={TextVariant.TERTIARY}>No runs in the selected timeframe</Text>
          }
          fillWidth
          height={300}
        />
      </Widget>
    </FlexWrapper>
  );
};

export default ObservabilityPage;
