import { readFileSync } from "node:fs";
import path from "node:path";
import { parse } from "yaml";

/**
 * Builds the API reference model from api/openapi.yaml at build time (AC-28: generated, never hand-copied).
 * Only internal `#/components/...` references are resolved. A missing or unparsable spec fails the build.
 */

type Json = Record<string, unknown>;
const isObj = (v: unknown): v is Json => typeof v === "object" && v !== null && !Array.isArray(v);

export interface FieldRow { name: string; type: string; required: boolean; description: string }
export interface ParamRow { name: string; in: string; required: boolean; type: string; description: string }
export interface ExampleBlock { label: string; status?: string; json: string }
export interface ResponseRow { status: string; description: string; schema: string; headers: string[] }
export interface Operation {
  id: string; method: string; path: string; summary: string; tag: string;
  auth: string[]; params: ParamRow[];
  requestType: string | null; requestRows: FieldRow[]; requestExamples: ExampleBlock[];
  responses: ResponseRow[]; responseExamples: ExampleBlock[];
  served: boolean | null;
}
export interface SchemaDoc { name: string; description: string; rows: FieldRow[]; enumValues: string[] }
export interface ErrorResponseDoc { name: string; status: string; description: string; code: string }
export interface ApiModel {
  title: string; version: string; summary: string; description: string; servers: string[];
  securitySchemes: { name: string; description: string }[];
  errorCodes: string[];
  errorResponses: ErrorResponseDoc[];
  servedIds: string[] | null;
  operations: Operation[];
  tags: { name: string; description: string }[];
  schemas: SchemaDoc[];
}

const SPEC_PATH = path.join(process.cwd(), "..", "..", "api", "openapi.yaml");

/** Operations whose behaviour was executed in docs/coordination/handoffs/HANDOFF-M4-BE-01.md (curl guide). */
export const EXECUTED_IN_CURL_GUIDE = new Set([
  "createJob", "listJobs", "getJob", "listJobAttempts", "listJobEvents", "listWorkers", "listHandlers", "getConfig", "healthLive", "healthReady", "prometheusMetrics",
]);

function deref(root: Json, node: unknown, seen = 0): unknown {
  if (isObj(node) && typeof node.$ref === "string") {
    if (seen > 20) throw new Error("openapi: $ref loop");
    if (!node.$ref.startsWith("#/")) throw new Error(`openapi: external $ref ${node.$ref}`);
    let cur: unknown = root;
    for (const seg of node.$ref.slice(2).split("/")) cur = isObj(cur) ? cur[seg.replace(/~1/g, "/").replace(/~0/g, "~")] : undefined;
    if (cur === undefined) throw new Error(`openapi: unresolved ${node.$ref}`);
    return deref(root, cur, seen + 1);
  }
  return node;
}
const refName = (n: unknown): string | null => (isObj(n) && typeof n.$ref === "string" ? n.$ref.split("/").pop()! : null);

function typeLabel(root: Json, schema: unknown): string {
  const nm = refName(schema);
  const s = deref(root, schema);
  if (!isObj(s)) return "any";
  if (nm) return nm;
  if (Array.isArray(s.allOf)) return s.allOf.map((x) => typeLabel(root, x)).join(" + ");
  if (Array.isArray(s.oneOf)) return s.oneOf.map((x) => typeLabel(root, x)).join(" | ");
  if (Array.isArray(s.enum)) return `enum: ${s.enum.map(String).join(" | ")}`;
  let t = Array.isArray(s.type) ? s.type.join(" | ") : typeof s.type === "string" ? s.type : isObj(s.properties) ? "object" : "any";
  if (t === "array" && s.items) t = `array of ${typeLabel(root, s.items)}`;
  if (typeof s.format === "string") t += ` (${s.format})`;
  const bounds: string[] = [];
  if (typeof s.minimum === "number" || typeof s.maximum === "number") bounds.push(`${s.minimum ?? ""}..${s.maximum ?? ""}`);
  if (typeof s.minLength === "number" || typeof s.maxLength === "number") bounds.push(`length ${s.minLength ?? 0}..${s.maxLength ?? ""}`);
  if (typeof s.maxItems === "number") bounds.push(`max ${s.maxItems} items`);
  if (s.default !== undefined) bounds.push(`default ${JSON.stringify(s.default)}`);
  return bounds.length ? `${t}, ${bounds.join(", ")}` : t;
}

function describe(root: Json, schema: unknown): string {
  const s = deref(root, schema);
  return isObj(s) && typeof s.description === "string" ? s.description.replace(/\s+/g, " ").trim() : "";
}

function rowsOf(root: Json, schema: unknown, depth = 0): FieldRow[] {
  const d0 = deref(root, schema);
  if (!isObj(d0)) return [];
  let s: Json = d0;
  if (Array.isArray(s.allOf)) {
    const merged: Json = { properties: {}, required: [] };
    for (const part of s.allOf) {
      const p = deref(root, part);
      if (isObj(p)) {
        Object.assign(merged.properties as Json, isObj(p.properties) ? p.properties : {});
        (merged.required as string[]).push(...(Array.isArray(p.required) ? (p.required as string[]) : []));
      }
    }
    s = merged;
  }
  const props = isObj(s.properties) ? s.properties : {};
  const req = new Set(Array.isArray(s.required) ? (s.required as string[]) : []);
  const out: FieldRow[] = [];
  for (const [k, v] of Object.entries(props)) {
    out.push({ name: k, type: typeLabel(root, v), required: req.has(k), description: describe(root, v) });
    const d = deref(root, v);
    if (depth < 1 && isObj(d) && isObj(d.properties) && !refName(v)) {
      for (const r of rowsOf(root, d, depth + 1)) out.push({ ...r, name: `${k}.${r.name}` });
    }
  }
  return out;
}

function jsonBlock(v: unknown): string { return JSON.stringify(v, null, 2); }

function examplesOf(root: Json, media: unknown, status?: string): ExampleBlock[] {
  if (!isObj(media)) return [];
  const out: ExampleBlock[] = [];
  if (isObj(media.examples)) {
    for (const [name, ex] of Object.entries(media.examples)) {
      const e = deref(root, ex);
      if (isObj(e) && "value" in e) out.push({ label: name, status, json: jsonBlock(e.value) });
    }
  } else if ("example" in media) out.push({ label: "example", status, json: jsonBlock(media.example) });
  return out;
}

export function loadApi(): ApiModel {
  const raw = readFileSync(SPEC_PATH, "utf8");
  const root = parse(raw) as Json;
  if (!isObj(root) || !isObj(root.info) || !isObj(root.paths)) throw new Error("openapi: not an OpenAPI document");
  const info = root.info;
  const description = String(info.description ?? "");
  // From draft.6 the contract says "every operation is routed" (apps/control-plane/internal/api/v1.go registers all of them; a unit
  // test parses that file). Older drafts listed the served subset in a sentence. The M4-era sentence stays in the changelog text,
  // so the draft.6 statement is checked first.
  const allRouted = /every operation is routed/i.test(description);
  const m = /serves\s+([\s\S]*?)\s+with direct API tokens/.exec(description);
  const listed = m ? m[1]!.replace(/\s+and\s+/g, ",").split(",").map((x) => x.trim()).filter(Boolean) : null;
  const servedIds = allRouted ? Object.values(root.paths).flatMap((item) => (isObj(item) ? ["get", "post", "put", "patch", "delete"].flatMap((mt) => (isObj(item[mt]) && typeof (item[mt] as Json).operationId === "string" ? [(item[mt] as Json).operationId as string] : [])) : [])) : listed;

  const comps = isObj(root.components) ? root.components : {};
  const globalSec = Array.isArray(root.security) ? root.security.flatMap((s) => Object.keys(s as Json)) : [];
  const operations: Operation[] = [];

  for (const [p, item] of Object.entries(root.paths)) {
    if (!isObj(item)) continue;
    const pathParams = Array.isArray(item.parameters) ? item.parameters : [];
    for (const method of ["get", "post", "put", "patch", "delete"]) {
      const op = item[method];
      if (!isObj(op)) continue;
      const id = String(op.operationId ?? `${method}-${p}`);
      const params: ParamRow[] = [...pathParams, ...(Array.isArray(op.parameters) ? op.parameters : [])].map((x) => {
        const d = deref(root, x) as Json;
        return { name: String(d.name), in: String(d.in), required: d.required === true, type: typeLabel(root, d.schema), description: describe(root, d) };
      });
      let requestType: string | null = null; let requestRows: FieldRow[] = []; let requestExamples: ExampleBlock[] = [];
      const rb = deref(root, op.requestBody);
      if (isObj(rb) && isObj(rb.content)) {
        const [ct, media] = Object.entries(rb.content)[0] ?? [];
        if (ct && isObj(media)) { requestType = refName(media.schema) ?? ct; requestRows = rowsOf(root, media.schema); requestExamples = examplesOf(root, media); }
      }
      const responses: ResponseRow[] = []; const responseExamples: ExampleBlock[] = [];
      for (const [status, r] of Object.entries(isObj(op.responses) ? op.responses : {})) {
        const d = deref(root, r);
        if (!isObj(d)) continue;
        const media = isObj(d.content) ? Object.values(d.content)[0] : undefined;
        const schema = isObj(media) ? (refName(media.schema) ?? (isObj(media.schema) && Array.isArray(media.schema.allOf) ? "Error" : typeLabel(root, media.schema))) : "none";
        responses.push({ status, description: String(d.description ?? "").replace(/\s+/g, " ").trim(), schema, headers: isObj(d.headers) ? Object.keys(d.headers) : [] });
        if (status.startsWith("2") && responseExamples.length < 3) responseExamples.push(...examplesOf(root, media, status).slice(0, 2));
      }
      const security = Array.isArray(op.security) ? op.security.flatMap((s) => Object.keys(s as Json)) : globalSec;
      operations.push({
        id, method: method.toUpperCase(), path: p, summary: String(op.summary ?? ""), tag: Array.isArray(op.tags) ? String(op.tags[0]) : "other",
        auth: security, params, requestType, requestRows, requestExamples, responses, responseExamples,
        served: id === "healthLive" || id === "healthReady" || id === "prometheusMetrics" ? true : servedIds ? servedIds.includes(id) : null,
      });
    }
  }

  const schemas: SchemaDoc[] = Object.entries(isObj(comps.schemas) ? comps.schemas : {}).map(([name, s]) => {
    const d = deref(root, s);
    return { name, description: describe(root, s), rows: rowsOf(root, s), enumValues: isObj(d) && Array.isArray(d.enum) ? d.enum.map(String) : [] };
  });
  const errSchema = schemas.find((s) => s.name === "Error");
  const errCodeRow = errSchema?.rows.find((r) => r.name === "code");
  const errorCodes = errCodeRow ? errCodeRow.type.replace(/^enum:\s*/, "").split(" | ") : [];

  const errorResponses: ErrorResponseDoc[] = [];
  for (const [name, r] of Object.entries(isObj(comps.responses) ? comps.responses : {})) {
    const d = r as Json;
    const media = isObj(d.content) ? (Object.values(d.content)[0] as Json | undefined) : undefined;
    const ex = media && isObj(media.example) ? (media.example as Json) : null;
    // the HTTP status is taken from the operations that reference this component response
    let status = "";
    for (const [, item] of Object.entries(root.paths)) for (const mth of ["get", "post", "delete"]) {
      const op = isObj(item) ? (item as Json)[mth] : undefined;
      if (!isObj(op) || !isObj(op.responses)) continue;
      for (const [st, rr] of Object.entries(op.responses)) if (refName(rr) === name && !status) status = st;
    }
    errorResponses.push({ name, status, description: String(d.description ?? "").replace(/\s+/g, " ").trim(), code: ex && typeof ex.code === "string" ? ex.code : "" });
  }
  errorResponses.sort((a, b) => a.status.localeCompare(b.status));

  return {
    title: String(info.title), version: String(info.version), summary: String(info.summary ?? ""), description,
    servers: Array.isArray(root.servers) ? root.servers.map((s) => String((s as Json).url)) : [],
    securitySchemes: Object.entries(isObj(comps.securitySchemes) ? comps.securitySchemes : {}).map(([name, s]) => ({ name, description: String((s as Json).description ?? "") })),
    errorCodes, errorResponses, servedIds, operations,
    tags: Array.isArray(root.tags) ? root.tags.map((t) => ({ name: String((t as Json).name), description: String((t as Json).description ?? "") })) : [],
    schemas,
  };
}
