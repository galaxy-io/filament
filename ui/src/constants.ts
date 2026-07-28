export const IS_DEBUG = import.meta.env.DEV;
export const IS_PRODUCTION = import.meta.env.PROD;

export const NOOP = () => {};

export const API_URL: string = import.meta.env.VITE_API_URL ?? "";

export const GITHUB_REPO_URL: string = "https://github.com/galaxy-io/filament";
export const DOCUMENTATION_URL: string = "https://filament.getgalaxy.io";

export enum Flow {
  CREATE_CONNECTION = "CREATE_CONNECTION",
}
