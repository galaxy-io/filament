import { useNavigate } from "@tanstack/react-router";

/**
 * Hook to open the connection drawer from anywhere in the app.
 * Updates the URL search params which triggers the drawer at the root level.
 */
export const useOpenConnectionDrawer = () => {
  const navigate = useNavigate();

  const openConnectionDrawer = (connectionId: string) => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, connectionId }),
    });
  };

  return openConnectionDrawer;
};
