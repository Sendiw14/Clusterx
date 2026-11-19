import { execFileSync, spawn, type ChildProcess } from "node:child_process";
import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { MOCK_CLIENT_ID, MOCK_CLIENT_SECRET, MOCK_OAUTH_PORT, startMockOAuth } from "./mock-oauth";

/**
 * Live E2E stack, started by Playwright global setup: own PostgreSQL 18 cluster, the real control-plane and worker
 * binaries built from this worktree, a mock OAuth server. It never touches the integrator's compose project on 8080.
 * Ports: PG 55440, HTTP 18180, gRPC 19190, metrics 19191, mock OAuth 19300, web 3100.
 */
const here = dirname(fileURLToPath(import.meta.url));
export const WEB_DIR = resolve(here, "../..");
const REPO = resolve(WEB_DIR, "../..");
export const STACK_DIR = join(WEB_DIR, ".e2e");
export const PORTS = { pg: 55440, http: 18180, grpc: 19190, metrics: 19191, web: 3100 };
export const PG_BIN = process.env.CLUSTERX_E2E_PG_BIN ?? "/opt/homebrew/opt/postgresql@18/bin";
const PG_DIR = process.env.CLUSTERX_E2E_PG_DIR ?? join(tmpdir(), "clusterx-e2e-pg");
const BIN_DIR = join(STACK_DIR, "bin");
export const KEY_FILE = join(STACK_DIR, "bff-private.jwk.json");
const JWKS_FILE = join(STACK_DIR, "bff.jwks.json");
export const API_URL = `http://127.0.0.1:${PORTS.http}`;
const ADMIN_DSN = `postgres://clusterx_admin@127.0.0.1:${PORTS.pg}/clusterx?sslmode=disable`;
const APP_DSN = `postgres://clusterx_app@127.0.0.1:${PORTS.pg}/clusterx?sslmode=disable`;
const PGENV = { ...process.env, LC_ALL: "en_US.UTF-8", PATH: `${PG_BIN}:${process.env.PATH}` };

/** Users the tests sign in as (numeric GitHub id, login, role). 1003 is deliberately not allowlisted. */
export const USERS = {
  operator: { id: 1001, login: "e2e-operator", role: "operator" },
  viewer: { id: 1002, login: "e2e-viewer", role: "viewer" },
  stranger: { id: 1003, login: "e2e-stranger", role: "none" },
  disabled: { id: 1004, login: "e2e-disabled", role: "operator" },
  limited: { id: 1005, login: "e2e-limited", role: "operator" },
  expiring: { id: 1006, login: "e2e-expiring", role: "operator" },
  /** One per spec file that disables or expires sessions, so parallel specs never share a victim. */
  victim: { id: 1007, login: "e2e-victim", role: "operator" },
} as const;

const children: ChildProcess[] = [];
let oauth: Awaited<ReturnType<typeof startMockOAuth>> | undefined;

const goEnv = (extra: Record<string, string> = {}) => ({
  ...process.env,
  CLUSTERX_ENV: "development",
  CLUSTERX_LOG_LEVEL: "warn",
  CLUSTERX_DATABASE_URL: APP_DSN,
  CLUSTERX_MIGRATE_DATABASE_URL: ADMIN_DSN,
  CLUSTERX_BFF_JWKS_FILE: JWKS_FILE,
  ...extra,
});

function run(cmd: string, args: string[], opts: { env?: NodeJS.ProcessEnv; cwd?: string; input?: string } = {}): string {
  return execFileSync(cmd, args, { env: opts.env ?? PGENV, cwd: opts.cwd, input: opts.input, encoding: "utf8", stdio: ["pipe", "pipe", "pipe"], timeout: 300_000 });
}
const go = () => (existsSync("/opt/homebrew/bin/go") ? "/opt/homebrew/bin/go" : "go");

/** Admin SQL for tests (expire or inspect sessions, read audit rows). Local throwaway cluster only. */
export function sql(statement: string): string {
  return run(join(PG_BIN, "psql"), ["-h", "127.0.0.1", "-p", String(PORTS.pg), "-U", "clusterx_admin", "-d", "clusterx", "-At", "-v", "ON_ERROR_STOP=1", "-c", statement]);
}
/** The coordinator's own CLI (user upsert/disable): same code paths an operator uses. */
export function cli(...args: string[]): string {
  return run(join(BIN_DIR, "clusterx-control-plane"), args, { env: goEnv() });
}

async function waitFor(what: string, probe: () => Promise<boolean>, ms = 60_000) {
  const end = Date.now() + ms;
  while (Date.now() < end) {
    try { if (await probe()) return; } catch { /* not yet */ }
    await new Promise((r) => setTimeout(r, 250));
  }
  throw new Error(`timed out waiting for ${what}`);
}

function spawnLogged(name: string, cmd: string, args: string[], env: NodeJS.ProcessEnv) {
  const p = spawn(cmd, args, { env, stdio: ["ignore", "pipe", "pipe"] });
  const log: string[] = [];
  for (const s of [p.stdout, p.stderr]) s.on("data", (d: Buffer) => { log.push(d.toString()); if (log.length > 400) log.shift(); });
  p.on("exit", (code, sig) => { if (code !== 0 && sig !== "SIGTERM" && sig !== "SIGKILL") console.error(`[e2e] ${name} exited code=${code} signal=${sig}\n${log.join("").slice(-2000)}`); });
  children.push(p);
  return p;
}

export async function startStack(): Promise<void> {
  rmSync(STACK_DIR, { recursive: true, force: true });
  mkdirSync(BIN_DIR, { recursive: true });
  const cpEnv = { ...process.env };
  console.log("[e2e] building control plane and worker (go build)");
  run(go(), ["build", "-o", join(BIN_DIR, "clusterx-control-plane"), "./cmd/clusterx-control-plane"], { cwd: join(REPO, "apps/control-plane"), env: cpEnv });
  run(go(), ["build", "-o", join(BIN_DIR, "clusterx-worker"), "./cmd/clusterx-worker"], { cwd: join(REPO, "apps/worker"), env: cpEnv });

  console.log(`[e2e] PostgreSQL cluster ${PG_DIR} on ${PORTS.pg}`);
  if (existsSync(join(PG_DIR, "postmaster.pid"))) try { run(join(PG_BIN, "pg_ctl"), ["-D", PG_DIR, "stop", "-m", "immediate"]); } catch { /* stale */ }
  rmSync(PG_DIR, { recursive: true, force: true });
  run(join(PG_BIN, "initdb"), ["-D", PG_DIR, "-U", "clusterx_admin", "--auth=trust", "-E", "UTF8", "--locale=en_US.UTF-8"]);
  run(join(PG_BIN, "pg_ctl"), ["-D", PG_DIR, "-w", "-l", join(STACK_DIR, "pg.log"), "-o", `-p ${PORTS.pg} -k '' -c listen_addresses=127.0.0.1 -c fsync=off`, "start"]);
  const psql = (db: string, q: string) => run(join(PG_BIN, "psql"), ["-h", "127.0.0.1", "-p", String(PORTS.pg), "-U", "clusterx_admin", "-d", db, "-v", "ON_ERROR_STOP=1", "-c", q]);
  psql("postgres", "CREATE DATABASE clusterx");
  psql("clusterx", "CREATE ROLE clusterx_runtime NOLOGIN; CREATE ROLE clusterx_retention NOLOGIN; CREATE ROLE clusterx_app LOGIN IN ROLE clusterx_runtime");

  // BFF signing key: generated by the coordinator's own dev-keygen (public JWKS to Go, private JWK to the web server only).
  run(join(BIN_DIR, "clusterx-control-plane"), ["dev-keygen", "--out-dir", STACK_DIR, "--kid", "e2e-1"], { env: goEnv() });
  writeFileSync(KEY_FILE, readFileSync(join(STACK_DIR, "bff-dev-private.jwk.json")), { mode: 0o600 });
  writeFileSync(JWKS_FILE, readFileSync(join(STACK_DIR, "bff-dev.jwks.json")));
  run(join(BIN_DIR, "clusterx-control-plane"), ["migrate"], { env: goEnv() });
  for (const u of Object.values(USERS)) if (u.role !== "none") cli("user", "upsert", "--github-id", String(u.id), "--login", u.login, "--role", u.role);
  cli("user", "disable", "--github-id", String(USERS.disabled.id));

  console.log("[e2e] control plane");
  spawnLogged("control-plane", join(BIN_DIR, "clusterx-control-plane"), ["serve"], goEnv({
    CLUSTERX_HTTP_ADDR: `127.0.0.1:${PORTS.http}`, CLUSTERX_GRPC_ADDR: `127.0.0.1:${PORTS.grpc}`, CLUSTERX_METRICS_ADDR: `127.0.0.1:${PORTS.metrics}`, CLUSTERX_METRICS_TOKEN: "e2e-metrics-token-not-a-secret-0000",
    // Parallel specs sign in a lot; the dedicated rate-limit user still hits the real mutation limit.
    CLUSTERX_RATE_LOGINS_PER_MINUTE: "600", CLUSTERX_RATE_AUTH_FAILURES_PER_MINUTE: "600", CLUSTERX_RATE_MUTATIONS_PER_SECOND: "20",
  }));
  await waitFor("control plane ready", async () => (await fetch(`${API_URL}/health/ready`)).ok);

  // Two workers: the drain spec drains `e2e-worker-drain` and nothing else, so job specs keep a live worker.
  for (const name of ["e2e-worker-1", "e2e-worker-drain"]) {
    const cred = cli("worker-credential", "create", "--name", name).match(/^secret=(.+)$/m)?.[1];
    if (!cred) throw new Error("worker credential was not created");
    spawnLogged(name, join(BIN_DIR, "clusterx-worker"), [], { ...process.env, CLUSTERX_ENV: "development", CLUSTERX_LOG_LEVEL: "warn", CLUSTERX_WORKER_CONTROL_PLANE_ADDR: `127.0.0.1:${PORTS.grpc}`, CLUSTERX_WORKER_INSECURE: "true", CLUSTERX_WORKER_NAME: name, CLUSTERX_WORKER_CAPACITY: "4", CLUSTERX_WORKER_CREDENTIAL: cred });
  }
  await waitFor("a live worker", async () => {
    const r = sql("SELECT count(*) FROM workers WHERE status = 'active'");
    return Number(r.trim()) >= 2;
  });

  oauth = await startMockOAuth();
  console.log(`[e2e] mock OAuth on ${MOCK_OAUTH_PORT} (client ${MOCK_CLIENT_ID})`);
  void MOCK_CLIENT_SECRET;
}

export async function stopStack(): Promise<void> {
  for (const c of children.splice(0)) c.kill("SIGTERM");
  await new Promise((r) => setTimeout(r, 800));
  oauth?.server.close();
  try { run(join(PG_BIN, "pg_ctl"), ["-D", PG_DIR, "stop", "-m", "fast"]); } catch { /* not running */ }
}
