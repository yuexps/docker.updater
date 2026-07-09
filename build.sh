#!/bin/bash
set -e

echo "=================================================="
echo "[INFO] 开始打包构建 Docker Updater"
echo "=================================================="

# 1. 构建 Vue 3 前端静态资产
echo "[INFO] 正在构建 Vue 3 前端静态资产..."
cd frontend
npm run build
cd ..

# 2. 交叉编译 Linux x86_64 二进制文件
echo "[INFO] 正在交叉编译 Linux x86_64 二进制文件..."
mkdir -p fnpack/app/bin
export GOOS=linux
export GOARCH=amd64
go build -trimpath -ldflags="-s -w" -o fnpack/app/bin/docker-updater

# 3. 调用 fnpack 构建 fpk 安装包
echo "[INFO] 正在调用 fnpack 构建 fpk 安装包..."
cd fnpack
chmod +x fnpack-1.2.3-linux-amd64
./fnpack-1.2.3-linux-amd64 build
cd ..

echo "=================================================="
echo "[SUCCESS] 核心构建与 .fpk 打包已完成！"
echo "=================================================="
