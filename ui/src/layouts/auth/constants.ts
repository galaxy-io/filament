import {
  BookOpenTextIcon,
  GithubLogoIcon,
  type Icon as PhosphorIcon,
  SlackLogoIcon,
} from "@phosphor-icons/react";

import { DOCUMENTATION_URL, GITHUB_REPO_URL, SLACK_COMMUNITY_URL } from "@/constants";

export const AUTH_LAYOUT_INSET = 32;
export const AUTH_LAYOUT_CONTENT_WIDTH = 320;
export const AUTH_LAYOUT_ASIDE_BREAKPOINT = 1080;
export const AUTH_LAYOUT_ASIDE_CONTENT_WIDTH = 320;
export const AUTH_LAYOUT_ASIDE_GRID_SPACING = 12;
export const AUTH_LAYOUT_ASIDE_GRID_DOT_SIZE = 1;
export const AUTH_LAYOUT_ASIDE_GRID_OPACITY = 1;
export const AUTH_LAYOUT_ASIDE_LINK_PADDING = "12px 16px";

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
