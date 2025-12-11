#!/bin/bash

# Target host (Moxa or Raspberry Pi)
TARGET_IP="192.168.1.14"

# Output filename
OUT_FILE="captura_${TARGET_IP}_60s.pcap"

# Temporary path (root always can write to /tmp)
TMP_PATH="/tmp/${OUT_FILE}"

echo "[INFO] Starting 60-second capture for host ${TARGET_IP}..."
echo "[INFO] Temporary file: ${TMP_PATH}"

# Run tshark capture
sudo tshark \
    -a duration:60 \
    -f "host ${TARGET_IP}" \
    -w "${TMP_PATH}"

# Check if capture was created
if [ ! -f "${TMP_PATH}" ]; then
    echo "[ERROR] Capture file was not created. Something went wrong."
    exit 1
fi

# Move file to current directory
echo "[INFO] Moving capture to current directory: $(pwd)"
sudo mv "${TMP_PATH}" .

# Fix ownership (optional, nice for editing/viewing)
if [ -f "${OUT_FILE}" ]; then
    sudo chown "$USER":"$USER" "${OUT_FILE}"
fi

echo "[OK] Capture completed: ${OUT_FILE}"
