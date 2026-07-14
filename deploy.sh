#!/bin/bash
# ============================================================
#  GinChat 一键部署脚本
#  用法:
#    bash deploy.sh                  # 完整部署（后端+前端）
#    bash deploy.sh backend          # 只部署后端
#    bash deploy.sh frontend         # 只部署前端
#    bash deploy.sh dryrun           # 只构建不上传
#
#    bash deploy.sh list             # 列出服务器所有版本
#    bash deploy.sh status           # 查看当前运行版本
#    bash deploy.sh rollback <ver>   # 回滚到指定版本
#    bash deploy.sh cleanup [keep]   # 清理旧版本（默认保留10个）
# ============================================================

set -e

# ==================== 配置 ====================
SERVER_IP="YOUR_SERVER_IP"
SERVER_USER="admin"
SSH_PORT="22"
REMOTE_DIR="/www/wwwroot/ginchat-server"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$SCRIPT_DIR/.deploy"

# 确保在项目根目录
cd "$SCRIPT_DIR"

# 自动查找 Go（处理 Git Bash 下 PATH 不完整的情况）
if ! command -v go &>/dev/null; then
    for found in "$HOME/sdk/go"*/bin /c/Users/*/sdk/go*/bin /d/GoLang/bin; do
        if [ -x "$found/go" ] || [ -x "$found/go.exe" ]; then
            export PATH="$found:$PATH"
            break
        fi
    done
fi

# 检查必要命令
for cmd in go npm ssh scp; do
    if ! command -v $cmd &>/dev/null; then
        echo "[ERROR] $cmd not found in PATH"
        exit 1
    fi
done

# ==================== 帮助 ====================
show_help() {
    echo ""
    echo "GinChat Deploy Script"
    echo "====================="
    echo ""
    echo "Deploy commands:"
    echo "  bash deploy.sh                Full deploy (backend + frontend)"
    echo "  bash deploy.sh backend        Backend only"
    echo "  bash deploy.sh frontend       Frontend only"
    echo "  bash deploy.sh dryrun         Build only, no upload"
    echo ""
    echo "Management commands:"
    echo "  bash deploy.sh list           List all deployed versions"
    echo "  bash deploy.sh status         Show current version + health"
    echo "  bash deploy.sh rollback <v>   Rollback to a version"
    echo "  bash deploy.sh cleanup [N]    Remove old releases (keep N)"
    echo ""
}

# ==================== 远程命令 ====================
# 管理命令（list/status/rollback/cleanup）直接通过 SSH 执行
run_remote() {
    scp -q -P "$SSH_PORT" -o StrictHostKeyChecking=no "$SCRIPT_DIR/deploy-remote.sh" "${SERVER_USER}@${SERVER_IP}:/tmp/deploy-remote.sh"
    ssh -p "$SSH_PORT" -o StrictHostKeyChecking=no "$SERVER_USER@$SERVER_IP" "sudo bash /tmp/deploy-remote.sh $*"
}

# ==================== 参数解析 ====================
MODE="${1:-help}"

# 管理命令（不需要本地构建）
case "$MODE" in
    list|status|cleanup)
        run_remote "$@"
        exit 0
        ;;
    rollback)
        if [ -z "${2:-}" ]; then
            echo "Usage: bash deploy.sh rollback <version>"
            echo "Run 'bash deploy.sh list' to see versions."
            exit 1
        fi
        echo ""
        echo "WARNING: This will restart the service on $SERVER_IP."
        printf "Rollback to version '%s'? [y/N] " "${2}"
        read -r confirm
        if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
            echo "Cancelled."
            exit 0
        fi
        run_remote "$@"
        exit 0
        ;;
    help|-h|--help)
        show_help
        exit 0
        ;;
    full|backend|frontend|dryrun)
        ;;
    *)
        show_help
        exit 1
        ;;
esac

DEPLOY_BACKEND="NO"
DEPLOY_FRONTEND="NO"
DRYRUN="NO"

case "$MODE" in
    full)     DEPLOY_BACKEND="YES"; DEPLOY_FRONTEND="YES" ;;
    backend)  DEPLOY_BACKEND="YES" ;;
    frontend) DEPLOY_FRONTEND="YES" ;;
    dryrun)   DEPLOY_BACKEND="YES"; DEPLOY_FRONTEND="YES"; DRYRUN="YES" ;;
esac

TIMESTAMP=$(date +%Y%m%d-%H%M%S)
GIT_HASH=$(git -C "$SCRIPT_DIR" rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION_TAG="${TIMESTAMP}-${GIT_HASH}"
PACKAGE_NAME="ginchat-deploy-${VERSION_TAG}.tar.gz"

echo ""
echo "===================================================="
echo "       GinChat IM Deploy Script"
echo "===================================================="
echo "  Version   : $VERSION_TAG"
echo "  Target    : ${SERVER_USER}@${SERVER_IP}"
echo "  Backend   : $DEPLOY_BACKEND"
echo "  Frontend  : $DEPLOY_FRONTEND"
echo "===================================================="
echo ""

mkdir -p "$DEPLOY_DIR"

# ==================== Step 1: 构建后端 ====================
if [ "$DEPLOY_BACKEND" = "YES" ]; then
    echo "--- [1/5] Build Go Backend ---"

    echo "  Cross-compiling Linux amd64 binary..."
    cd "$SCRIPT_DIR"
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
        go build -ldflags="-s -w -X main.Version=$VERSION_TAG" -o "$DEPLOY_DIR/ginchat_server" .
    echo "  [OK] Build done ($(du -h "$DEPLOY_DIR/ginchat_server" | cut -f1))"
fi

# ==================== Step 2: 构建前端 ====================
if [ "$DEPLOY_FRONTEND" = "YES" ]; then
    echo "--- [2/5] Build Frontend ---"

    cd "$SCRIPT_DIR/frontend"
    [ -d "node_modules" ] || npm install
    npm run build
    cd "$SCRIPT_DIR"

    rm -rf "$DEPLOY_DIR/frontend/dist"
    mkdir -p "$DEPLOY_DIR/frontend"
    cp -r frontend/dist "$DEPLOY_DIR/frontend/dist"
    echo "  [OK] Frontend build done"
fi

# ==================== Step 3: 打包 ====================
echo "--- [3/5] Package ---"

cat > "$DEPLOY_DIR/DEPLOY_VERSION.txt" << EOF
Version: $VERSION_TAG
Time: $(date '+%Y-%m-%d %H:%M:%S')
Backend: $DEPLOY_BACKEND
Frontend: $DEPLOY_FRONTEND
Git: $GIT_HASH
EOF

cd "$DEPLOY_DIR"
tar -czf "$SCRIPT_DIR/$PACKAGE_NAME" *
cd "$SCRIPT_DIR"
echo "  [OK] Packaged: $PACKAGE_NAME"

# ==================== Step 4: 干跑 ====================
if [ "$DRYRUN" = "YES" ]; then
    echo ""
    echo "--- Dry Run Mode ---"
    echo "  Backend : $DEPLOY_DIR/ginchat_server"
    echo "  Frontend: $DEPLOY_DIR/frontend/dist/"
    echo "  Done."
    rm -f "$SCRIPT_DIR/$PACKAGE_NAME"
    exit 0
fi

# ==================== Step 5: 上传 ====================
echo "--- [4/5] Upload to Server ---"
echo "  Uploading $PACKAGE_NAME ..."
scp -P "$SSH_PORT" -o StrictHostKeyChecking=no "$PACKAGE_NAME" "${SERVER_USER}@${SERVER_IP}:/tmp/$PACKAGE_NAME"
echo "  [OK] Upload done"

# ==================== Step 6: 远程部署 ====================
echo "--- [5/5] Remote Deploy ---"

# 上传最新的 deploy-remote.sh 并通过它执行部署
scp -q -P "$SSH_PORT" -o StrictHostKeyChecking=no "$SCRIPT_DIR/deploy-remote.sh" "${SERVER_USER}@${SERVER_IP}:/tmp/deploy-remote.sh"
ssh -p "$SSH_PORT" -o StrictHostKeyChecking=no "$SERVER_USER@$SERVER_IP" \
    "sudo bash /tmp/deploy-remote.sh deploy $VERSION_TAG /tmp/$PACKAGE_NAME $DEPLOY_BACKEND $DEPLOY_FRONTEND"

# ==================== 清理 ====================
rm -f "$SCRIPT_DIR/$PACKAGE_NAME"

echo ""
echo "===================================================="
echo "       Deploy Success!"
echo "===================================================="
echo "  Version : $VERSION_TAG"
echo "  URL     : http://$SERVER_IP/"
echo "  Panel   : https://$SERVER_IP:8888/"
echo "===================================================="
echo ""
echo "  Tip: bash deploy.sh list    # see all versions"
echo "       bash deploy.sh status  # check current status"
echo "       bash deploy.sh rollback <v>  # rollback"
