import { styled } from "@linaria/react";
import { FunctionIcon, WarningIcon } from "@phosphor-icons/react";
import pluralize from "pluralize";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import type { PipelineCanvasValidationIssue } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasValidation";

const MAX_ISSUES_IN_TOOLTIP = 3;

const PipelineLayoutNavbarSaveIssueRow = withTheme(styled.div<
  PropsWithTheme<{ $isClickable: boolean }>
>`
  width: 100%;

  padding: 6px;

  display: flex;
  align-items: center;

  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 4px;

  background-color: ${({ theme }) => theme.color.background.primary};

  cursor: ${({ $isClickable }) => ($isClickable ? "pointer" : "default")};

  &:hover {
    background-color: ${({ theme, $isClickable }) =>
      $isClickable ? theme.color.background.secondary : theme.color.background.primary};
  }
`);

interface PipelineLayoutNavbarSaveIssuesProps {
  issues: PipelineCanvasValidationIssue[];
  onSelectResource: (edgeId: string) => void;
}

// Lists what blocks Save: one row per invalid resource, then any graph-level
// problem, capped with a "+N more" line.
const PipelineLayoutNavbarSaveIssues = ({
  issues,
  onSelectResource,
}: PipelineLayoutNavbarSaveIssuesProps) => {
  const hidden = issues.length - MAX_ISSUES_IN_TOOLTIP;
  return (
    <FlexWrapper
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.CENTER}
      gap={4}
      minWidth="240px"
      maxWidth="320px"
    >
      {issues.slice(0, MAX_ISSUES_IN_TOOLTIP).map(({ edgeId, ...issue }) => (
        <PipelineLayoutNavbarSaveIssueRow
          key={`${edgeId ?? ""}|${issue.resource ?? ""}|${issue.message}`}
          $isClickable={edgeId !== undefined}
          onClick={edgeId === undefined ? undefined : () => onSelectResource(edgeId)}
        >
          <FlexWrapper
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.SPACE_BETWEEN}
            gap={16}
            fillWidth
          >
            <FlexWrapper gap={8} alignItems={AlignItems.CENTER} overflow="hidden">
              <Icon
                component={issue.resource === undefined ? WarningIcon : FunctionIcon}
                size={16}
                variant={IconVariant.SECONDARY}
              />
              <FlexItem shrink={1} minWidth={0} overflow="hidden">
                <Text size={TextSize.BODY_MD} isEllipsis>
                  {issue.resource ?? issue.message}
                </Text>
              </FlexItem>
            </FlexWrapper>
            {issue.invalidSteps !== undefined && (
              <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM} variant={TextVariant.ERROR}>
                {issue.invalidSteps} {pluralize("issue", issue.invalidSteps)}
              </Text>
            )}
          </FlexWrapper>
        </PipelineLayoutNavbarSaveIssueRow>
      ))}
      {hidden > 0 && (
        <FlexWrapper padding="6px 0">
          <Text variant={TextVariant.TERTIARY} size={TextSize.BODY_SM}>
            +{hidden} more {pluralize("issue", hidden)}
          </Text>
        </FlexWrapper>
      )}
    </FlexWrapper>
  );
};

export default PipelineLayoutNavbarSaveIssues;
