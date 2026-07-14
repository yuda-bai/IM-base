#!/bin/bash
# ============================================================
#  deploy-remote.sh — 服务器端版本管理
#  通过 deploy.sh 管道传输到服务器执行，也可直接在服务器上运行。
#
#  用法:
#    bash deploy-remote.sh deploy <version> <package> <backend> <frontend>
#    bash deploy-remote.sh list
#    bash deploy-remote.sh status
#    bash deploy-remote.sh rollback <version>
#    bash deploy-remote.sh bootstrap
# ============================================================

set -e

REMOTE_DIR="/www/wwwroot/ginchat-server"
RELEASES_DIR="$REMOTE_DIR/releases"
BACKUP_DIR="$REMOTE_DIR/backups"
VERSIONS_LOG="$REMOTE_DIR/VERSIONS.log"

COMMAND="${1:-}"
SERVER_IP="YOUR_SERVER_IP"

# ==================== 辅助函数 ====================

stop_service() {
    echo "  Stopping backend..."
    sudo pkill ginchat_server 2>/dev/null || echo "    (service not running)"
    sleep 1
}

start_service() {
    echo "  Starting backend..."
    cd "$REMOTE_DIR"
    sudo nohup ./ginchat_server > /dev/null 2>&1 &
    sleep 2

    if pgrep -f ginchat_server > /dev/null; then
        echo "  [OK] Service started"
        return 0
    else
        echo "  [FAIL] Service failed to start!"
        return 1
    fi
}

health_check() {
    echo "  Health check..."
    sleep 1
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:8080/index 2>/dev/null || echo "000")
    if [ "$HTTP_CODE" = "200" ]; then
        echo "  [OK] Health check passed (HTTP $HTTP_CODE)"
    else
        echo "  [WARN] Health check: HTTP $HTTP_CODE"
    fi
}

log_event() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | sudo tee -a "$VERSIONS_LOG" > /dev/null
}

current_version() {
    if [ -L "$REMOTE_DIR/current" ]; then
        basename "$(readlink "$REMOTE_DIR/current")"
    else
        echo "none"
    fi
}

# ==================== bootstrap ====================
do_bootstrap() {
    if [ -L "$REMOTE_DIR/current" ]; then
        echo "  Version management already bootstrapped."
        return 0
    fi

    echo "  Bootstrapping version management..."
    sudo mkdir -p "$RELEASES_DIR"
    sudo mkdir -p "$RELEASES_DIR/initial/frontend"

    # 迁移现有二进制
    if [ -f "$REMOTE_DIR/ginchat_server" ] && [ ! -L "$REMOTE_DIR/ginchat_server" ]; then
        sudo mv "$REMOTE_DIR/ginchat_server" "$RELEASES_DIR/initial/"
        echo "    (migrated existing binary)"
    fi

    # 迁移现有前端
    if [ -d "$REMOTE_DIR/frontend/dist" ] && [ ! -L "$REMOTE_DIR/frontend/dist" ]; then
        sudo mv "$REMOTE_DIR/frontend/dist" "$RELEASES_DIR/initial/frontend/"
        echo "    (migrated existing frontend)"
    fi

    sudo mkdir -p "$REMOTE_DIR/frontend"

    # 建立符号链接
    cd "$REMOTE_DIR"
    sudo ln -sfn "releases/initial" current
    sudo ln -sfn "current/ginchat_server" ginchat_server
    sudo ln -sfn "../current/frontend/dist" frontend/dist

    sudo touch "$VERSIONS_LOG"
    log_event "BOOTSTRAP (migrated existing deployment)"

    echo "  [OK] Bootstrap complete"
}

# ==================== deploy ====================
do_deploy() {
    VERSION="${1:-}"
    PACKAGE="${2:-}"
    DEPLOY_BACKEND="${3:-YES}"
    DEPLOY_FRONTEND="${4:-YES}"

    if [ -z "$VERSION" ] || [ -z "$PACKAGE" ]; then
        echo "ERROR: deploy requires version and package path"
        exit 1
    fi

    # 引导（首次自动迁移）
    do_bootstrap

    RELEASE_DIR="$RELEASES_DIR/$VERSION"

    echo "  Creating release directory..."
    sudo mkdir -p "$RELEASE_DIR"
    sudo mkdir -p "$RELEASE_DIR/frontend"

    # 解压
    echo "  Extracting package..."
    sudo mkdir -p /tmp/ginchat-deploy
    sudo rm -rf /tmp/ginchat-deploy/*
    sudo tar -xzf "$PACKAGE" -C /tmp/ginchat-deploy/
    cd /tmp/ginchat-deploy

    # 安装后端
    if [ "$DEPLOY_BACKEND" = "YES" ]; then
        echo "  Installing backend..."
        if [ -f ginchat_server ]; then
            sudo cp ginchat_server "$RELEASE_DIR/"
            sudo chmod +x "$RELEASE_DIR/ginchat_server"
            echo "  [OK] Backend installed ($(du -h "$RELEASE_DIR/ginchat_server" | cut -f1))"
        else
            echo "  [WARN] ginchat_server not found in package"
        fi
    fi

    # 安装前端
    if [ "$DEPLOY_FRONTEND" = "YES" ]; then
        echo "  Installing frontend..."
        if [ -d "frontend/dist" ]; then
            sudo rm -rf "$RELEASE_DIR/frontend/dist"
            sudo cp -r frontend/dist "$RELEASE_DIR/frontend/dist"
            sudo chown -R www:www "$RELEASE_DIR/frontend/dist"
            echo "  [OK] Frontend installed"
        else
            echo "  [WARN] frontend/dist not found in package"
        fi
    fi

    # 复制元数据
    if [ -f "DEPLOY_VERSION.txt" ]; then
        sudo cp DEPLOY_VERSION.txt "$RELEASE_DIR/"
    fi

    # 清理
    sudo rm -rf /tmp/ginchat-deploy
    sudo rm -f "$PACKAGE"

    # 备份（兼容旧系统）
    sudo mkdir -p "$BACKUP_DIR"
    CURRENT_V=$(current_version)
    if [ "$CURRENT_V" != "none" ] && [ -f "$RELEASES_DIR/$CURRENT_V/ginchat_server" ]; then
        echo "  Backing up current binary..."
        sudo cp "$RELEASES_DIR/$CURRENT_V/ginchat_server" "$BACKUP_DIR/ginchat_server.bak-$VERSION"
        echo "  [OK] Backup done"
    fi

    # 停止 → 切换 → 启动
    if [ "$DEPLOY_BACKEND" = "YES" ]; then
        stop_service

        echo "  Switching to new version..."
        cd "$REMOTE_DIR"
        sudo ln -sfn "releases/$VERSION" current
        echo "  [OK] Switched current -> releases/$VERSION"

        start_service
        health_check
    else
        echo "  (backend not updated, skipping restart)"
    fi

    log_event "DEPLOY $VERSION (backend=$DEPLOY_BACKEND frontend=$DEPLOY_FRONTEND)"

    echo ""
    echo "===================================================="
    echo "       Deploy Complete!"
    echo "       Version : $VERSION"
    echo "       URL     : http://$SERVER_IP/"
    echo "===================================================="
}

# ==================== list ====================
do_list() {
    if [ ! -d "$RELEASES_DIR" ] || [ -z "$(ls -A "$RELEASES_DIR" 2>/dev/null)" ]; then
        echo ""
        echo "No releases found. Run a deploy first."
        exit 0
    fi

    CURRENT_V=$(current_version)

    echo ""
    echo "  Deployed versions:"
    echo "  ----------------------------------------"
    for rel in $(ls -1t "$RELEASES_DIR" 2>/dev/null); do
        if [ "$rel" = "$CURRENT_V" ]; then
            printf "  %-37s <-- current\n" "$rel"
        else
            printf "  %-37s\n" "$rel"
        fi
    done
    echo "  ----------------------------------------"
    echo "  Total: $(ls -1 "$RELEASES_DIR" | wc -l) versions"
    echo ""
}

# ==================== status ====================
do_status() {
    if [ ! -L "$REMOTE_DIR/current" ]; then
        echo ""
        echo "Version management not bootstrapped. Run a deploy first."
        exit 1
    fi

    CURRENT_V=$(current_version)
    echo ""
    echo "  Current version: $CURRENT_V"
    echo "  ----------------------------------------"

    # 发行版元数据
    if [ -f "$REMOTE_DIR/current/DEPLOY_VERSION.txt" ]; then
        while IFS= read -r line; do
            echo "  $line"
        done < "$REMOTE_DIR/current/DEPLOY_VERSION.txt"
    fi

    # 进程状态
    PID=$(pgrep -f "ginchat_server" 2>/dev/null | head -1 || echo "")
    if [ -n "$PID" ]; then
        echo "  Service: running (PID $PID)"
    else
        echo "  Service: NOT running"
    fi

    # 健康检查
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:8080/index 2>/dev/null || echo "000")
    if [ "$HTTP_CODE" = "200" ]; then
        echo "  Health:  OK (HTTP $HTTP_CODE)"
    else
        echo "  Health:  WARN (HTTP $HTTP_CODE)"
    fi

    # 磁盘使用
    if [ -d "$RELEASES_DIR" ]; then
        TOTAL_SIZE=$(du -sh "$RELEASES_DIR" 2>/dev/null | cut -f1)
        echo "  Releases disk: $TOTAL_SIZE"
    fi

    echo "  ----------------------------------------"
    echo ""
}

# ==================== rollback ====================
do_rollback() {
    TARGET="${1:-}"

    if [ -z "$TARGET" ]; then
        echo "ERROR: specify a version to rollback to."
        echo "Usage: bash deploy-remote.sh rollback <version>"
        echo "Run 'list' to see available versions."
        exit 1
    fi

    TARGET_DIR="$RELEASES_DIR/$TARGET"

    if [ ! -d "$TARGET_DIR" ]; then
        echo "ERROR: version '$TARGET' not found."
        echo "Available versions:"
        ls -1 "$RELEASES_DIR" 2>/dev/null || echo "  (none)"
        exit 1
    fi

    if [ ! -f "$TARGET_DIR/ginchat_server" ]; then
        echo "ERROR: no binary in $TARGET_DIR"
        exit 1
    fi

    CURRENT_V=$(current_version)
    if [ "$TARGET" = "$CURRENT_V" ]; then
        echo "Already running version $TARGET. Nothing to do."
        exit 0
    fi

    echo ""
    echo "  Rolling back to: $TARGET"
    echo "  ----------------------------------------"

    # 先备份当前版本
    sudo mkdir -p "$BACKUP_DIR"
    if [ "$CURRENT_V" != "none" ] && [ -f "$RELEASES_DIR/$CURRENT_V/ginchat_server" ]; then
        sudo cp "$RELEASES_DIR/$CURRENT_V/ginchat_server" "$BACKUP_DIR/ginchat_server.bak-before-rollback-to-$TARGET"
    fi

    stop_service

    echo "  Switching current -> releases/$TARGET ..."
    cd "$REMOTE_DIR"
    sudo ln -sfn "releases/$TARGET" current
    echo "  [OK] Switched"

    start_service || {
        echo ""
        echo "  [FAIL] Rollback failed to start service!"
        echo "  The 'current' symlink has been changed to releases/$TARGET"
        echo "  Manual recovery: ssh to server and check logs"
        exit 1
    }

    health_check

    log_event "ROLLBACK from $CURRENT_V to $TARGET"

    echo "  ----------------------------------------"
    echo "  Rollback complete."
    echo "  Now running: $TARGET"
    echo ""
}

# ==================== cleanup ====================
do_cleanup() {
    KEEP="${1:-10}"
    CURRENT_V=$(current_version)

    echo ""
    echo "  Cleaning old releases (keeping latest $KEEP)..."
    echo "  Current: $CURRENT_V"
    echo "  ----------------------------------------"

    COUNT=0
    for rel in $(ls -1t "$RELEASES_DIR" 2>/dev/null); do
        COUNT=$((COUNT + 1))
        if [ $COUNT -gt $KEEP ] && [ "$rel" != "$CURRENT_V" ]; then
            echo "  Removing: $rel"
            sudo rm -rf "$RELEASES_DIR/$rel"
        fi
    done

    log_event "CLEANUP kept=$KEEP"
    echo "  [OK] Cleanup done"
    echo ""
}

# ==================== 调度 ====================
case "$COMMAND" in
    deploy)
        do_deploy "${2:-}" "${3:-}" "${4:-YES}" "${5:-YES}"
        ;;
    list)
        do_list
        ;;
    status)
        do_status
        ;;
    rollback)
        do_rollback "${2:-}"
        ;;
    bootstrap)
        do_bootstrap
        ;;
    cleanup)
        do_cleanup "${2:-10}"
        ;;
    *)
        echo ""
        echo "Usage: deploy-remote.sh <command> [args...]"
        echo ""
        echo "Deploy:"
        echo "  deploy <ver> <pkg> [backend] [frontend]   Deploy new version"
        echo ""
        echo "Management:"
        echo "  list                                       List all versions"
        echo "  status                                     Show current version"
        echo "  rollback <version>                         Rollback to version"
        echo "  bootstrap                                  Init version mgmt"
        echo "  cleanup [keep=N]                           Remove old releases"
        echo ""
        exit 1
        ;;
esac
