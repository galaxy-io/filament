import type { FileRoutesByTo } from "@/routeTree.gen";
import { useMatchRoute } from "@tanstack/react-router";

export type TRoutes = keyof FileRoutesByTo;

export const useRouteMatch = ({
  route,
  fuzzy = false,
}: {
  route: TRoutes;
  fuzzy?: boolean;
}): { isRouteMatch: boolean } => {
  const matchRoute = useMatchRoute();

  const isRouteMatch = matchRoute({ to: route, fuzzy }) !== false;

  return { isRouteMatch };
};
