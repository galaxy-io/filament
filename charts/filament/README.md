# Filament Helm Chart

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![AppVersion: 0.1.0](https://img.shields.io/badge/AppVersion-0.1.0-informational?style=flat-square)

A Helm chart for Filament

## Introduction

This chart deploys Filament server, control plane, Kubernetes worker dispatch support, and the configuration needed to connect Filament to PostgreSQL and NATS.

## Optional Requirements

| Repository | Name | Version |
|------------|------|---------|
| https://charts.bitnami.com/bitnami | postgresql | 18.7.11 |
| https://charts.zitadel.com | zitadel | 10.0.4 |
| https://nats-io.github.io/k8s/helm/charts | nats | 2.14.2 |

The vendored PostgreSQL, NATS, and Zitadel charts are disabled by default. See the [Bitnami PostgreSQL chart](https://artifacthub.io/packages/helm/bitnami/postgresql), [NATS chart](https://artifacthub.io/packages/helm/nats/nats), and [Zitadel chart](https://artifacthub.io/packages/helm/zitadel/zitadel) documentation for their full configuration surfaces.

## Runtime configuration

Filament requires a PostgreSQL DSN and a NATS URL. The default PostgreSQL-backed secret provider also requires a base64-encoded encryption key; AWS Secrets Manager uses its own credentials instead. Provide the values through `existingSecret` or through chart values so the chart can create the Secret.

To use an existing Secret:

```sh
kubectl create secret generic filament-runtime \
  --from-literal=PERSISTENCE_DSN='postgresql://USER:PASSWORD@HOST:5432/filament?sslmode=require' \
  --from-literal=ENCRYPTION_KEY="$(openssl rand -base64 32)" \
  --from-literal=NATS_URL='nats://nats.example.com:4222'

helm upgrade --install filament . \
  --set existingSecret=filament-runtime
```

For a local or test cluster with the vendored PostgreSQL and NATS charts:

```sh
PG_PASSWORD="$(openssl rand -hex 24)"
ENC_KEY="$(openssl rand -base64 32)"

helm upgrade --install filament . \
  --set postgresql.enabled=true \
  --set nats.enabled=true \
  --set-string postgresql.auth.password="$PG_PASSWORD" \
  --set-string persistence.postgresql.dsn="postgresql://filament:${PG_PASSWORD}@filament-postgresql:5432/filament?sslmode=disable" \
  --set-string secrets.postgres.encryptionKey="$ENC_KEY" \
  --set-string eventBus.nats.url='nats://filament-nats:4222'
```

## Overrides

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| fullnameOverride | string | `""` | String to fully override the generated release name. |
| nameOverride | string | `""` | Provide a name in place of `filament`. |
| namespaceOverride | string | `""` | Override the namespace used in rendered manifests. |

## Common parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| commonLabels | object | `{}` | Labels added to all Filament resources. |
| existingSecret | string | `""` | Name of an existing Secret containing `PERSISTENCE_DSN`, `NATS_URL`, `ENCRYPTION_KEY` when using the PostgreSQL-backed secret provider, and `AUTH_PAT` when auth is enabled. When set, the chart does not create its own Secret. |

## Server parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| server.autoscaling.enabled | bool | `false` | Enable a HorizontalPodAutoscaler for the server. |
| server.autoscaling.maxReplicas | int | `10` | Maximum server replicas when autoscaling is enabled. |
| server.autoscaling.minReplicas | int | `1` | Minimum server replicas when autoscaling is enabled. |
| server.autoscaling.targetCpu | int | `80` | Target average CPU utilization percentage for server autoscaling. |
| server.image.pullPolicy | string | `"IfNotPresent"` | Server image pull policy. |
| server.image.pullSecrets | list | `[]` | Image pull secrets for the server Deployment. |
| server.image.repository | string | `"ghcr.io/galaxy-io/filament/server"` | Server image repository. |
| server.image.tag | string | `""` (defaults to chart appVersion) | Server image tag. |
| server.ingress.annotations | object | `{}` | Annotations for the Ingress, e.g. a cert-manager issuer or ALB settings. |
| server.ingress.className | string | `""` | IngressClass name, e.g. `nginx` or `alb`. Empty uses the cluster default. |
| server.ingress.enabled | bool | `false` | Enable an Ingress for the server API and UI. |
| server.ingress.hosts | list | `["filament.example.com"]` | Hostnames served by the Ingress. |
| server.ingress.path | string | `"/"` | Path served by the Ingress. |
| server.ingress.pathType | string | `"Prefix"` | PathType for the path. |
| server.ingress.tls | list | `[]` | Ingress TLS configuration, passed through verbatim. |
| server.replicas | int | `1` | Number of server replicas. Ignored when `server.autoscaling.enabled` is true. |
| server.resources | object | `{}` (See [values.yaml]) | Server resource requests and limits. |
| server.service.port | int | `8080` | Server service and container port. |
| server.serviceAccount.annotations | object | `{}` | Annotations for the chart-created server ServiceAccount, e.g. an IRSA role ARN. |
| server.serviceAccount.name | string | `""` | Existing ServiceAccount name for the server. When set, the chart does not create one. |

## Control plane parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| controlPlane.autoscaling.enabled | bool | `false` | Enable a HorizontalPodAutoscaler for the control plane. |
| controlPlane.autoscaling.maxReplicas | int | `10` | Maximum control plane replicas when autoscaling is enabled. |
| controlPlane.autoscaling.minReplicas | int | `1` | Minimum control plane replicas when autoscaling is enabled. |
| controlPlane.autoscaling.targetCpu | int | `80` | Target average CPU utilization percentage for control plane autoscaling. |
| controlPlane.dispatch.job.backoffLimit | int | `1` | Kubernetes Job backoff limit for dispatched workers. |
| controlPlane.dispatch.job.namePrefix | string | `"filament"` | Prefix used when naming dispatched worker Jobs. |
| controlPlane.dispatch.job.ttlSecondsAfterFinished | int | `3600` | Seconds to retain completed dispatched worker Jobs. |
| controlPlane.dispatch.mode | string | `"kubernetes"` | Worker dispatch backend. |
| controlPlane.dispatch.worker.activeDeadlineSeconds | string | `""` | Worker Job active deadline in seconds. Leave empty for no deadline. |
| controlPlane.dispatch.worker.heartbeatSeconds | int | `30` | Interval in seconds between worker heartbeats, stored in the worker ConfigMap as `HEARTBEAT_SECONDS`. Empty uses the worker's own default. |
| controlPlane.dispatch.worker.image.pullPolicy | string | `"IfNotPresent"` | Worker image pull policy. |
| controlPlane.dispatch.worker.image.pullSecrets | list | `[]` | Image pull secrets for dispatched worker Jobs. |
| controlPlane.dispatch.worker.image.repository | string | `"ghcr.io/galaxy-io/filament/worker"` | Worker image repository used for dispatched Jobs. |
| controlPlane.dispatch.worker.image.tag | string | `""` (defaults to chart appVersion) | Worker image tag. |
| controlPlane.dispatch.worker.restartPolicy | string | `"Never"` | Restart policy for dispatched worker Jobs. |
| controlPlane.dispatch.worker.serviceAccount.annotations | object | `{}` | Annotations for the chart-created worker ServiceAccount, e.g. an IRSA role ARN. |
| controlPlane.dispatch.worker.serviceAccount.name | string | `""` | Existing ServiceAccount name for dispatched worker Jobs. When set, the chart does not create one. |
| controlPlane.dispatch.worker.terminationGraceSeconds | int | `30` | Worker Job termination grace period in seconds. |
| controlPlane.enabled | bool | `true` | Deploy the Filament control plane. |
| controlPlane.health.port | int | `8081` | Port the control plane serves `/livez` and `/readyz` on, stored in the ConfigMap as `HEALTH_ADDR` and used for the container port and probes. |
| controlPlane.image.pullPolicy | string | `"IfNotPresent"` | Control plane image pull policy. |
| controlPlane.image.pullSecrets | list | `[]` | Image pull secrets for the control plane Deployment. |
| controlPlane.image.repository | string | `"ghcr.io/galaxy-io/filament/control-plane"` | Control plane image repository. |
| controlPlane.image.tag | string | `""` (defaults to chart appVersion) | Control plane image tag. |
| controlPlane.replicas | int | `1` | Number of control plane replicas. Ignored when `controlPlane.autoscaling.enabled` is true. |
| controlPlane.resources | object | `{}` (See [values.yaml]) | Control plane resource requests and limits. |
| controlPlane.serviceAccount.annotations | object | `{}` | Annotations for the chart-created control plane ServiceAccount, e.g. an IRSA role ARN. |
| controlPlane.serviceAccount.name | string | `""` | Existing ServiceAccount name for the control plane. When set, the chart does not create one. |

## Metrics parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| metrics.type | string | `"postgresql"` | Metrics backend for the dashboard query API, stored in the server ConfigMap as `METRICSSTORE_PROVIDER`. `postgresql` answers from the datastore's own pool. |

## Persistence parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| persistence.migrate | bool | `true` | Run database migrations as a post-install/post-upgrade hook Job. |
| persistence.postgresql.dsn | string | required | PostgreSQL connection string stored in the chart-created Secret as `PERSISTENCE_DSN`. Required unless `existingSecret` is set. |
| persistence.type | string | `"postgresql"` | Persistence provider. |

## Secret provider parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| secrets.aws.region | string | `""` | AWS region for Secrets Manager, stored in the ConfigMap as `AWS_REGION`. Leave empty to use the SDK default chain (env, IMDS). |
| secrets.postgres.encryptionKey | string | required | Base64-encoded AES key stored in the chart-created Secret as `ENCRYPTION_KEY`. Required unless `existingSecret` is set. |
| secrets.postgres.encryptionKeyId | string | `""` | Encryption key identifier stored with each secret row. Change this when rotating keys. |
| secrets.prefix | string | `""` | Extra name prefix external secret stores apply to every secret reference, stored in the ConfigMap as `SECRETS_PREFIX`. Filament-minted references are already namespaced under `filament/`. Unused by postgres. |
| secrets.type | string | `"postgres"` | Secret storage provider. Valid values are `postgres` and `aws-secrets-manager`. |

## Event bus parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| eventBus.nats.stream | string | `"EVENTBUS"` | NATS stream used by Filament. |
| eventBus.nats.subjects | string | `"ingestion.v1.>"` | NATS subject filter consumed by Filament. |
| eventBus.nats.ttlSeconds | int | `604800` | Maximum age of messages retained in the NATS stream, in seconds. Set to 0 to disable age-based expiration. |
| eventBus.nats.url | string | required | NATS connection URL stored in the chart-created Secret as `NATS_URL`. Required unless `existingSecret` is set. |
| eventBus.type | string | `"nats"` | Event bus provider. |

## Authentication parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| auth.enabled | bool | `false` | Enable authentication. Disabled leaves the API unauthenticated and every request scoped to the default tenant. |
| auth.type | string | `"zitadel"` | Identity provider. Valid value is `zitadel`. |
| auth.uiOrigin | string | `""` | Browser origin sign-in returns to, stored in the ConfigMap as `AUTH_UI_ORIGIN`. This is where Filament serves its own login page, so it is normally the ingress host. |
| auth.zitadel.issuer | string | required when `auth.enabled=true` | Issuer URL Filament and the browser reach the provider at, stored in the ConfigMap as `AUTH_ISSUER`. Both resolve it, so it must be one name that answers from inside the cluster and from the browser, and it must equal the issuer Zitadel advertises or discovery fails. |
| auth.zitadel.pat | string | required when `auth.enabled=true` and `zitadel.enabled=false` | Machine-user personal access token, stored in the chart-created Secret as `AUTH_PAT`. Zitadel generates the token itself and will not accept one you choose, so create the machine user out of band and paste the result here. Ignored when `zitadel.enabled=true`: the vendored setup job mints a token into its own Secret and the server reads it from there. |

## Observability parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| observability.otel.endpoint | string | `""` | OTLP endpoint metrics and traces are exported to, stored in the ConfigMaps as `OTEL_EXPORTER_OTLP_ENDPOINT`. Use an `http://` scheme for plaintext in-cluster collectors. Empty disables export. |
| observability.otel.protocol | string | `""` | OTLP transport, stored in the ConfigMaps as `OTEL_EXPORTER_OTLP_PROTOCOL`. Valid values are `grpc` (collector port 4317) and `http/protobuf` (port 4318). Empty defaults to `grpc`. |

## Vendored PostgreSQL parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| postgresql.auth.database | string | `"filament"` | Database created by the vendored PostgreSQL chart. |
| postgresql.auth.enablePostgresUser | bool | `false` | Disable the default `postgres` superuser in the vendored PostgreSQL chart. |
| postgresql.auth.password | string | required when `postgresql.enabled=true` | Password for the vendored PostgreSQL user. Must match `persistence.postgresql.dsn` when vendored PostgreSQL is enabled. |
| postgresql.auth.username | string | `"filament"` | Username created by the vendored PostgreSQL chart. |
| postgresql.backup.enabled | bool | `false` | Enable daily logical dumps (`pg_dumpall`) of the vendored PostgreSQL to a dedicated PVC. See `backup.cronjob.*` in the upstream chart for schedule and storage options. |
| postgresql.enabled | bool | `false` | Enable the vendored Bitnami PostgreSQL chart for local or test clusters. See the [Bitnami PostgreSQL chart](https://artifacthub.io/packages/helm/bitnami/postgresql) for additional configuration. |
| postgresql.fullnameOverride | string | `"filament-postgresql"` | Full name override for the vendored PostgreSQL release. |
| postgresql.primary.persistence.size | string | `"8Gi"` | PVC size for the vendored PostgreSQL primary. |
| postgresql.primary.resources | object | `{"limits":{"memory":"1Gi"},"requests":{"cpu":"250m","memory":"512Mi"}}` | Resources for the vendored PostgreSQL primary. Overrides the upstream `nano` preset (192Mi memory limit), which risks OOM kills and unclean shutdowns under real load. |

## Vendored NATS parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| nats.config.jetstream.enabled | bool | `true` | Enable JetStream in the vendored NATS chart. |
| nats.config.jetstream.fileStore.pvc.size | string | `"10Gi"` | PVC size for the vendored NATS JetStream file store. |
| nats.container.merge | object | `{"resources":{"limits":{"memory":"512Mi"},"requests":{"cpu":"100m","memory":"256Mi"}}}` | Merged into the vendored NATS container spec. Sets resources so the pod is not BestEffort QoS (first evicted under node pressure); the upstream chart sets none. |
| nats.enabled | bool | `false` | Enable the vendored NATS chart for local or test clusters. See the [NATS chart](https://artifacthub.io/packages/helm/nats/nats) for additional configuration. |
| nats.fullnameOverride | string | `"filament-nats"` | Full name override for the vendored NATS release. |

## Vendored Zitadel parameters

The vendored Zitadel runs its init and setup jobs as post-install hooks so the vendored PostgreSQL exists before they poll it, and the server starts once setup has minted its access token. Do not install with `--wait`, which holds those hooks until pods that depend on them are ready.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| zitadel.enabled | bool | `false` | Enable the vendored Zitadel chart for local or test clusters. See the [Zitadel chart](https://artifacthub.io/packages/helm/zitadel/zitadel) for additional configuration. |
| zitadel.fullnameOverride | string | `"filament-zitadel"` | Full name override for the vendored Zitadel release. |
| zitadel.initJob.annotations | object | `{"helm.sh/hook":"post-install,post-upgrade","helm.sh/hook-delete-policy":"before-hook-creation","helm.sh/hook-weight":"1"}` | Hook timing for the Zitadel init job, moved to post-install so the vendored PostgreSQL exists before the job polls it. |
| zitadel.login.enabled | bool | `false` | Deploy Zitadel's own login UI. Filament serves its own sign-in pages and points the OIDC client at them, so the upstream login Deployment, Service, ConfigMap, ServiceAccount, and generated keypair are unused. |
| zitadel.resources.limits.memory | string | `"1Gi"` |  |
| zitadel.resources.requests | object | `{"cpu":"100m","memory":"512Mi"}` | Resources for the vendored Zitadel pod. The upstream chart sets none, which leaves it BestEffort QoS and first to be evicted. |
| zitadel.setupJob.annotations | object | `{"helm.sh/hook":"post-install,post-upgrade","helm.sh/hook-delete-policy":"before-hook-creation","helm.sh/hook-weight":"2"}` | Hook timing for the Zitadel setup job, weighted to run after init. |
| zitadel.zitadel.configmapConfig.Database.Postgres.Admin.SSL.Mode | string | `"disable"` |  |
| zitadel.zitadel.configmapConfig.Database.Postgres.Database | string | `"zitadel"` | Database Zitadel creates and owns. Keep it separate from Filament's own database even when they share a server. |
| zitadel.zitadel.configmapConfig.Database.Postgres.Host | string | required when `zitadel.enabled=true` | PostgreSQL server Zitadel connects to, e.g. the vendored `filament-postgresql` Service. The init job also needs `Admin.Username` and `Admin.Password` for a role that can create databases, and the runtime needs `User.Password`. |
| zitadel.zitadel.configmapConfig.Database.Postgres.User.SSL.Mode | string | `"disable"` |  |
| zitadel.zitadel.configmapConfig.Database.Postgres.User.Username | string | `"zitadel"` |  |
| zitadel.zitadel.configmapConfig.ExternalDomain | string | required when `zitadel.enabled=true` | Hostname Zitadel advertises itself at. |
| zitadel.zitadel.configmapConfig.ExternalSecure | bool | `true` | Serve external traffic over HTTPS. Zitadel builds its OIDC issuer from `ExternalSecure`, `ExternalDomain`, and `ExternalPort`, and that issuer must match `auth.zitadel.issuer` exactly. |
| zitadel.zitadel.masterkey | string | required when `zitadel.enabled=true` | 32-character key Zitadel encrypts its stored credentials with. |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs](https://github.com/norwoodj/helm-docs)
