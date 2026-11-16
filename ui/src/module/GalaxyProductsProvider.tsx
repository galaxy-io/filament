import { createContext, type FC, type PropsWithChildren, useContext } from "react";

import { GALAXY_DEFAULT_PRODUCT_URLS } from "@/module/constants";
import type { GalaxyProduct, GalaxyProductUrls } from "@/module/types";

const GalaxyProductUrlsContext = createContext<GalaxyProductUrls>(GALAXY_DEFAULT_PRODUCT_URLS);

interface GalaxyProductsProviderProps {
  productUrls: GalaxyProductUrls;
}

export const GalaxyProductsProvider: FC<PropsWithChildren<GalaxyProductsProviderProps>> = ({
  productUrls,
  children,
}) => (
  <GalaxyProductUrlsContext.Provider value={productUrls}>
    {children}
  </GalaxyProductUrlsContext.Provider>
);

export const useGalaxyProductUrl = (product: GalaxyProduct): string | undefined =>
  useContext(GalaxyProductUrlsContext)[product];
