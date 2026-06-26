#!/bin/bash
# Claude Code 状态栏 — 实时显示上下文容量 (方块进度条风格)
# 使用 python3 解析 stdin JSON（Windows 兼容）

input=$(cat 2>/dev/null)

if [ -z "$input" ] || [ "$input" = "{}" ] || [ "$input" = "null" ]; then
  echo "📊 等待会话数据..."
  exit 0
fi

PYTHONIOENCODING=utf-8 python -c "
import json, sys
try:
    d = json.loads(sys.stdin.read())
except:
    print('📊 解析中...')
    sys.exit(0)

model = d.get('model') or d.get('current_model', '')
used = d.get('tokens_used') or d.get('token_count') or d.get('usage')
total = d.get('context_window') or d.get('max_tokens') or d.get('tokens_total')
pct_val = d.get('context_usage_pct') or d.get('usage_pct')

parts = []
if model:
    # 短名称
    short = model.replace('claude-','').replace('opus','Op').replace('sonnet','So').replace('haiku','Ha').replace('fable','Fa').replace('[1m]','').strip()
    parts.append(f'🧠 {short}')

# 方块进度条 (10格，每格10%，四舍五入)
def bar(pct):
    filled = min(10, max(0, int((pct + 5) / 10)))
    if filled <= 0:
        filled = 1
    return chr(0x25B0)*filled + chr(0x25B1)*(10-filled)

if used is not None and total is not None and total > 0:
    pct = int(used * 100 / total)
    icon = '🔴' if pct >= 90 else '🟡' if pct >= 70 else '🟢' if pct >= 50 else '🔵'
    parts.append(f'{icon} {bar(pct)} {pct}%')
elif pct_val is not None:
    pct = int(pct_val)
    icon = '🔴' if pct >= 90 else '🟡' if pct >= 70 else '🟢' if pct >= 50 else '🔵'
    parts.append(f'{icon} {bar(pct)} {pct}%')

if parts:
    print(' | '.join(parts))
else:
    print('📊 状态栏活跃 | /context 查看详情')
" <<< "$input"
