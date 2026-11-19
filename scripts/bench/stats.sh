#!/usr/bin/env bash
# Summarise docker-stats.csv and lock-samples.csv of one run directory: per container mean/max CPU%
# (100% = one VM core) and max memory; lock-wait samples.
set -euo pipefail
d=$1
awk -F, '{gsub(/%/,"",$3); split($4,m," "); v=m[1]; u=v; sub(/[0-9.]+/,"",u); sub(/[A-Za-z]+/,"",v);
  if (u=="GiB") v*=1024; else if (u=="KiB") v/=1024; else if (u=="B") v/=1048576;
  n[$2]++; s[$2]+=$3; if ($3>c[$2]) c[$2]=$3; if (v>mm[$2]) mm[$2]=v}
  END {printf "%-32s %8s %8s %8s %10s\n","container","samples","cpu_mean","cpu_max","mem_max_MiB";
  for (k in n) printf "%-32s %8d %8.1f %8.1f %10.1f\n",k,n[k],s[k]/n[k],c[k],mm[k]}' "$d/docker-stats.csv" | sort
# CPU-seconds over the run from cgroup counters (a container that was (re)started mid-run counts from 0).
wall=$(sed -n 's/.*wall_s_driver_start_to_all_terminal=\([0-9]*\).*/\1/p' "$d/driver.txt")
awk -v wall="$wall" 'NR==FNR {b[$1]=$2; next} {cpu=($2-b[$1])/1e6; printf "%-32s cpu_s=%7.2f avg_cores=%5.2f mem_peak_MiB=%7.1f mem_now_MiB=%7.1f\n", $1, cpu, cpu/wall, $3/1048576, $4/1048576}' \
  "$d/cgroup-before.txt" "$d/cgroup-after.txt" | sort
awk -F, '{n++; if ($2>0) w++; if ($2>m) m=$2; if ($4>b) b=$4} END {printf "lock samples=%d with_waiters=%d max_ungranted_locks=%d max_backends=%d\n", n, w, m, b}' "$d/lock-samples.csv"
