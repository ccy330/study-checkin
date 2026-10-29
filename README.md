# 拾光自习 · 个人学习打卡

HBuilderX uni-app Vue3 + Go 单体 + MySQL。仅包含个人端鉴权、学习计划、打卡、统计，不包含后台管理或社交。

## 目录

`page/` 前端，对应 `E:\massage\page`；`bankend/` Go API，对应 `E:\massage\bankend`，按要求保留 bankend 拼写。二者位于同一个仓库根目录。

## Docker 启动

1. 复制 `.env.example` 为 `.env`，替换两个密码。不要提交 `.env`。
2. 在仓库根目录执行 `docker compose up -d --build`。
3. 打开 `http://localhost:8080/health` 验证；手机服务地址填电脑局域网 IP 或部署域名。

Compose 使用独立 MySQL 数据卷，不占用宿主机 3306 端口。生产环境配置 HTTPS 反向代理、登录限流、数据库备份，CORS_ORIGIN 只允许实际 H5 域名。原生 APP 不受浏览器 CORS 约束。

已有 Docker MySQL 可先创建独立库和最小权限用户，然后设置 `MYSQL_DSN=study:密码@tcp(127.0.0.1:3306)/study_checkin?charset=utf8mb4`，在 bankend 执行 `go run .`。服务启动自动建表；DSN 不可使用无密码 root。容器部署时 host 使用数据库容器在同一网络中的名称。

单独构建：`docker build -t study-checkin:1.0.0 ./bankend`。运行时传入 MYSQL_DSN，使用 `-p 8080:8080` 并将容器连接到 MySQL 所在 Docker 网络。

## HBuilderX 与 APK

1. HBuilderX 导入 `page` 文件夹，使用 Vue3 编译器；运行到浏览器可本地预览。
2. 登录自己的 DCloud 账号，在 manifest.json 为本应用获取独立 AppID，填写 Android 包名和签名证书。
3. 发行 → 原生 App 云打包 → Android。生成的 APK 位于 `page/unpackage/release/apk/`；具体以 HBuilderX 打包结果为准。uni-app CLI 的 H5 或 App 资源编译产物不等于 APK。
4. APP 首次登录前设置后端地址。正式包使用 HTTPS 后端。开发时若使用 HTTP，须在 Android 打包配置允许明文访问；不要将该设置用于正式环境。

## 验证

在 bankend 执行 `go test ./...`、`go vet ./...`。提供专用 `TEST_MYSQL_DSN` 后执行 `go test -count=1 -v ./...` 会启用真实 MySQL 集成测试：注册、密码校验、未登录阻断、跨用户隔离、计划 CRUD、重复打卡、统计、级联删除、退出及过期。测试只清理自身随机创建的用户。

## Git 与发布

按 `feat`、`test`、`docs`、`build` 分步提交；主分支 main。仅在验证并生成 APK 后创建版本 tag：`git tag -a v1.0.0 -m "Release v1.0.0"`，然后 `git push origin main --tags`。使用 GitHub CLI 登录后执行 `./scripts/publish-release.ps1 -Repository 用户名/仓库名 -Apk APK完整路径`，脚本校验真实 APK 并上传至 Release。

详细接口与统计口径见 [docs/API.md](docs/API.md)。不提交数据库密码、DCloud 登录凭据、Token 或签名证书。
