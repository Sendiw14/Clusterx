// Thin typed client over the generated OpenAPI types. Generated file: ./schema.d.ts (never hand-edit).
import createClient from "openapi-fetch";
import type { paths, components } from "./schema.js";

export type { paths, components };
export type ApiError = components["schemas"]["Error"];

export function createClusterxClient(baseUrl: string, init?: { fetch?: typeof fetch; headers?: HeadersInit }) {
  return createClient<paths>({ baseUrl, fetch: init?.fetch, headers: init?.headers });
}
