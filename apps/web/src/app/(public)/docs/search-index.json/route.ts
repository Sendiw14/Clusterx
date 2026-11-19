import { buildSearchIndex } from "@/lib/docs/search-index";

export const dynamic = "force-static";

/** Static build-time search index for the docs (no external service). */
export function GET() {
  return Response.json(buildSearchIndex(), { headers: { "Cache-Control": "public, max-age=300" } });
}
