import {
  BookOpenTextIcon,
  GithubLogoIcon,
  type Icon as PhosphorIcon,
  SlackLogoIcon,
} from "@phosphor-icons/react";

import { GalaxyTheme } from "@galaxy-io/dls/theme/types";

import type { AuthLayoutAsideFieldPalette } from "@/layouts/auth/types";

import { DOCUMENTATION_URL, GITHUB_REPO_URL, SLACK_COMMUNITY_URL } from "@/constants";

export const AUTH_LAYOUT_INSET = 32;
export const AUTH_LAYOUT_CONTENT_WIDTH = 320;
export const AUTH_LAYOUT_ASIDE_BREAKPOINT = 1080;
export const AUTH_LAYOUT_ASIDE_CONTENT_WIDTH = 320;
export const AUTH_LAYOUT_ASIDE_LINK_PADDING = "12px 16px";

export const AUTH_LAYOUT_ASIDE_FIELD_SPACING = 13;
export const AUTH_LAYOUT_ASIDE_FIELD_NOISE_SCALE = 0.014;
export const AUTH_LAYOUT_ASIDE_FIELD_PATCH_THRESHOLD = 0.34;
export const AUTH_LAYOUT_ASIDE_FIELD_BASE_ALPHA = 0.03;
export const AUTH_LAYOUT_ASIDE_FIELD_PATCH_ALPHA = 0.14;
export const AUTH_LAYOUT_ASIDE_FIELD_GLOW_EXPONENT = 2.4;
export const AUTH_LAYOUT_ASIDE_FIELD_GLOW_FLOOR = 0.7;
export const AUTH_LAYOUT_ASIDE_FIELD_GLOW_GAIN = 1.9;
export const AUTH_LAYOUT_ASIDE_FIELD_GLOW_RADIUS = 0.55;
export const AUTH_LAYOUT_ASIDE_FIELD_DOT_RADIUS = 0.75;
export const AUTH_LAYOUT_ASIDE_FIELD_TINT_COUNT = 3;
export const AUTH_LAYOUT_ASIDE_FIELD_TINT_CHANCE = 0.05;
export const AUTH_LAYOUT_ASIDE_FIELD_TINT_GLOW_CHANCE = 0.55;
export const AUTH_LAYOUT_ASIDE_FIELD_ACCENT_TINT = 1;
export const AUTH_LAYOUT_ASIDE_FIELD_ACCENT_THRESHOLD = 0.12;
export const AUTH_LAYOUT_ASIDE_FIELD_ACCENT_GAIN = 1.4;

export const AUTH_LAYOUT_ASIDE_FIELD_SPRITE_STEPS = 12;
export const AUTH_LAYOUT_ASIDE_FIELD_SPRITE_MIN_RADIUS = 0.6;
export const AUTH_LAYOUT_ASIDE_FIELD_SPRITE_STEP_RADIUS = 0.28;
export const AUTH_LAYOUT_ASIDE_FIELD_HALO_RADIUS = 170;
export const AUTH_LAYOUT_ASIDE_FIELD_HALO_ALPHA = 0.14;

export const AUTH_LAYOUT_ASIDE_FIELD_POINTER_RADIUS = 118;
export const AUTH_LAYOUT_ASIDE_FIELD_POINTER_CHARGE_RADIUS = 96;
export const AUTH_LAYOUT_ASIDE_FIELD_POINTER_EASE = 13;
export const AUTH_LAYOUT_ASIDE_FIELD_POINTER_GAIN = 7;
export const AUTH_LAYOUT_ASIDE_FIELD_POINTER_PUSH = 620;
export const AUTH_LAYOUT_ASIDE_FIELD_POINTER_PULL = 1700;

export const AUTH_LAYOUT_ASIDE_FIELD_RIPPLE_SPEED = 460;
export const AUTH_LAYOUT_ASIDE_FIELD_RIPPLE_WIDTH = 54;
export const AUTH_LAYOUT_ASIDE_FIELD_RIPPLE_LIFE = 1.2;
export const AUTH_LAYOUT_ASIDE_FIELD_RIPPLE_LIMIT = 4;
export const AUTH_LAYOUT_ASIDE_FIELD_RIPPLE_GAIN = 3.4;
export const AUTH_LAYOUT_ASIDE_FIELD_RIPPLE_PUSH = 1100;

export const AUTH_LAYOUT_ASIDE_FIELD_ENERGY_DECAY = 2.4;
export const AUTH_LAYOUT_ASIDE_FIELD_ENERGY_ALPHA = 0.55;
export const AUTH_LAYOUT_ASIDE_FIELD_ENERGY_RADIUS = 1.9;
export const AUTH_LAYOUT_ASIDE_FIELD_SPRING = 90;
export const AUTH_LAYOUT_ASIDE_FIELD_DAMPING = 5.5;
export const AUTH_LAYOUT_ASIDE_FIELD_DRIFT_SPEED = 0.55;
export const AUTH_LAYOUT_ASIDE_FIELD_DRIFT_AMPLITUDE = 0.7;
export const AUTH_LAYOUT_ASIDE_FIELD_DRIFT_ALPHA = 0.22;
export const AUTH_LAYOUT_ASIDE_FIELD_MIN_ALPHA = 0.012;
export const AUTH_LAYOUT_ASIDE_FIELD_MAX_DELTA = 1 / 30;

export const AUTH_LAYOUT_ASIDE_FIELD_PALETTE_MAP: Record<
  Exclude<GalaxyTheme, GalaxyTheme.SYSTEM>,
  AuthLayoutAsideFieldPalette
> = {
  [GalaxyTheme.DARK]: {
    ink: "#0b0b0c",
    tints: ["#005eff", "#6d4aff", "#0d9aa8"],
    halo: [0, 94, 255],
  },
  [GalaxyTheme.LIGHT]: {
    ink: "#ffffff",
    tints: ["#3075ff", "#a79af2", "#5fd0c3"],
    halo: [48, 117, 255],
  },
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
