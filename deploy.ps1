# ============================================================
#  GinChat 一键部署脚本
#  用法:
#    .\deploy.ps1              # 完整部署
#    .\deploy.ps1 -Backend     # 只部署后端
#    .\deploy.ps1 -Frontend    # 只部署前端
#    .\deploy.ps1 -DryRun      # 只构建不上传
# ============================================================

param(
    [switch]$Backend,
    [switch]$Frontend,
    [switch]$DryRun
)

# ==================== 配置 ====================
$SERVER_IP   = "YOUR_SERVER_IP"
$SERVER_USER = "admin"
$SSH_PORT    = "22"
$REMOTE_DIR  = "/www/wwwroot/ginchat-server"

# 默认完整部署
if (-not $Backend -and -not $Frontend) {
    $DeployBackend  = $true
    $DeployFrontend = $true
} else {
    $DeployBackend  = $Backend
    $DeployFrontend = $Frontend
}

$ErrorActionPreference = "Stop"
$SCRIPT_DIR = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $SCRIPT_DIR

$TIMESTAMP    = Get-Date -Format "yyyyMMdd-HHmmss"
$GIT_HASH     = try { (git rev-parse --short HEAD 2>$null).Trim() } catch { "unknown" }
if (-not $GIT_HASH) { $GIT_HASH = "unknown" }
$VERSION_TAG  = "$TIMESTAMP-$GIT_HASH"
$DEPLOY_DIR   = Join-Path $SCRIPT_DIR ".deploy"
$PACKAGE_NAME = "ginchat-deploy-$VERSION_TAG.zip"

Write-Host ""
Write-Host "====================================================" -ForegroundColor Cyan
Write-Host "        GinChat IM Deploy Script" -ForegroundColor Cyan
Write-Host "====================================================" -ForegroundColor Cyan
Write-Host "  Version : $VERSION_TAG" -ForegroundColor Cyan
Write-Host "  Target  : $SERVER_USER@$SERVER_IP" -ForegroundColor Cyan
$backendLabel = if ($DeployBackend) { "YES" } else { "NO" }
$frontendLabel = if ($DeployFrontend) { "YES" } else { "NO" }
Write-Host "  Backend : $backendLabel    Frontend: $frontendLabel" -ForegroundColor Cyan
Write-Host "====================================================" -ForegroundColor Cyan
Write-Host ""

# 创建部署目录
if (-not (Test-Path $DEPLOY_DIR)) {
    New-Item -ItemType Directory -Force -Path $DEPLOY_DIR | Out-Null
}

# ==================== Step 1: 构建后端 ====================
if ($DeployBackend) {
    Write-Host "--- [1/5] Build Go Backend ---" -ForegroundColor Yellow

    Write-Host "  Cross-compiling Linux amd64 binary..."
    $outputFile = Join-Path $DEPLOY_DIR "ginchat_server"

    # 用 bash 执行 Go 编译（避免 PowerShell 引号转义问题）
    bash -c "cd `"$SCRIPT_DIR`" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags='-s -w -X main.Version=$VERSION_TAG' -o `"$outputFile`" ."
    if ($LASTEXITCODE -ne 0) {
        Write-Host "  [FAIL] Build failed!" -ForegroundColor Red
        exit 1
    }

    $fileSize = [math]::Round((Get-Item $outputFile).Length / 1MB, 2)
    Write-Host "  [OK] Build done ($fileSize MB)" -ForegroundColor Green
}

# ==================== Step 2: 构建前端 ====================
if ($DeployFrontend) {
    Write-Host "--- [2/5] Build Frontend ---" -ForegroundColor Yellow

    $frontendPath = Join-Path $SCRIPT_DIR "frontend"
    Set-Location $frontendPath

    if (-not (Test-Path "node_modules")) {
        Write-Host "  Installing dependencies..."
        npm install 2>&1 | Out-Null
    }

    Write-Host "  Building production bundle..."
    npm run build 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) {
        Write-Host "  [FAIL] Frontend build failed!" -ForegroundColor Red
        Set-Location $SCRIPT_DIR
        exit 1
    }

    Set-Location $SCRIPT_DIR

    # 复制前端 dist
    $distTarget = Join-Path $DEPLOY_DIR "frontend\dist"
    if (Test-Path $distTarget) {
        Remove-Item -Recurse -Force $distTarget
    }
    New-Item -ItemType Directory -Force -Path (Join-Path $DEPLOY_DIR "frontend") | Out-Null
    Copy-Item -Recurse "frontend\dist" $distTarget
    Write-Host "  [OK] Frontend build done" -ForegroundColor Green
}

# ==================== Step 3: 打包 ====================
Write-Host "--- [3/5] Package ---" -ForegroundColor Yellow

$backendLabel2 = if ($DeployBackend) { "YES" } else { "NO" }
$frontendLabel2 = if ($DeployFrontend) { "YES" } else { "NO" }
$manifest = @"
Version: $VERSION_TAG
Time: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')
Backend: $backendLabel2
Frontend: $frontendLabel2
"@
$manifestPath = Join-Path $DEPLOY_DIR "DEPLOY_VERSION.txt"
$manifest | Out-File -FilePath $manifestPath -Encoding UTF8

# 打包为 zip
$packagePath = Join-Path $SCRIPT_DIR $PACKAGE_NAME
if (Test-Path $packagePath) { Remove-Item $packagePath }
Compress-Archive -Path "$DEPLOY_DIR\*" -DestinationPath $packagePath -CompressionLevel Optimal
Write-Host "  [OK] Packaged: $PACKAGE_NAME" -ForegroundColor Green

# ==================== Step 4: 干跑模式 ====================
if ($DryRun) {
    Write-Host ""
    Write-Host "--- Dry Run Mode ---" -ForegroundColor Cyan
    Write-Host "  Backend : $DEPLOY_DIR\ginchat_server" -ForegroundColor Cyan
    Write-Host "  Frontend: $DEPLOY_DIR\frontend\dist\" -ForegroundColor Cyan
    Write-Host "  No upload. Done." -ForegroundColor Cyan
    exit 0
}

# ==================== Step 5: 上传 ====================
Write-Host "--- [4/5] Upload to Server ---" -ForegroundColor Yellow
Write-Host "  Uploading $PACKAGE_NAME ..."

$scpTarget = "${SERVER_USER}@${SERVER_IP}:/tmp/$PACKAGE_NAME"
scp -P $SSH_PORT -o StrictHostKeyChecking=no $packagePath $scpTarget
if ($LASTEXITCODE -ne 0) {
    Write-Host "  [FAIL] Upload failed!" -ForegroundColor Red
    exit 1
}
Write-Host "  [OK] Upload done" -ForegroundColor Green

# ==================== Step 6: 远程部署 ====================
Write-Host "--- [5/5] Remote Deploy ---" -ForegroundColor Yellow

# 生成 shell 脚本，保存为文件再执行
$remoteShPath = Join-Path $DEPLOY_DIR "remote-deploy.sh"
$remoteSh = @'
#!/bin/bash
set -e

REMOTE_DIR="__REMOTE_DIR__"
BACKUP_DIR="$REMOTE_DIR/backups"
PACKAGE="/tmp/__PACKAGE_NAME__"
VERSION_TAG="__VERSION_TAG__"
SERVER_IP="__SERVER_IP__"
DEPLOY_BACKEND="__DEPLOY_BACKEND__"
DEPLOY_FRONTEND="__DEPLOY_FRONTEND__"

echo "  Creating backup dir..."
sudo mkdir -p "$BACKUP_DIR"

# 备份旧版本
if [ -f "$REMOTE_DIR/ginchat_server" ]; then
    echo "  Backing up old version..."
    sudo cp "$REMOTE_DIR/ginchat_server" "$BACKUP_DIR/ginchat_server.bak-$VERSION_TAG"
    echo "  [OK] Backup done"
fi

# 停止服务
echo "  Stopping backend..."
sudo pkill ginchat_server 2>/dev/null || echo "    (service not running)"
sleep 1

# 解压
echo "  Extracting package..."
sudo mkdir -p /tmp/ginchat-deploy
sudo unzip -o "$PACKAGE" -d /tmp/ginchat-deploy/ > /dev/null

# 找到解压目录
cd /tmp/ginchat-deploy
SUB_DIR=$(ls -d */ | head -1)
cd "$SUB_DIR"

# 安装后端
if [ "$DEPLOY_BACKEND" = "YES" ]; then
    echo "  Installing backend..."
    sudo cp ginchat_server "$REMOTE_DIR/"
    sudo chmod +x "$REMOTE_DIR/ginchat_server"
    echo "  [OK] Backend installed"
fi

# 安装前端
if [ "$DEPLOY_FRONTEND" = "YES" ]; then
    echo "  Installing frontend..."
    sudo rm -rf "$REMOTE_DIR/frontend/dist"
    sudo cp -r frontend/dist "$REMOTE_DIR/frontend/dist"
    sudo chown -R www:www "$REMOTE_DIR/frontend/dist"
    echo "  [OK] Frontend installed"
fi

# 清理
echo "  Cleaning up..."
sudo rm -rf /tmp/ginchat-deploy
sudo rm -f "$PACKAGE"

# 启动服务
echo "  Starting backend..."
cd "$REMOTE_DIR"
sudo nohup ./ginchat_server > /dev/null 2>&1 &
sleep 2

# 验证
if pgrep -f ginchat_server > /dev/null; then
    echo "  [OK] Service started"
else
    echo "  [FAIL] Service failed to start!"
    exit 1
fi

# 健康检查
echo "  Health check..."
sleep 1
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:8080/index 2>/dev/null || echo "000")
if [ "$HTTP_CODE" = "200" ]; then
    echo "  [OK] Health check passed (HTTP $HTTP_CODE)"
else
    echo "  [WARN] Health check: HTTP $HTTP_CODE"
fi

echo ""
echo "===================================================="
echo "       Deploy Complete!"
echo "       Version : $VERSION_TAG"
echo "       URL     : http://$SERVER_IP/"
echo "===================================================="
'@

# 替换占位符
$remoteSh = $remoteSh.Replace('__REMOTE_DIR__', $REMOTE_DIR)
$remoteSh = $remoteSh.Replace('__PACKAGE_NAME__', $PACKAGE_NAME)
$remoteSh = $remoteSh.Replace('__VERSION_TAG__', $VERSION_TAG)
$remoteSh = $remoteSh.Replace('__SERVER_IP__', $SERVER_IP)
$remoteSh = $remoteSh.Replace('__DEPLOY_BACKEND__', $backendLabel)
$remoteSh = $remoteSh.Replace('__DEPLOY_FRONTEND__', $frontendLabel)

# 写文件 (LF 换行)
$remoteSh -replace "`r`n", "`n" | Out-File -FilePath $remoteShPath -Encoding ASCII -NoNewline

Write-Host "  Executing remote deploy..."
Get-Content $remoteShPath -Raw | ssh -p $SSH_PORT -o StrictHostKeyChecking=no "$SERVER_USER@$SERVER_IP" "bash -s"

if ($LASTEXITCODE -eq 0) {
    Write-Host ""
    Write-Host "====================================================" -ForegroundColor Green
    Write-Host "       Deploy Success!" -ForegroundColor Green
    Write-Host "====================================================" -ForegroundColor Green
    Write-Host "  Version : $VERSION_TAG" -ForegroundColor Green
    Write-Host "  URL     : http://$SERVER_IP/" -ForegroundColor Green
    Write-Host "  Panel   : https://$SERVER_IP:8888/" -ForegroundColor Green
    Write-Host "====================================================" -ForegroundColor Green
} else {
    Write-Host "[FAIL] Deploy error!" -ForegroundColor Red
}

# 清理
Remove-Item $packagePath -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "Rollback command (run on server):" -ForegroundColor Yellow
Write-Host "  sudo cp $REMOTE_DIR/backups/ginchat_server.bak-$VERSION_TAG $REMOTE_DIR/ginchat_server" -ForegroundColor Yellow
Write-Host "  sudo pkill ginchat_server && cd $REMOTE_DIR && sudo nohup ./ginchat_server > /dev/null 2>&1 &" -ForegroundColor Yellow
