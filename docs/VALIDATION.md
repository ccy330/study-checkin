# 本机验收记录

日期：2026-10-29（北京时间）。

- Go 1.27.2 编译成功，`go vet ./...` 通过。
- Docker MySQL 8.0 独立测试库集成测试通过：注册/重复用户名、正确与错误密码、全局鉴权、跨用户读写隔离、计划 CRUD、每日重复打卡拦截、统计计算、级联删除、退出失效与 Token 过期。
- 单元测试通过：计划边界校验、连续天数、未登录业务接口、额外 JSON 拒绝。
- HBuilderX 4.87 Vue3 编译器完成 H5 正式资源构建；浏览器验证注册、新建计划、打卡、统计，控制台无错误。
- `docker build -t study-checkin:dev ./bankend` 实际构建成功。
- Docker 后端容器映射本机 18080，连接已有 Docker MySQL；`/health` 返回正常，未鉴权业务接口返回 401。

尚待完成：Android APK 云打包、APK 安装验证与 APK Release 上传。HBuilderX 打包命令当前返回 `user not login`，需要项目所有者登录 DCloud 并为本应用获取 AppID。H5 资源包不能替代 APK。
