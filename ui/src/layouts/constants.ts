import { EmptyStateSize } from "@galaxy-io/dls/feedback/EmptyState";
import { TextSize } from "@galaxy-io/dls/text/Text";
import type { Space } from "@galaxy-io/dls/theme/enums";

import { LayoutSize } from "@/layouts/types";

export const LAYOUT_SIZE_TO_GAP_MAP: Record<LayoutSize, Space> = {
  [LayoutSize.SMALL]: 12,
  [LayoutSize.MEDIUM]: 16,
  [LayoutSize.LARGE]: 24,
};

export const LAYOUT_SIZE_TO_MESSAGE_SIZE_MAP: Record<LayoutSize, TextSize> = {
  [LayoutSize.SMALL]: TextSize.BODY_SM,
  [LayoutSize.MEDIUM]: TextSize.BODY_MD,
  [LayoutSize.LARGE]: TextSize.BODY_LG,
};

export const LAYOUT_SIZE_TO_EMPTY_STATE_SIZE_MAP: Record<LayoutSize, EmptyStateSize> = {
  [LayoutSize.SMALL]: EmptyStateSize.SMALL,
  [LayoutSize.MEDIUM]: EmptyStateSize.MEDIUM,
  [LayoutSize.LARGE]: EmptyStateSize.LARGE,
};
