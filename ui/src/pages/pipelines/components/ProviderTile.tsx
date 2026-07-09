import type { ComponentType } from "react";

import { styled } from "@linaria/react";

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
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

const TILE_SIZE = 20;
const LOGO_HEIGHT = 12;

const PROVIDER_LOGOMARKS: Record<string, ComponentType<{ height: number }>> = {
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

const TileWrapper = withTheme(styled.div<PropsWithTheme>`
  width: ${TILE_SIZE}px;
  height: ${TILE_SIZE}px;

  display: flex;
  align-items: center;
  justify-content: center;

  background-color: ${({ theme }) => theme.color.background.secondary};

  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 3.5px;

  overflow: hidden;
`);

interface ProviderTileProps {
  provider: string;
}

/**
 * A 20px tile showing a provider's logomark, falling back to the provider's
 * first letter when no logomark exists in the DLS.
 */
const ProviderTile = ({ provider }: ProviderTileProps) => {
  const Logomark = PROVIDER_LOGOMARKS[provider.toLowerCase()];

  return (
    <Tooltip body={provider}>
      <TileWrapper>
        {Logomark ? (
          <Logomark height={LOGO_HEIGHT} />
        ) : (
          <Text size={TextSize.CAPTION} variant={TextVariant.SECONDARY} isMonospace>
            {provider.charAt(0).toUpperCase()}
          </Text>
        )}
      </TileWrapper>
    </Tooltip>
  );
};

export const ProviderOverflowTile = ({ count }: { count: number }) => {
  return (
    <TileWrapper>
      <Text size={TextSize.CAPTION} variant={TextVariant.SECONDARY} isMonospace>
        +{count}
      </Text>
    </TileWrapper>
  );
};

export default ProviderTile;
