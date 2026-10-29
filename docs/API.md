# HTTP API

统一前缀 `/api`，JSON 请求与响应。注册、登录之外均需 `Authorization: Bearer <token>`；未知业务路径也先鉴权。错误响应 `{"error":"提示"}`。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | /register | username、password；成功返回 token、expires_at、username |
| POST | /login | 同上 |
| POST | /logout | 当前 Token 立即失效 |
| GET | /plans | 当前用户全部计划及今日打卡状态 |
| POST | /plans | 创建计划 |
| GET | /plans/{id} | 计划详情 |
| PUT | /plans/{id} | 编辑计划 |
| DELETE | /plans/{id} | 删除计划及其打卡历史 |
| POST | /checkins | plan_id、minutes（1–1440）、note（最多 1000 字） |
| GET | /checkins?plan_id=1 | 最近 500 条历史，可按计划筛选 |
| GET | /stats | 累计次数、时长、连续天数、完成率、30 天趋势、各计划进度 |
| GET | /health | 公开健康检查；数据库不可用返回 503 |

计划字段：`name`（1–100 字）、`description`（最多 2000 字）、`cycle`（daily / weekly）、`target`（整个计划目标打卡次数，1–10000）、`start_date`、`end_date`（YYYY-MM-DD，跨度最多 3660 天）。周期为计划组织方式；每日每计划最多打卡一次，不自动重置总目标。

注册用户名 3–32 字符，密码 8–72 字节。密码使用 bcrypt；Token 为密码学随机 256 位不透明字符串，数据库仅存 SHA-256 摘要，有效期 7 天，过期返回 401，客户端清除会话并显示登录页面。

北京时间为统一业务日期；客户端不能指定或补打过去日期。连续打卡按有任一打卡记录的自然日去重，当天尚未打卡时从昨天计算。计划完成率为达到目标次数的计划数除以全部计划数；单计划进度上限 100%。删除计划会重新计算统计。

状态码：400 参数错误，401 未登录/过期，404 不存在或不属于当前用户，409 用户名冲突/重复打卡/编辑日期排除已有记录，500 服务异常。生产环境通过 HTTPS 反向代理访问，并在入口设置注册登录限流。
