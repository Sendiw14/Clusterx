/**
 * Commands shown on the home page and in the quickstart. Every line here is one the integrator executed or
 * that appears in an executed guide (see VERIFICATION). Do not add a command without evidence; list it as
 * "not yet verified" in the docs instead. A Vitest test checks that the quickstart doc contains these lines.
 */
export const START_STACK = `git clone <repository-url> clusterx
cd clusterx
cp .env.example .env
# fill the three empty values in .env, for example with: openssl rand -hex 24
./infra/compose/bootstrap.sh`;

export const SUBMIT_JOB = `export CLUSTERX_TOKEN=<value of CLUSTERX_OPERATOR_TOKEN in infra/compose/.secrets.env>

curl -X POST http://127.0.0.1:8080/api/v1/jobs \\
  -H "Authorization: Bearer $CLUSTERX_TOKEN" \\
  -H "Content-Type: application/json" \\
  -H "Idempotency-Key: guide-1" \\
  -d '{"handler_name":"hash_sha256","handler_version":1,"payload":{"data":"hello"},"priority":5}'`;

export const HOME_QUICKSTART = `${START_STACK}

# then submit a job (hash_sha256 is one of the three example handlers)
${SUBMIT_JOB}`;

/** What was actually run, by whom, and what is still open. Rendered next to the snippets. */
export const VERIFICATION = {
  verified: "Integrator, 2026-11-18, Colima: bootstrap.sh exited 0 and left 3 live workers; POST /api/v1/jobs with hash_sha256 and fail_n_times both succeeded. Independent audit, 2026-11-20: the same from a clean clone on macOS (under $HOME) and in a Linux arm64 VM, plus a killed worker recovered.",
  curlForm: "The curl text above is the form executed in the M4 curl guide against built binaries on port 18080. Port 8080 is the port the compose file publishes.",
  open: "A Linux amd64 host has not been tried, and the console is not part of this path: it needs a GitHub OAuth app (see the Quickstart for the local mock-sign-in route).",
} as const;
