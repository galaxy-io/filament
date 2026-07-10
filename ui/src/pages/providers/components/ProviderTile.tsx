import type { ComponentType } from "react";

import { styled } from "@linaria/react";
import { match } from "ts-pattern";

import GithubLogomark from "@galaxy-io/dls/icons/sources/GithubLogomark";
import GoogleBigqueryLogomark from "@galaxy-io/dls/icons/sources/GoogleBigqueryLogomark";
import HubspotLogomark from "@galaxy-io/dls/icons/sources/HubspotLogomark";
import JiraLogomark from "@galaxy-io/dls/icons/sources/JiraLogomark";
import LinearLogomark from "@galaxy-io/dls/icons/sources/LinearLogomark";
import MySQLLogomark from "@galaxy-io/dls/icons/sources/MySQLLogomark";
import NotionLogomark from "@galaxy-io/dls/icons/sources/NotionLogomark";
import PostgresLogomark from "@galaxy-io/dls/icons/sources/PostgresLogomark";
import S3Logomark from "@galaxy-io/dls/icons/sources/S3Logomark";
import SalesforceLogomark from "@galaxy-io/dls/icons/sources/SalesforceLogomark";
import SlackLogoIconmark from "@galaxy-io/dls/icons/sources/SlackLogoIconmark";
import SnowflakeLogomark from "@galaxy-io/dls/icons/sources/SnowflakeLogomark";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

export enum ProviderTileSize {
  SMALL = "SMALL",
  MEDIUM = "MEDIUM",
  LARGE = "LARGE",
}

const PROVIDER_TO_LOGOMARK_MAP: Record<string, ComponentType<{ height: number }>> = {
  bigquery: GoogleBigqueryLogomark,
  github: GithubLogomark,
  hubspot: HubspotLogomark,
  jira: JiraLogomark,
  linear: LinearLogomark,
  mysql: MySQLLogomark,
  notion: NotionLogomark,
  postgres: PostgresLogomark,
  s3: S3Logomark,
  object: S3Logomark,
  salesforce: SalesforceLogomark,
  slack: SlackLogoIconmark,
  snowflake: SnowflakeLogomark,
};

const getTileSize = (size: ProviderTileSize): number =>
  match(size)
    .with(ProviderTileSize.SMALL, () => 24)
    .with(ProviderTileSize.MEDIUM, () => 32)
    .with(ProviderTileSize.LARGE, () => 48)
    .exhaustive();

const getTileRadius = (size: ProviderTileSize): number =>
  match(size)
    .with(ProviderTileSize.SMALL, () => 4)
    .with(ProviderTileSize.MEDIUM, () => 5)
    .with(ProviderTileSize.LARGE, () => 8)
    .exhaustive();

const getLogoHeight = (size: ProviderTileSize): number =>
  match(size)
    .with(ProviderTileSize.SMALL, () => 16)
    .with(ProviderTileSize.MEDIUM, () => 20)
    .with(ProviderTileSize.LARGE, () => 32)
    .exhaustive();

const getTextSize = (size: ProviderTileSize): TextSize =>
  match(size)
    .with(ProviderTileSize.SMALL, () => TextSize.CAPTION)
    .with(ProviderTileSize.MEDIUM, () => TextSize.BODY_MD)
    .with(ProviderTileSize.LARGE, () => TextSize.BODY_LG)
    .exhaustive();

const TileWrapper = withTheme(styled.div<
  PropsWithTheme<{ $size: ProviderTileSize }>
>`
  width: ${({ $size }) => getTileSize($size)}px;
  height: ${({ $size }) => getTileSize($size)}px;

  display: flex;
  align-items: center;
  justify-content: center;

  background-color: ${({ theme }) => theme.color.background.secondary};

  border-radius: ${({ $size }) => getTileRadius($size)}px;

  overflow: hidden;
`);

interface ProviderTileProps {
  provider: string;
  size?: ProviderTileSize;
}

/**
 * A tile showing a provider's logomark, falling back to the provider's
 * first letter when no logomark exists in the DLS.
 */
const ProviderTile = ({
  provider,
  size = ProviderTileSize.MEDIUM,
}: ProviderTileProps) => {
  const Logomark = PROVIDER_TO_LOGOMARK_MAP[provider.toLowerCase()];

  return (
    <TileWrapper $size={size}>
      {Logomark ? (
        <Logomark height={getLogoHeight(size)} />
      ) : (
        <Text
          size={getTextSize(size)}
          variant={TextVariant.SECONDARY}
          isMonospace
        >
          {provider.charAt(0).toUpperCase()}
        </Text>
      )}
    </TileWrapper>
  );
};

export const ProviderOverflowTile = ({ count }: { count: number }) => {
  return (
    <TileWrapper $size={ProviderTileSize.SMALL}>
      <Text size={TextSize.CAPTION} variant={TextVariant.SECONDARY} isMonospace>
        +{count}
      </Text>
    </TileWrapper>
  );
};

export default ProviderTile;
