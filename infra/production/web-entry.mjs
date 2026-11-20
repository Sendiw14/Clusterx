// Production entry for the web container (infra/production/web.Dockerfile). The web config (apps/web/src/server/config.ts)
// reads secrets from environment variables; this wrapper lets compose pass them as files instead (not visible in
// `docker inspect`): every CLUSTERX_<NAME>_FILE (except CLUSTERX_BFF_KEY_FILE, which is itself a path setting) is read
// into CLUSTERX_<NAME> and removed, then apps/web/server.ts starts unchanged. Requested of the web lane: native *_FILE.
import { readFileSync } from "node:fs";

for (const [k, path] of Object.entries(process.env)) {
  if (!k.startsWith("CLUSTERX_") || !k.endsWith("_FILE") || k === "CLUSTERX_BFF_KEY_FILE") continue;
  const name = k.slice(0, -"_FILE".length);
  if (process.env[name]) {
    console.error(`[web-entry] set ${name} or ${k}, not both`);
    process.exit(1);
  }
  try {
    process.env[name] = readFileSync(path, "utf8").trim();
  } catch {
    console.error(`[web-entry] ${k}: cannot read ${path}`);
    process.exit(1);
  }
  delete process.env[k];
}
await import("/app/apps/web/server.ts");
