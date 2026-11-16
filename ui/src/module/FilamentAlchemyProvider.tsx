import { createContext, type FC, type PropsWithChildren, useContext } from "react";

const FilamentAlchemyUrlContext = createContext<string | undefined>(undefined);

interface FilamentAlchemyProviderProps {
  alchemyUrl?: string;
}

export const FilamentAlchemyProvider: FC<PropsWithChildren<FilamentAlchemyProviderProps>> = ({
  alchemyUrl,
  children,
}) => (
  <FilamentAlchemyUrlContext.Provider value={alchemyUrl}>
    {children}
  </FilamentAlchemyUrlContext.Provider>
);

export const useFilamentAlchemyUrl = (): string | undefined =>
  useContext(FilamentAlchemyUrlContext);
