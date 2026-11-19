import { randomBytes } from "node:crypto";
import { defineConfig, devices } from "@playwright/test";
import { API_URL, KEY_FILE, PORTS } from "./e2e/stack/stack";
import { MOCK_CLIENT_ID, MOCK_CLIENT_SECRET, MOCK_OAUTH_PORT } from "./e2e/stack/mock-oauth";

const PORT = PORTS.web;

/**
 * Chromium only (Firefox/WebKit are not required for the M3 foundation). Runs against a production build: `pnpm build`
 * must have run first (the package script order lint, typecheck, test, build, e2e guarantees it).
 */
export default defineConfig({
  testDir: "./e2e",
  timeout: 60_000,
  expect: { timeout: 10_000 },
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: 0,
  globalSetup: "./e2e/global-setup.ts",
  globalTeardown: "./e2e/global-teardown.ts",
  reporter: [["list"], ["json", { outputFile: "test-results/e2e.json" }]],
  use: { baseURL: `http://localhost:${PORT}`, trace: "retain-on-failure" },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    // The production entry (`server.ts`: Next plus the /bff/stream relay), wired to the live stack that global setup starts
    // (Go control plane, PostgreSQL, mock OAuth). Plain test credentials: they exist only on this machine for this run.
    command: "node server.ts",
    env: {
      NODE_ENV: "production",
      PORT: String(PORT),
      CLUSTERX_ENV: "test",
      CLUSTERX_APP_ORIGIN: `http://localhost:${PORT}`,
      CLUSTERX_API_URL: API_URL,
      CLUSTERX_BFF_KEY_FILE: KEY_FILE,
      CLUSTERX_WEB_SESSION_KEY: randomBytes(32).toString("base64"),
      CLUSTERX_GITHUB_CLIENT_ID: MOCK_CLIENT_ID,
      CLUSTERX_GITHUB_CLIENT_SECRET: MOCK_CLIENT_SECRET,
      CLUSTERX_GITHUB_AUTHORIZE_URL: `http://localhost:${MOCK_OAUTH_PORT}/authorize`,
      CLUSTERX_GITHUB_TOKEN_URL: `http://127.0.0.1:${MOCK_OAUTH_PORT}/token`,
      CLUSTERX_GITHUB_USER_URL: `http://127.0.0.1:${MOCK_OAUTH_PORT}/user`,
    },
    url: `http://localhost:${PORT}/`,
    reuseExistingServer: false,
    timeout: 60_000,
  },
});
