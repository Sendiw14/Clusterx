-- Reconciliation + latency + contention report for one benchmark run (run as clusterx_admin).
\pset footer off
\echo '== jobs by status'
SELECT status, count(*) FROM jobs GROUP BY 1 ORDER BY 1;
\echo '== jobs by workload class'
SELECT handler_name, coalesce(payload->>'n','-') n, coalesce(payload->>'permanent','false') permanent, status,
       count(*) jobs, sum(attempts_used) attempts
FROM jobs GROUP BY 1, 2, 3, 4 ORDER BY 1, 2, 3, 4;
\echo '== attempts by state'
SELECT state, count(*) FROM job_attempts GROUP BY 1 ORDER BY 1;
\echo '== invariants (mismatch/active/non_terminal columns must be 0)'
SELECT (SELECT count(*) FROM jobs) jobs,
  (SELECT count(*) FROM jobs WHERE status NOT IN ('succeeded', 'failed', 'canceled')) non_terminal,
  (SELECT count(*) FROM job_attempts) attempts,
  (SELECT sum(attempts_used) FROM jobs) attempts_used_sum,
  (SELECT sum(attempts_used - 1) FROM jobs WHERE attempts_used > 1) automatic_retries,
  (SELECT count(*) FROM jobs j WHERE attempts_used <> (SELECT count(*) FROM job_attempts a WHERE a.job_id = j.id)) attempt_count_mismatch,
  (SELECT count(*) FROM jobs j WHERE version <> (SELECT max(version) FROM job_events e WHERE e.job_id = j.id)
      OR version <> (SELECT count(*) FROM job_events e WHERE e.job_id = j.id)) event_version_mismatch,
  (SELECT count(*) FROM job_attempts WHERE state = 'active') active_attempts;
\echo '== end-to-end latency (terminal_at - created_at, ms) and throughput'
SELECT count(*) n,
  round(percentile_cont(0.50) WITHIN GROUP (ORDER BY extract(epoch FROM terminal_at - created_at)) * 1000) p50_ms,
  round(percentile_cont(0.95) WITHIN GROUP (ORDER BY extract(epoch FROM terminal_at - created_at)) * 1000) p95_ms,
  round(percentile_cont(0.99) WITHIN GROUP (ORDER BY extract(epoch FROM terminal_at - created_at)) * 1000) p99_ms,
  round(max(extract(epoch FROM terminal_at - created_at)) * 1000) max_ms,
  round(extract(epoch FROM max(terminal_at) - min(created_at))::numeric, 3) span_s,
  round((count(*) / extract(epoch FROM max(terminal_at) - min(created_at)))::numeric, 1) jobs_per_s
FROM jobs;
\echo '== end-to-end latency by class (ms)'
SELECT handler_name, coalesce(payload->>'permanent','false') permanent, count(*) n,
  round(percentile_cont(0.50) WITHIN GROUP (ORDER BY extract(epoch FROM terminal_at - created_at)) * 1000) p50_ms,
  round(percentile_cont(0.95) WITHIN GROUP (ORDER BY extract(epoch FROM terminal_at - created_at)) * 1000) p95_ms,
  round(percentile_cont(0.99) WITHIN GROUP (ORDER BY extract(epoch FROM terminal_at - created_at)) * 1000) p99_ms
FROM jobs GROUP BY 1, 2 ORDER BY 1, 2;
\echo '== queue wait (first attempt started_at - created_at, ms)'
SELECT round(percentile_cont(0.50) WITHIN GROUP (ORDER BY extract(epoch FROM a.started_at - j.created_at)) * 1000) p50_ms,
  round(percentile_cont(0.95) WITHIN GROUP (ORDER BY extract(epoch FROM a.started_at - j.created_at)) * 1000) p95_ms,
  round(percentile_cont(0.99) WITHIN GROUP (ORDER BY extract(epoch FROM a.started_at - j.created_at)) * 1000) p99_ms
FROM jobs j JOIN job_attempts a ON a.job_id = j.id AND a.attempt_number = 1;
\echo '== attempts per worker'
SELECT w.name, a.state, count(*) FROM job_attempts a JOIN workers w ON w.id = a.worker_id GROUP BY 1, 2 ORDER BY 1, 2;
\echo '== database counters since reset'
SELECT xact_commit, xact_rollback, deadlocks, conflicts, blks_hit, blks_read, temp_bytes, tup_inserted, tup_updated
FROM pg_stat_database WHERE datname = current_database();
\echo '== top statements by total time'
SELECT calls, round(total_exec_time) total_ms, round(mean_exec_time::numeric, 3) mean_ms, round(max_exec_time::numeric, 1) max_ms,
  left(regexp_replace(query, '\s+', ' ', 'g'), 100) query
FROM pg_stat_statements WHERE dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
ORDER BY total_exec_time DESC LIMIT 12;
