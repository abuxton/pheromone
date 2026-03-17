#!/usr/bin/env bash
# run-benchmark.sh — ADR-008 Ollama resource benchmark
#
# Measures:
#   1. RAM before/during/after inference for each model profile
#   2. Cold-start time (first token after model load)
#   3. Inference time for a system-management prompt (10 runs, p50/p90/p99)
#   4. Fallback latency (stop Ollama → OllamaReasoner falls back to rule-based)
#
# Success criteria (ADR-008 §Issue #25):
#   - phi3.5:3.8b-mini-instruct-q4_K_M RAM <= 3 GB on 8 GB instance
#   - qwen2.5:1.5b-instruct-q4_K_M      RAM <= 1.5 GB on 8 GB instance
#   - Inference time < 5 seconds (p50)
#   - Fallback to RuleBasedReasoner < 100 ms
#
# Outputs: ${RESULTS_DIR}/benchmark.json  (JSON results)
#          ${RESULTS_DIR}/benchmark.log   (human-readable log)
set -euo pipefail

RESULTS_DIR="${RESULTS_DIR:-/tmp/pheromone-benchmark}"
PHI_MODEL="phi3.5:3.8b-mini-instruct-q4_K_M"
QWEN_MODEL="qwen2.5:1.5b-instruct-q4_K_M"
OLLAMA_URL="http://localhost:11434"
INFERENCE_RUNS=10

# System-management prompt mirroring what OllamaReasoner sends (ADR-008)
PROMPT='You are a system-management AI. Analyse the following twin drift and return ONLY a JSON array of actions.

Twin ID: twin-benchmark
Desired version: 1.0.0
Drifted fields:
  - nginx_version: desired="1.24" actual="1.20"
  - swap_enabled: desired="false" actual="true"

Respond with a JSON array using this schema:
[{"skill_name":"<skill>","action_type":"<type>","twin_id":"<id>","params":{"key":"value"},"rationale":"<reason>"}]
Return an empty array [] if no action is needed.'

mkdir -p "${RESULTS_DIR}"
LOG="${RESULTS_DIR}/benchmark.log"
RESULT_JSON="${RESULTS_DIR}/benchmark.json"

log() { echo "[$(date -u +%H:%M:%S)] $*" | tee -a "${LOG}"; }

ram_mib() {
  # Returns current RSS of the ollama process in MiB (0 if not running)
  local pid
  pid=$(pgrep -f 'ollama serve' 2>/dev/null | head -1 || true)
  if [ -z "${pid}" ]; then echo "0"; return; fi
  awk '/VmRSS/ {printf "%.0f", $2/1024}' "/proc/${pid}/status" 2>/dev/null || echo "0"
}

total_ram_mib() {
  awk '/MemTotal/ {printf "%.0f", $2/1024}' /proc/meminfo
}

free_ram_mib() {
  awk '/MemAvailable/ {printf "%.0f", $2/1024}' /proc/meminfo
}

elapsed_ms() {
  # Usage: elapsed_ms <start_ns>
  local start_ns=$1
  local end_ns
  end_ns=$(date +%s%N)
  echo $(( (end_ns - start_ns) / 1000000 ))
}

ensure_ollama_running() {
  if ! curl -sf "${OLLAMA_URL}/" >/dev/null 2>&1; then
    log "Starting Ollama service..."
    systemctl start ollama
    for i in $(seq 1 30); do
      if curl -sf "${OLLAMA_URL}/" >/dev/null 2>&1; then
        log "Ollama API ready after ${i}s."
        return
      fi
      sleep 1
    done
    log "ERROR: Ollama API did not become ready in 30s"
    exit 1
  fi
}

benchmark_model() {
  local model="$1"
  local profile_name="$2"
  local ram_target_mib="$3"

  log "========================================================"
  log "Benchmarking: ${model} (${profile_name} profile)"
  log "RAM target: <= ${ram_target_mib} MiB"
  log "========================================================"

  ensure_ollama_running

  # Unload model from memory before measuring (force unload via short keepalive)
  curl -sf -X POST "${OLLAMA_URL}/api/generate" \
    -H 'Content-Type: application/json' \
    -d "{\"model\":\"${model}\",\"prompt\":\"hi\",\"stream\":false,\"keep_alive\":\"0s\"}" \
    >/dev/null 2>&1 || true
  sleep 2

  local ram_baseline
  ram_baseline=$(free_ram_mib)
  local proc_ram_before
  proc_ram_before=$(ram_mib)
  log "Baseline free RAM: ${ram_baseline} MiB"
  log "Ollama process RSS before load: ${proc_ram_before} MiB"

  # --- Cold-start measurement ---
  log "Measuring cold-start time (first inference after model load)..."
  local cold_start_ns
  cold_start_ns=$(date +%s%N)
  curl -sf -X POST "${OLLAMA_URL}/api/generate" \
    -H 'Content-Type: application/json' \
    -d "{\"model\":\"${model}\",\"prompt\":\"${PROMPT}\",\"stream\":false}" \
    -o "${RESULTS_DIR}/cold_start_response.json" 2>/dev/null
  local cold_start_ms
  cold_start_ms=$(elapsed_ms "${cold_start_ns}")

  local ram_during
  ram_during=$(ram_mib)
  local free_during
  free_during=$(free_ram_mib)
  local model_ram_mib=$(( ram_baseline - free_during ))
  [ "${model_ram_mib}" -lt 0 ] && model_ram_mib=0

  log "Cold-start time: ${cold_start_ms} ms"
  log "Ollama process RSS after load: ${ram_during} MiB"
  log "Estimated model RAM usage: ~${model_ram_mib} MiB"

  # --- Warm inference measurement (INFERENCE_RUNS runs) ---
  log "Measuring warm inference time (${INFERENCE_RUNS} runs)..."
  local times=()
  for i in $(seq 1 "${INFERENCE_RUNS}"); do
    local t_ns
    t_ns=$(date +%s%N)
    curl -sf -X POST "${OLLAMA_URL}/api/generate" \
      -H 'Content-Type: application/json' \
      -d "{\"model\":\"${model}\",\"prompt\":\"${PROMPT}\",\"stream\":false}" \
      -o /dev/null 2>/dev/null
    times+=( "$(elapsed_ms "${t_ns}")" )
    log "  run ${i}: ${times[-1]} ms"
  done

  # Sort and compute percentiles
  IFS=$'\n' sorted=($(printf '%s\n' "${times[@]}" | sort -n)); unset IFS
  local count=${#sorted[@]}
  local p50_idx=$(( count / 2 ))
  local p90_idx=$(( count * 90 / 100 ))
  local p99_idx=$(( count * 99 / 100 ))
  local p50="${sorted[${p50_idx}]}"
  local p90="${sorted[${p90_idx}]}"
  local p99="${sorted[${p99_idx}]:-${sorted[-1]}}"

  local total=0
  for t in "${sorted[@]}"; do (( total += t )); done
  local avg=$(( total / count ))

  log "Warm inference — avg: ${avg} ms  p50: ${p50} ms  p90: ${p90} ms  p99: ${p99} ms"

  # --- Criteria evaluation ---
  local ram_pass="FAIL"
  [ "${model_ram_mib}" -le "${ram_target_mib}" ] && ram_pass="PASS"

  local latency_pass="FAIL"
  [ "${p50}" -lt 5000 ] && latency_pass="PASS"

  log "RESULT ram_target_mib=${ram_target_mib}: ${ram_pass} (measured ~${model_ram_mib} MiB)"
  log "RESULT inference_p50<5000ms: ${latency_pass} (measured ${p50} ms)"

  # Emit JSON fragment
  cat >> "${RESULT_JSON}.parts" <<EOF
{
  "model": "${model}",
  "profile": "${profile_name}",
  "ram_target_mib": ${ram_target_mib},
  "ram_measured_mib": ${model_ram_mib},
  "ram_pass": ${ram_pass,,},
  "cold_start_ms": ${cold_start_ms},
  "warm_inference_avg_ms": ${avg},
  "warm_inference_p50_ms": ${p50},
  "warm_inference_p90_ms": ${p90},
  "warm_inference_p99_ms": ${p99},
  "inference_latency_pass": ${latency_pass,,}
}
EOF
}

benchmark_fallback() {
  log "========================================================"
  log "Benchmarking: OllamaReasoner fallback (Ollama stopped)"
  log "Target: < 100 ms"
  log "========================================================"

  ensure_ollama_running

  # Build and run the Go fallback test against live Ollama then stopped Ollama
  local project_dir="/vagrant-project"
  export PATH=$PATH:/usr/local/go/bin

  if [ ! -f "${project_dir}/go.mod" ]; then
    log "WARN: project not found at ${project_dir} — skipping Go fallback benchmark"
    cat >> "${RESULT_JSON}.parts" <<EOF
{
  "fallback_test": "skipped",
  "reason": "project directory not mounted at ${project_dir}"
}
EOF
    return
  fi

  log "Running Go integration test (OllamaReasoner fallback)..."
  cd "${project_dir}"
  # Run only the fallback latency test; it uses port 19999 (refused) so no live Ollama needed
  local fallback_start_ns
  fallback_start_ns=$(date +%s%N)
  if go test ./internal/skill/... -run "TestOllamaReasoner_FallbackLatencyUnderThreshold" \
      -v -count=1 -timeout 30s > "${RESULTS_DIR}/fallback_test.log" 2>&1; then
    local fallback_ms
    fallback_ms=$(elapsed_ms "${fallback_start_ns}")
    local latency
    latency=$(grep -oP 'fallback latency: \K[0-9.]+µs' "${RESULTS_DIR}/fallback_test.log" | head -1 || echo "unknown")
    log "Go fallback test PASS — measured latency: ${latency} (total test: ${fallback_ms} ms)"
    cat >> "${RESULT_JSON}.parts" <<EOF
{
  "fallback_test": "pass",
  "measured_latency": "${latency}",
  "test_duration_ms": ${fallback_ms}
}
EOF
  else
    log "Go fallback test FAIL — see ${RESULTS_DIR}/fallback_test.log"
    cat >> "${RESULT_JSON}.parts" <<EOF
{
  "fallback_test": "fail",
  "log": "${RESULTS_DIR}/fallback_test.log"
}
EOF
  fi
}

# ─── Main ─────────────────────────────────────────────────────────────────────

TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)
TOTAL_RAM=$(total_ram_mib)
OS_INFO=$(lsb_release -ds 2>/dev/null || cat /etc/os-release | grep PRETTY_NAME | cut -d= -f2 | tr -d '"')
KERNEL=$(uname -r)
CPU=$(grep "model name" /proc/cpuinfo | head -1 | cut -d: -f2 | xargs)
VCPUS=$(nproc)
OLLAMA_VER=$(ollama --version 2>&1 | grep -oP '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "unknown")

log "========================================================"
log "ADR-008 Ollama Benchmark — ${TIMESTAMP}"
log "OS: ${OS_INFO}"
log "Kernel: ${KERNEL}"
log "CPU: ${CPU} (${VCPUS} vCPUs)"
log "Total RAM: ${TOTAL_RAM} MiB"
log "Ollama: ${OLLAMA_VER}"
log "========================================================"

# Reset JSON parts file
rm -f "${RESULT_JSON}.parts"

# Run benchmarks
benchmark_model "${PHI_MODEL}"  "standard" 3072   # 3 GB target
benchmark_model "${QWEN_MODEL}" "edge"     1536   # 1.5 GB target
benchmark_fallback

# Assemble final JSON
{
  echo "{"
  echo "  \"benchmark_timestamp\": \"${TIMESTAMP}\","
  echo "  \"environment\": {"
  echo "    \"os\": \"${OS_INFO}\","
  echo "    \"kernel\": \"${KERNEL}\","
  echo "    \"cpu\": \"${CPU}\","
  echo "    \"vcpus\": ${VCPUS},"
  echo "    \"total_ram_mib\": ${TOTAL_RAM},"
  echo "    \"ollama_version\": \"${OLLAMA_VER}\""
  echo "  },"
  echo "  \"adr_008_targets\": {"
  echo "    \"phi_ram_mib\": 3072,"
  echo "    \"qwen_ram_mib\": 1536,"
  echo "    \"inference_p50_ms\": 5000,"
  echo "    \"fallback_ms\": 100"
  echo "  },"
  echo "  \"results\": ["
  # Join parts with commas
  local_parts=()
  while IFS= read -r line; do
    local_parts+=("${line}")
  done < "${RESULT_JSON}.parts"
  printf '%s\n' "${local_parts[@]}" | awk 'BEGIN{first=1} /^\{/{if(!first)printf ",\n"; first=0} {print}'
  echo "  ]"
  echo "}"
} > "${RESULT_JSON}"

rm -f "${RESULT_JSON}.parts"

log "========================================================"
log "Benchmark complete. Results written to:"
log "  JSON: ${RESULT_JSON}"
log "  Log:  ${LOG}"
log "========================================================"

# Print summary
log ""
log "=== SUMMARY ==="
jq -r '.results[] | "[\(.profile // .fallback_test)] \(.model // "fallback"): RAM=\(.ram_measured_mib // "n/a")MiB (target \(.ram_target_mib // "n/a")MiB) ram_pass=\(.ram_pass // "n/a") p50=\(.warm_inference_p50_ms // "n/a")ms latency_pass=\(.inference_latency_pass // "n/a")"' \
  "${RESULT_JSON}" 2>/dev/null | tee -a "${LOG}" || true
