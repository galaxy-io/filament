import type { FC } from "react";

import {
  CalendarCheckIcon,
  CalendarXIcon,
  GearIcon,
  PencilSimpleIcon,
  PlayIcon,
} from "@phosphor-icons/react";

import { MenuItem, MenuSeparator } from "@galaxy-io/dls/menu/Menu";

import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";
import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import { ACTIVE_RUN_STATUSES } from "@/constants";

interface PipelinesTableRowActionsProps {
  pipeline: Pipeline;
  onRun: (pipeline: Pipeline) => void;
  onScheduleToggle: (pipeline: Pipeline) => void;
  onEdit: (pipeline: Pipeline) => void;
  onSettings: (pipeline: Pipeline) => void;
}

const PipelinesTableRowActions: FC<PipelinesTableRowActionsProps> = ({
  pipeline,
  onRun,
  onScheduleToggle,
  onEdit,
  onSettings,
}) => {
  const isActive = pipeline.lastRun ? ACTIVE_RUN_STATUSES.has(pipeline.lastRun.status) : false;
  const scheduleConfig =
    pipeline.executionMode === ExecutionMode.CONTINUOUS ? undefined : pipeline.schedule?.config;

  return (
    <>
      <MenuItem
        label="Run now"
        icon={PlayIcon}
        onSelect={() => onRun(pipeline)}
        isDisabled={isActive}
      />
      {scheduleConfig && (
        <MenuItem
          label={scheduleConfig.isEnabled ? "Pause schedule" : "Resume schedule"}
          icon={scheduleConfig.isEnabled ? CalendarXIcon : CalendarCheckIcon}
          onSelect={() => onScheduleToggle(pipeline)}
        />
      )}
      <MenuSeparator />
      <MenuItem label="Edit pipeline" icon={PencilSimpleIcon} onSelect={() => onEdit(pipeline)} />
      <MenuItem label="Settings" icon={GearIcon} onSelect={() => onSettings(pipeline)} />
    </>
  );
};

export default PipelinesTableRowActions;
