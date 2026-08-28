export const IS_DEBUG = import.meta.env.DEV;
export const IS_PRODUCTION = import.meta.env.PROD;

export const NOOP = () => {};

export const API_URL: string = import.meta.env.VITE_API_URL ?? "";

export const GITHUB_REPO_URL: string = "https://github.com/galaxy-io/filament";
export const DOCUMENTATION_URL: string = "https://filament.getgalaxy.io";
export const SLACK_COMMUNITY_URL: string =
  "https://join.slack.com/t/galaxy-filament/shared_invite/zt-45qln0zg5-9CTvKVXugvh3sEhpD6KzJQ";
