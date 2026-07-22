import { createFileRoute } from "@tanstack/react-router";

import PipelineCanvas from "@/pages/pipelines/canvas/PipelineCanvas";

// version pins the canvas to a read-only preview of that graph version; absent
// means latest (editable). Scoped to the canvas route so navigating to
// history/settings always operates on the latest version.
export const Route = createFileRoute("/pipelines/$id/canvas")({
  validateSearch: (search: Record<string, unknown>): { version?: number } => {
    const version = Number(search.version);
    return Number.isInteger(version) && version > 0 ? { version } : {};
  },
  component: PipelineCanvas,
});
