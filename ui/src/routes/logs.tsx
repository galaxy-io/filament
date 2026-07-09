import { createFileRoute } from "@tanstack/react-router";

import PageLayout from "@/layouts/PageLayout";

export const Route = createFileRoute("/logs")({
  component: LogsPage,
});

function LogsPage() {
  return (
    <PageLayout title="Logs" isEmpty emptyMessage="Run logs will land here — TailRun streaming is next.">
      {null}
    </PageLayout>
  );
}
