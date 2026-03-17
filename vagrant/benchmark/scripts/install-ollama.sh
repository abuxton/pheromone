#!/usr/bin/env bash
# install-ollama.sh — Install Ollama and pull ADR-008 benchmark models on Ubuntu 24.04
#
# Models pulled:
#   phi3.5:3.8b-mini-instruct-q4_K_M  — ADR-008 standard profile (~3 GB RAM)
#   qwen2.5:1.5b-instruct-q4_K_M      — ADR-008 edge profile (~1.5 GB RAM)
set -euo pipefail

PHI_MODEL="phi3.5:3.8b-mini-instruct-q4_K_M"
QWEN_MODEL="qwen2.5:1.5b-instruct-q4_K_M"

echo "[ollama] Installing Ollama..."

if command -v ollama &>/dev/null; then
  echo "[ollama] Ollama already installed: $(ollama --version 2>&1 | head -1)"
else
  curl -fsSL https://ollama.com/install.sh | sh
  echo "[ollama] Ollama installed: $(ollama --version 2>&1 | head -1)"
fi

# Ensure Ollama service is running
if ! systemctl is-active --quiet ollama 2>/dev/null; then
  echo "[ollama] Starting Ollama service..."
  systemctl enable ollama
  systemctl start ollama
  sleep 5
fi

# Wait for Ollama API to be ready (up to 30 s)
echo "[ollama] Waiting for Ollama API..."
for i in $(seq 1 30); do
  if curl -sf http://localhost:11434/ >/dev/null 2>&1; then
    echo "[ollama] API ready after ${i}s."
    break
  fi
  sleep 1
done

# Pull models
echo "[ollama] Pulling ${PHI_MODEL} (standard profile, ~3 GB)..."
ollama pull "${PHI_MODEL}"

echo "[ollama] Pulling ${QWEN_MODEL} (edge profile, ~1.5 GB)..."
ollama pull "${QWEN_MODEL}"

echo "[ollama] Models available:"
ollama list
