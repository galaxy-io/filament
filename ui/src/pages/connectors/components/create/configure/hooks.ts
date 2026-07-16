import { useContext } from "react";

import { CreateConnectionConfigureContext } from "@/pages/connectors/components/create/configure/CreateConnectionConfigureProvider";

export function useCreateConnectionContext() {
  const context = useContext(CreateConnectionConfigureContext);
  if (!context) {
    throw new Error(
      "useCreateConnectionContext must be used within CreateConnectionConfigureProvider",
    );
  }
  return context;
}
