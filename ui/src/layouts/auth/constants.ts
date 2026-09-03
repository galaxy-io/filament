import {
  BookOpenTextIcon,
  GithubLogoIcon,
  type Icon as PhosphorIcon,
  SlackLogoIcon,
} from "@phosphor-icons/react";

import { GalaxyTheme } from "@galaxy-io/dls/theme/types";

import type {
  AuthLayoutAsideFieldPalette,
  AuthLayoutAsideFieldRipple,
  AuthLayoutAsideFieldWave,
} from "@/layouts/auth/types";

import { DOCUMENTATION_URL, GITHUB_REPO_URL, SLACK_COMMUNITY_URL } from "@/constants";

export const AUTH_LAYOUT_INSET = 32;
export const AUTH_LAYOUT_CONTENT_WIDTH = 320;
export const AUTH_LAYOUT_ASIDE_BREAKPOINT = 1080;
export const AUTH_LAYOUT_ASIDE_CONTENT_WIDTH = 320;
export const AUTH_LAYOUT_ASIDE_LINK_PADDING = "12px 16px";

export const AUTH_LAYOUT_ASIDE_FIELD_SPACING = 12;
export const AUTH_LAYOUT_ASIDE_FIELD_DOT_RADIUS = 0.5;
export const AUTH_LAYOUT_ASIDE_FIELD_BASE = 0.88;
export const AUTH_LAYOUT_ASIDE_FIELD_PULSE = 1;
export const AUTH_LAYOUT_ASIDE_FIELD_JITTER = 0.9;
export const AUTH_LAYOUT_ASIDE_FIELD_DRIFT_SCALE = 0.0055;
export const AUTH_LAYOUT_ASIDE_FIELD_DRIFT_X = 231;
export const AUTH_LAYOUT_ASIDE_FIELD_DRIFT_Y = -168;
export const AUTH_LAYOUT_ASIDE_FIELD_DRIFT_WEIGHT = 0.9;
export const AUTH_LAYOUT_ASIDE_FIELD_MAX_DELTA = 1 / 30;

export const AUTH_LAYOUT_ASIDE_FIELD_WAVES: AuthLayoutAsideFieldWave[] = [
  { directionX: 0.72, directionY: 0.69, length: 260, speed: 112, weight: 0.7 },
  { directionX: -0.4, directionY: 0.92, length: 380, speed: -104, weight: 0.55 },
];

export const AUTH_LAYOUT_ASIDE_FIELD_RIPPLES: AuthLayoutAsideFieldRipple[] = [
  { originX: 0.85, originY: 0.12, length: 250, speed: 120, weight: 0.9 },
  { originX: 0.08, originY: 0.78, length: 340, speed: -115, weight: 0.75 },
  { originX: 0.55, originY: 1.15, length: 430, speed: 129, weight: 0.6 },
];

export const AUTH_LAYOUT_ASIDE_FIELD_PALETTE_MAP: Record<
  Exclude<GalaxyTheme, GalaxyTheme.SYSTEM>,
  AuthLayoutAsideFieldPalette
> = {
  [GalaxyTheme.DARK]: { color: "#000000", alpha: 0.16 },
  [GalaxyTheme.LIGHT]: { color: "#ffffff", alpha: 0.32 },
};

interface AuthLayoutAsideLink {
  icon: PhosphorIcon;
  label: string;
  description: string;
  url: string;
}

export const AUTH_LAYOUT_ASIDE_LINKS: AuthLayoutAsideLink[] = [
  {
    icon: BookOpenTextIcon,
    label: "Documentation",
    description: "Guides, connectors, and deployment",
    url: DOCUMENTATION_URL,
  },
  {
    icon: GithubLogoIcon,
    label: "GitHub",
    description: "Read the source, open an issue",
    url: GITHUB_REPO_URL,
  },
  {
    icon: SlackLogoIcon,
    label: "Slack community",
    description: "Ask questions, share what you build",
    url: SLACK_COMMUNITY_URL,
  },
];
