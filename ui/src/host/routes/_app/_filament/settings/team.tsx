import { createFileRoute } from "@tanstack/react-router";

import TeamPage from "@/pages/settings/TeamPage";

import { teamSearchSchema } from "@/module/schemas";

export const Route = createFileRoute("/_app/_filament/settings/team")({
  validateSearch: teamSearchSchema,
  component: TeamPage,
});
