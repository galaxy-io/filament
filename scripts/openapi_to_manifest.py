#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = ["pyyaml"]
# ///
"""Convert an OpenAPI 3.x spec into a draft filament HTTP connector manifest.

Usage:
    uv run scripts/openapi_to_manifest.py <path-or-url> [--name NAME] [--out PATH]
        [--include GLOB]... [--exclude GLOB]... [--no-validate]

The input may be a local file or an http(s) URL, JSON or YAML. Every list-style
GET endpoint becomes a resource; auth, pagination, and field types are inferred
heuristically. The result is written to connectors/http/manifests/<name>.yaml by
default and validated with `go test ./connectors/http -run TestManifests`.
Review the TODO comments and warnings before wiring the connector into
catalog.go/register.go (see .claude/skills/http-connector).
"""

from __future__ import annotations

import argparse
import fnmatch
import json
import re
import subprocess
import sys
import urllib.request
from collections import Counter
from pathlib import Path

import yaml

REPO_ROOT = Path(__file__).resolve().parent.parent
MANIFESTS_DIR = REPO_ROOT / "connectors" / "http" / "manifests"

RECORDS_CONTAINER_KEYS = ("data", "items", "results", "records", "entries", "elements", "value")
CURSOR_REQUEST_PARAMS = ("cursor", "next_cursor", "starting_after", "page_token", "start", "after", "next")
CURSOR_RESPONSE_KEYS = ("next_cursor", "cursor", "next_page_token", "next_token", "starting_after", "after", "next")
NEXT_URL_KEYS = ("next", "next_url", "next_page", "next_uri", "next_link", "next_page_url")
PAGINATION_CONTAINERS = ("page", "paging", "pagination", "meta", "links", "response_metadata")
PAGE_SIZE_PARAMS = ("page_size", "per_page", "pageSize", "perPage", "size", "count", "limit")
NAME_DROP_TOKENS = {"api", "apis", "developer", "rest", "openapi", "spec", "platform", "public"}
STRIP_PATH_PREFIX = re.compile(r"^(api|developer|rest|v\d+(\.\d+)*)$", re.IGNORECASE)

WARNINGS: list[str] = []


def warn(msg: str) -> None:
    WARNINGS.append(msg)
    print(f"warning: {msg}", file=sys.stderr)


def fail(msg: str) -> None:
    print(f"error: {msg}", file=sys.stderr)
    sys.exit(1)


# ---------------------------------------------------------------------------
# Spec loading and $ref resolution


def load_spec(src: str) -> dict:
    if re.match(r"^https?://", src):
        req = urllib.request.Request(
            src,
            headers={
                "User-Agent": "filament-openapi-to-manifest/1.0",
                "Accept": "application/json, application/yaml, text/yaml, */*",
            },
        )
        with urllib.request.urlopen(req, timeout=60) as resp:
            raw = resp.read().decode("utf-8", errors="replace")
    else:
        path = Path(src)
        if not path.exists():
            fail(f"spec file not found: {src}")
        raw = path.read_text()
    try:
        spec = json.loads(raw)
    except json.JSONDecodeError:
        try:
            spec = yaml.safe_load(raw)
        except yaml.YAMLError as e:
            fail(f"input is neither valid JSON nor valid YAML: {e}")
    if not isinstance(spec, dict):
        fail("spec did not parse to an object")
    if "swagger" in spec:
        fail(f"Swagger {spec['swagger']} specs are not supported; convert to OpenAPI 3.x first")
    if not str(spec.get("openapi", "")).startswith("3"):
        fail("not an OpenAPI 3.x document (missing/unsupported 'openapi' version)")
    return spec


class Resolver:
    def __init__(self, spec: dict):
        self.spec = spec

    def deref(self, schema, _seen: frozenset = frozenset()) -> dict:
        while isinstance(schema, dict) and "$ref" in schema:
            ref = schema["$ref"]
            if not isinstance(ref, str) or not ref.startswith("#/"):
                warn(f"skipping non-local $ref {ref!r}")
                return {}
            if ref in _seen:
                return {"type": "object"}
            _seen = _seen | {ref}
            node = self.spec
            for part in ref[2:].split("/"):
                part = part.replace("~1", "/").replace("~0", "~")
                if not isinstance(node, dict) or part not in node:
                    warn(f"unresolvable $ref {ref!r}")
                    return {}
                node = node[part]
            schema = node
        if not isinstance(schema, dict):
            return {}
        if "allOf" in schema:
            merged: dict = {"type": "object", "properties": {}, "required": []}
            parts = list(schema["allOf"]) + [{k: v for k, v in schema.items() if k != "allOf"}]
            for sub in parts:
                sub = self.deref(sub, _seen)
                merged["properties"].update(sub.get("properties", {}))
                merged["required"] += [r for r in sub.get("required", []) if r not in merged["required"]]
                if sub.get("nullable"):
                    merged["nullable"] = True
            return merged
        return schema

    def type_of(self, schema) -> tuple[str, bool]:
        """Map a schema to (manifest field type, nullable)."""
        s = self.deref(schema)
        nullable = bool(s.get("nullable"))
        t = s.get("type")
        if isinstance(t, list):
            nullable = nullable or "null" in t
            non_null = [x for x in t if x != "null"]
            t = non_null[0] if len(non_null) == 1 else None
        if t is None:
            subs = s.get("oneOf") or s.get("anyOf") or []
            resolved = [self.deref(x) for x in subs]
            non_null = [r for r in resolved if r.get("type") != "null"]
            if len(non_null) < len(resolved):
                nullable = True
            if len(non_null) == 1:
                inner_t, inner_n = self.type_of(non_null[0])
                return inner_t, nullable or inner_n
            return "json", nullable
        if t == "integer":
            return "int64", nullable
        if t == "number":
            return "float64", nullable
        if t == "boolean":
            return "bool", nullable
        if t == "string":
            fmt = s.get("format", "")
            if fmt == "date-time":
                return "timestamptz", nullable
            if fmt == "date":
                return "date", nullable
            if fmt == "uuid":
                return "uuid", nullable
            return "string", nullable
        return "json", nullable


# ---------------------------------------------------------------------------
# YAML emitter (house style: block maps, flow field specs, TODO comments)


class Flow(dict):
    pass


class CMap(dict):
    def __init__(self, *a, **k):
        super().__init__(*a, **k)
        self.key_comments: dict[str, str] = {}


BOOLISH = {"true", "false", "yes", "no", "on", "off", "null", "none", "~", "y", "n"}


def needs_quote(s: str, flow: bool) -> bool:
    if s == "" or s != s.strip():
        return True
    if s.lower() in BOOLISH:
        return True
    if s[0] in "!&*?{}[]#|>@`\"'%,":
        return True
    if s[0] == "-" and (len(s) == 1 or s[1] in " \t"):
        return True
    if s[0].isdigit() or (s[0] in "+-." and len(s) > 1 and s[1].isdigit()):
        return True
    if ": " in s or s.endswith(":") or " #" in s:
        return True
    if "{{" in s or "\n" in s or "\t" in s:
        return True
    if flow and any(c in s for c in ",[]{}:"):
        return True
    return False


def fmt_scalar(v, flow: bool = False) -> str:
    if isinstance(v, bool):
        return "true" if v else "false"
    if isinstance(v, (int, float)):
        return str(v)
    if v is None:
        return "null"
    s = str(v)
    if not needs_quote(s, flow):
        return s
    return '"' + s.replace("\\", "\\\\").replace('"', '\\"') + '"'


def fmt_flow(d: dict) -> str:
    return "{ " + ", ".join(f"{fmt_scalar(k, True)}: {fmt_scalar(v, True)}" for k, v in d.items()) + " }"


def emit_map(d: dict, indent: int, out: list[str]) -> None:
    pad = " " * indent
    comments = getattr(d, "key_comments", {})
    for k, v in d.items():
        if k in comments:
            out.append(f"{pad}# {comments[k]}")
        key = fmt_scalar(k)
        if isinstance(v, Flow):
            out.append(f"{pad}{key}: {fmt_flow(v)}")
        elif isinstance(v, dict):
            if v:
                out.append(f"{pad}{key}:")
                emit_map(v, indent + 2, out)
            else:
                out.append(f"{pad}{key}: {{}}")
        elif isinstance(v, list):
            if not v:
                out.append(f"{pad}{key}: []")
            elif all(not isinstance(i, (dict, list)) for i in v):
                out.append(f"{pad}{key}: [" + ", ".join(fmt_scalar(i, True) for i in v) + "]")
            else:
                out.append(f"{pad}{key}:")
                emit_list(v, indent + 2, out)
        else:
            out.append(f"{pad}{key}: {fmt_scalar(v)}")


def emit_list(items: list, indent: int, out: list[str]) -> None:
    pad = " " * indent
    for item in items:
        if isinstance(item, dict):
            sub: list[str] = []
            emit_map(item, indent + 2, sub)
            i = 0
            while i < len(sub) and sub[i].lstrip().startswith("#"):
                out.append(pad + sub[i].lstrip())
                i += 1
            out.append(pad + "- " + sub[i][indent + 2 :])
            out.extend(sub[i + 1 :])
        else:
            out.append(f"{pad}- {fmt_scalar(item)}")


def dump_manifest(manifest: dict) -> str:
    out: list[str] = []
    emit_map(manifest, 0, out)
    return "\n".join(out) + "\n"


# ---------------------------------------------------------------------------
# Conversion


def slug(s: str) -> str:
    return re.sub(r"[^a-z0-9]+", "_", s.lower()).strip("_")


def default_name(spec: dict) -> str:
    tokens = slug(spec.get("info", {}).get("title", "connector")).split("_")
    while len(tokens) > 1 and tokens[-1] in NAME_DROP_TOKENS:
        tokens.pop()
    return "_".join(tokens) or "connector"


def one_line(s: str, limit: int = 120) -> str:
    s = re.sub(r"\s+", " ", s or "").strip()
    return s[: limit - 1] + "…" if len(s) > limit else s


def build_auth(spec: dict) -> tuple[dict | None, dict, dict]:
    """Returns (auth block, config entries, extra default query params)."""
    schemes = (spec.get("components") or {}).get("securitySchemes") or {}
    config: dict = {}
    if not schemes:
        warn("no securitySchemes found; manifest has no auth — add one by hand if needed")
        return None, config, {}

    order: list[str] = []
    for req in spec.get("security") or []:
        order += [n for n in req if n not in order]
    order += [n for n in schemes if n not in order]
    chosen = next((n for n in order if n in schemes), None)
    scheme = schemes[chosen]
    ignored = [n for n in schemes if n != chosen]
    if ignored:
        warn(f"multiple security schemes; using {chosen!r}, ignoring {ignored}")

    stype = scheme.get("type")
    help_text = one_line(scheme.get("description", ""))

    if stype == "http" and scheme.get("scheme") == "bearer":
        config["api_token"] = {"type": "secret", "required": True, "help": help_text or "API bearer token"}
        return {"bearer": "config.api_token"}, config, {}

    if stype == "http" and scheme.get("scheme") == "basic":
        config["username"] = {"type": "string", "required": True, "help": "Basic auth username"}
        config["password"] = {"type": "secret", "required": True, "help": "Basic auth password"}
        return {"basic": {"username": "config.username", "password": "config.password"}}, config, {}

    if stype == "apiKey":
        config["api_key"] = {"type": "secret", "required": True, "help": help_text or "API key"}
        if scheme.get("in") == "header":
            return {"header": {"name": scheme.get("name", "X-Api-Key"), "value": "config.api_key"}}, config, {}
        if scheme.get("in") == "query":
            warn("apiKey-in-query auth mapped to a default query param; verify it")
            return None, config, {scheme.get("name", "api_key"): "{{ config.api_key }}"}
        warn(f"apiKey in {scheme.get('in')!r} is unsupported; falling back to no auth")
        return None, {}, {}

    if stype == "oauth2":
        cc = (scheme.get("flows") or {}).get("clientCredentials")
        if cc and cc.get("tokenUrl"):
            config["client_id"] = {"type": "string", "required": True, "help": "OAuth2 client ID"}
            config["client_secret"] = {"type": "secret", "required": True, "help": "OAuth2 client secret"}
            auth = CMap(
                oauth2=CMap(
                    token_url=cc["tokenUrl"],
                    client_id="config.client_id",
                    client_secret="config.client_secret",
                )
            )
            auth["oauth2"].key_comments["token_url"] = (
                "TODO: add scope if the token endpoint requires explicit scopes, "
                'e.g. scope: "transactions:read users:read"'
            )
            warn("oauth2 client-credentials detected; set the required scopes on connection.auth.oauth2")
            return auth, config, {}
        warn("oauth2 without a client_credentials flow is unsupported; falling back to a bearer token config")
        config["api_token"] = {"type": "secret", "required": True, "help": "OAuth2 access token (obtain out of band)"}
        return {"bearer": "config.api_token"}, config, {}

    warn(f"unsupported security scheme type {stype!r}; manifest has no auth")
    return None, {}, {}


def base_url(spec: dict) -> str:
    servers = spec.get("servers") or []
    if not servers:
        warn("no servers defined; set connection.base_url by hand")
        return "https://TODO.example.com"
    url = servers[0].get("url", "")
    for var, defn in (servers[0].get("variables") or {}).items():
        if "default" in defn:
            url = url.replace("{" + var + "}", str(defn["default"]))
    if "{" in url:
        warn(f"server URL still contains template variables: {url}")
    if len(servers) > 1:
        warn(f"multiple servers; using {url!r}")
    return url


def success_json_schema(op: dict) -> dict | None:
    responses = op.get("responses") or {}
    for code in ("200", "201", "2XX", "default"):
        resp = responses.get(code)
        if not isinstance(resp, dict):
            continue
        content = resp.get("content") or {}
        for ctype, media in content.items():
            if "json" in ctype and isinstance(media, dict) and media.get("schema"):
                return media["schema"]
    return None


def find_records(schema: dict, resolver: Resolver):
    """Returns (records path, item schema, envelope schema, container key) or None."""
    if schema.get("type") == "array":
        return "$", schema.get("items", {}), None, None
    props = schema.get("properties")
    if not isinstance(props, dict):
        return None
    for key in RECORDS_CONTAINER_KEYS:
        if key not in props:
            continue
        p = resolver.deref(props[key])
        if p.get("type") == "array":
            return f"$.{key}", p.get("items", {}), schema, key
        if p.get("type") == "object":
            inner = [(n, resolver.deref(ip)) for n, ip in (p.get("properties") or {}).items()]
            arrays = [(n, ip) for n, ip in inner if ip.get("type") == "array"]
            if len(arrays) == 1:
                n, ip = arrays[0]
                return f"$.{key}.{n}", ip.get("items", {}), schema, key
    arrays = [(n, resolver.deref(p)) for n, p in props.items() if resolver.deref(p).get("type") == "array"]
    if len(arrays) == 1:
        n, p = arrays[0]
        pagination_ish = (
            set(PAGINATION_CONTAINERS)
            | set(NEXT_URL_KEYS)
            | set(CURSOR_RESPONSE_KEYS)
            | {"total", "count", "total_count", "total_pages", "has_more", "object", "url", "limit", "offset"}
        )
        if all(o in pagination_ish for o in set(props) - {n}):
            return f"$.{n}", p.get("items", {}), schema, n
    return None


def looks_like_url(name: str, schema: dict) -> bool:
    if schema.get("format") in ("uri", "url"):
        return True
    if "url" in name or "link" in name or "uri" in name:
        return True
    example = schema.get("example")
    if isinstance(example, str) and example.startswith("http"):
        return True
    return bool(re.search(r"\burl\b|https?://", (schema.get("description") or ""), re.IGNORECASE))


def find_next_field(envelope: dict, resolver: Resolver, records_key: str | None):
    candidates: list[tuple[str, str, dict]] = []
    for n, p in (envelope.get("properties") or {}).items():
        if n == records_key:
            continue
        s = resolver.deref(p)
        if s.get("type") == "object" and n in PAGINATION_CONTAINERS:
            for n2, p2 in (s.get("properties") or {}).items():
                candidates.append((f"{n}.{n2}", n2, resolver.deref(p2)))
        else:
            candidates.append((n, n, s))
    stringish = [(path, n, s) for path, n, s in candidates if s.get("type") in (None, "string")]
    for path, n, s in stringish:
        if n in NEXT_URL_KEYS and looks_like_url(n, s):
            return path, "url"
    for path, n, s in stringish:
        if n in CURSOR_RESPONSE_KEYS:
            return path, "cursor"
    return None


def detect_pagination(op: dict, query_names: set[str], envelope: dict | None, records_key: str | None, resolver: Resolver):
    """Returns (pagination value, set of query params it consumes)."""
    for resp in (op.get("responses") or {}).values():
        headers = (resp or {}).get("headers") if isinstance(resp, dict) else None
        if headers and any(h.lower() == "link" for h in headers):
            return {"link": "next"}, set()
    if envelope:
        found = find_next_field(envelope, resolver, records_key)
        if found:
            path, kind = found
            if kind == "url":
                return {"next_url": f"$.{path}"}, set()
            req = next((c for c in CURSOR_REQUEST_PARAMS if c in query_names), None)
            if req:
                return {"cursor": {"response": path, "request": f"query.{req}"}}, {req}
    if "page" in query_names:
        size = next((s for s in ("per_page", "page_size", "pageSize", "size") if s in query_names), None)
        if size:
            return {"page": {"number": "query.page", "size": f"query.{size}", "page_size": 100}}, {"page", size}
    if "offset" in query_names and "limit" in query_names:
        return {"offset": {"offset": "query.offset", "limit": "query.limit", "page_size": 100}}, {"offset", "limit"}
    return None, set()


def collect_params(path_item: dict, op: dict, resolver: Resolver) -> list[dict]:
    merged: dict[tuple[str, str], dict] = {}
    for p in (path_item.get("parameters") or []) + (op.get("parameters") or []):
        p = resolver.deref(p)
        if p.get("name"):
            merged[(p.get("in", ""), p["name"])] = p
    return list(merged.values())


def build_fields(item_schema: dict, resolver: Resolver) -> CMap:
    fields = CMap()
    required = set(item_schema.get("required") or [])
    for name, prop in (item_schema.get("properties") or {}).items():
        if name == "raw":
            continue
        if not re.fullmatch(r"[A-Za-z0-9_\-]+", name):
            continue
        ftype, nullable = resolver.type_of(prop)
        nullable = nullable or name not in required
        fields[name] = ftype + ("?" if nullable else "")
    fields["raw"] = Flow(path="$", type="json", mode="remainder")
    return fields


def pick_primary_key(fields: dict) -> list[str] | None:
    if "id" in fields:
        return ["id"]
    for name, spec in fields.items():
        if name == "raw":
            continue
        base = spec.rstrip("?") if isinstance(spec, str) else ""
        if name.endswith("_id") or base == "uuid":
            return [name]
    return None


def resource_names(entries: list[dict]) -> None:
    for e in entries:
        segs = [slug(s) for s in e["path"].split("/") if s and not s.startswith("{")]
        while segs and STRIP_PATH_PREFIX.match(segs[0]):
            segs.pop(0)
        e["segs"] = segs or ["root"]
        e["depth"] = 1
    for _ in range(10):
        counts = Counter("_".join(e["segs"][-e["depth"] :]) for e in entries)
        changed = False
        for e in entries:
            name = "_".join(e["segs"][-e["depth"] :])
            if counts[name] > 1 and e["depth"] < len(e["segs"]):
                e["depth"] += 1
                changed = True
        if not changed:
            break
    seen: dict[str, int] = {}
    for e in entries:
        name = "_".join(e["segs"][-e["depth"] :])
        if name in seen:
            seen[name] += 1
            name = f"{name}_{seen[name]}"
        else:
            seen[name] = 1
        e["name"] = name


def matches(patterns: list[str], name: str, path: str) -> bool:
    return any(fnmatch.fnmatch(name, p) or fnmatch.fnmatch(path, p) for p in patterns)


def convert(spec: dict, name: str, include: list[str], exclude: list[str]) -> str:
    resolver = Resolver(spec)
    auth, config, extra_query = build_auth(spec)

    entries: list[dict] = []
    for path, path_item in (spec.get("paths") or {}).items():
        if not isinstance(path_item, dict):
            continue
        op = path_item.get("get")
        if not isinstance(op, dict):
            continue
        if path.rstrip("/").split("/")[-1].startswith("{"):
            continue
        resp_schema = success_json_schema(op)
        if resp_schema is None:
            continue
        schema = resolver.deref(resp_schema)
        found = find_records(schema, resolver)
        if not found:
            continue
        records, item_schema, envelope, records_key = found
        item_schema = resolver.deref(item_schema)
        if item_schema.get("type") not in (None, "object") or not item_schema.get("properties"):
            warn(f"skipping {path}: list items are not objects with properties")
            continue
        params = collect_params(path_item, op, resolver)
        entries.append(
            {
                "path": path,
                "op": op,
                "records": records,
                "records_key": records_key,
                "item_schema": item_schema,
                "envelope": envelope,
                "params": params,
            }
        )

    if not entries:
        fail("no list-style GET endpoints found in the spec")

    resource_names(entries)
    if include:
        entries = [e for e in entries if matches(include, e["name"], e["path"])]
    if exclude:
        entries = [e for e in entries if not matches(exclude, e["name"], e["path"])]
    if not entries:
        fail("all resources were filtered out by --include/--exclude")

    for e in entries:
        query_params = [p for p in e["params"] if p.get("in") == "query"]
        query_names = {p["name"] for p in query_params}
        pagination, consumed = detect_pagination(e["op"], query_names, e["envelope"], e["records_key"], resolver)
        if pagination is None:
            warn(f"no pagination detected for {e['name']} ({e['path']}); defaulting to none")
        e["pagination"] = pagination or "none"

        query = CMap()
        if pagination and not isinstance(pagination, str) and ("next_url" in pagination or "cursor" in pagination):
            size = next((s for s in PAGE_SIZE_PARAMS if s in query_names and s not in consumed), None)
            if size:
                query[size] = "100"
                consumed = consumed | {size}
        for p in query_params:
            if p.get("required") and p["name"] not in consumed:
                cfg = slug(p["name"])
                config.setdefault(
                    cfg,
                    {"type": "string", "required": True, "help": one_line(p.get("description", "")) or f"Required query param {p['name']}"},
                )
                query[p["name"]] = f"{{{{ config.{cfg} }}}}"
        e["query"] = query

        path_params = [p for p in e["params"] if p.get("in") == "path"]
        params_map = CMap()
        for p in path_params:
            cfg = slug(p["name"])
            params_map[p["name"]] = f"config.{cfg}"
            config.setdefault(
                cfg,
                {"type": "string", "required": True, "help": one_line(p.get("description", "")) or f"Value for {{{p['name']}}} in {e['path']}"},
            )
        e["params_map"] = params_map
        if path_params:
            warn(f"{e['name']} has path params mapped to config ({', '.join(params_map)}); consider parent/for_each wiring")

        e["fields"] = build_fields(e["item_schema"], resolver)
        e["primary_key"] = pick_primary_key(e["fields"])
        if e["primary_key"] is None:
            warn(f"no primary key candidate for {e['name']}; set one by hand")

    pag_key = lambda p: json.dumps(p, sort_keys=True, default=dict)
    common_pag = Counter(pag_key(e["pagination"]) for e in entries).most_common(1)[0][0]
    common_records = Counter(e["records"] for e in entries).most_common(1)[0][0]
    queries = [json.dumps(e["query"], sort_keys=True) for e in entries]
    hoist_query = entries[0]["query"] if entries[0]["query"] and len(set(queries)) == 1 else None

    manifest = CMap()
    manifest["version"] = 1
    manifest["name"] = name
    if config:
        manifest["config"] = config
    defaults = CMap(method="GET")
    default_q = CMap(extra_query)
    if hoist_query:
        default_q.update(hoist_query)
    if default_q:
        defaults["query"] = default_q
    response = CMap(records=common_records)
    response["pagination"] = next(e["pagination"] for e in entries if pag_key(e["pagination"]) == common_pag)
    defaults["response"] = response
    manifest["defaults"] = defaults

    connection = CMap(base_url=base_url(spec))
    if auth:
        connection["auth"] = auth
    connection["headers"] = {"Accept": "application/json"}
    manifest["connection"] = connection

    resources = []
    for e in entries:
        r = CMap(name=e["name"], path=e["path"])
        if e["params_map"]:
            r["params"] = e["params_map"]
            r.key_comments["params"] = "TODO: path params are mapped to config; consider parent/for_each wiring instead"
        if e["query"] and not hoist_query:
            r["query"] = e["query"]
        if e["primary_key"]:
            r["primary_key"] = e["primary_key"]
        else:
            r.key_comments["fields"] = "TODO: no primary key candidate found; declare primary_key with a real unique field"
        if e["records"] != common_records:
            r["records"] = e["records"]
        r["fields"] = e["fields"]
        if pag_key(e["pagination"]) != common_pag:
            r["pagination"] = e["pagination"]
        resources.append(r)
    manifest["resources"] = resources
    manifest["discovery"] = {"mode": "static"}

    return dump_manifest(manifest)


# ---------------------------------------------------------------------------
# Validation and CLI


def run_validation(out_path: Path) -> bool:
    try:
        out_path.resolve().relative_to(MANIFESTS_DIR)
    except ValueError:
        print(f"note: {out_path} is outside {MANIFESTS_DIR}; skipping go test validation", file=sys.stderr)
        return True
    print("\nrunning: go test ./connectors/http -run TestManifests", file=sys.stderr)
    proc = subprocess.run(
        ["go", "test", "./connectors/http", "-run", "TestManifests"],
        cwd=REPO_ROOT,
        capture_output=True,
        text=True,
    )
    sys.stderr.write(proc.stdout + proc.stderr)
    return proc.returncode == 0


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("spec", help="OpenAPI 3.x spec: local file or http(s) URL, JSON or YAML")
    ap.add_argument("--name", help="connector name (default: slug of info.title)")
    ap.add_argument("--out", help="output path (default: connectors/http/manifests/<name>.yaml)")
    ap.add_argument("--include", action="append", default=[], metavar="GLOB", help="only keep resources matching this glob (name or path); repeatable")
    ap.add_argument("--exclude", action="append", default=[], metavar="GLOB", help="drop resources matching this glob (name or path); repeatable")
    ap.add_argument("--no-validate", action="store_true", help="skip the go test validation step")
    args = ap.parse_args()

    spec = load_spec(args.spec)
    name = slug(args.name) if args.name else default_name(spec)
    out_path = Path(args.out) if args.out else MANIFESTS_DIR / f"{name}.yaml"

    text = convert(spec, name, args.include, args.exclude)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(text)

    resource_count = text.count("\n  - name:")
    print(f"\nwrote {out_path} ({resource_count} resources, {len(WARNINGS)} warnings)", file=sys.stderr)

    ok = True
    if not args.no_validate:
        ok = run_validation(out_path)
        print("validation: PASS" if ok else "validation: FAIL (manifest left in place for iteration)", file=sys.stderr)
    print(
        "next: review TODOs/warnings, then wire the connector (catalog.go, register.go, docs) — see .claude/skills/http-connector",
        file=sys.stderr,
    )
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()