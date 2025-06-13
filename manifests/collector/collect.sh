#!/bin/bash
set -u

NSENTER="nsenter --mount=/proc/1/ns/mnt --"
TMP_OUTPUT=$(mktemp)
MAX_TOTAL_CHARS=80000   # ≈ 20k tokens
MAX_SECTION_LINES=200   # 每类日志最多保留 N 行
DEFAULT_TIMEOUT=10      # 每个命令最长执行秒数

write_section() {
  local name="$1"; shift
  local cmd=("$@")

  echo "[$name]" >> "$TMP_OUTPUT"

  if output=$(timeout "$DEFAULT_TIMEOUT" "${cmd[@]}" 2>/dev/null | tail -n "$MAX_SECTION_LINES"); then
    if [[ -n "$output" ]]; then
      echo "$output" | while read -r line; do
        echo "- $line" >> "$TMP_OUTPUT"
      done
    else
      echo "- <no data>" >> "$TMP_OUTPUT"
    fi
  else
    echo "- [ERROR] ${cmd[*]} failed or timed out" >> "$TMP_OUTPUT"
  fi

  echo "" >> "$TMP_OUTPUT"
}

cmd_exists() {
  $NSENTER bash -c "command -v $1" >/dev/null 2>&1
}

# 1. Kernel 日志
if cmd_exists dmesg; then
  write_section "kernel" $NSENTER dmesg -T | grep -Ei 'panic|BUG|hung|OOM|fail|error|warn'
else
  echo "[kernel]" >> "$TMP_OUTPUT"
  echo "- [SKIPPED] dmesg not found" >> "$TMP_OUTPUT"
  echo "" >> "$TMP_OUTPUT"
fi

# 2. GPFS 健康状态
if $NSENTER bash -c 'test -x /usr/lpp/mmfs/bin/mmhealth'; then
  write_section "gpfs.health" $NSENTER /usr/lpp/mmfs/bin/mmhealth node show all
else
  echo "[gpfs.health]" >> "$TMP_OUTPUT"
  echo "- [SKIPPED] mmhealth not found" >> "$TMP_OUTPUT"
  echo "" >> "$TMP_OUTPUT"
fi

# 3. GPFS 日志文件
if $NSENTER bash -c 'test -f /var/adm/ras/mmfs.log.latest'; then
  write_section "gpfs.log" $NSENTER grep -Ei 'error|warn' /var/adm/ras/mmfs.log.latest
else
  echo "[gpfs.log]" >> "$TMP_OUTPUT"
  echo "- [SKIPPED] mmfs.log.latest not found" >> "$TMP_OUTPUT"
  echo "" >> "$TMP_OUTPUT"
fi

# 输出控制
if [[ $(wc -c < "$TMP_OUTPUT") -gt $MAX_TOTAL_CHARS ]]; then
  echo "[截断提示] 总日志超出限制，仅保留前 $MAX_TOTAL_CHARS 字节" >> "$TMP_OUTPUT"
  head -c "$MAX_TOTAL_CHARS" "$TMP_OUTPUT"
else
  cat "$TMP_OUTPUT"
fi

rm -f "$TMP_OUTPUT"
