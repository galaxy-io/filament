export const IS_DEBUG = import.meta.env.DEV;
export const IS_PRODUCTION = import.meta.env.PROD;

export const NOOP = () => {};

/**
 * Base URL for the ConnectRPC API.
 *
 * Defaults to same-origin: in dev the Vite server proxies
 * /ingestion.v1.IngestionService to the Go server, and in production the Go
 * binary serves the embedded UI and the API from the same address. Set
 * VITE_API_URL to point at a remote Filament instance instead.
 */
export const API_BASE_URL: string = import.meta.env.VITE_API_URL ?? "";

export const APP_VERSION = "v0.0.1-alpha";

export const GITHUB_REPO_URL = "https://github.com/galaxy-io/filament";

export const CONNECTORS_DOCS_URL = "https://filament.getgalaxy.io/connectors";
