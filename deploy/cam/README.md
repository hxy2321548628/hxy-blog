# COS CAM 策略说明

本目录的策略文件是腾讯云 CAM 要求的标准 JSON，JSON 语法不允许注释，因此在本文件解释权限边界。修改策略后需运行 `make check` 验证 JSON 结构。

## 数据库备份身份

[`cos-backup-policy.json`](./cos-backup-policy.json) 只关联到无控制台登录权限的专用 CAM 子用户 `hxy-blog-backup`：

- `resource`：权限只覆盖南京地域、指定备份桶的 `mysql/*` 对象，不授予其他桶或前缀。

`action` 分为三组：

- 普通对象：`PutObject`、`HeadObject`、`GetObject`，用于上传、存在性检查和回读验证。
- 分片上传：创建任务、列举当前任务/分片、上传分片、完成或中止分片，供较大备份使用。
- 没有授予：`DeleteObject`、`DeleteBucket`、桶配置和账户级权限。服务器凭据泄露时，攻击者不能用该身份删除远端备份。

SecretId/SecretKey 只存放在服务器 root 可读的 COSCLI 配置中，不写入策略、代码库或 GitHub Secrets。

## 博客媒体身份

[`cos-media-policy.json`](./cos-media-policy.json) 只关联到另一个无控制台登录权限的专用 CAM 子用户 `hxy-blog-media`：

- 资源只覆盖南京地域正式媒体桶 `hxy-blog-media-1497610660` 的 `media/*` 对象。
- `PutObject` 用于写入已校验图片，`HeadObject` / `GetObject` 用于受控核验，`DeleteObject` 只用于数据库写入失败后的补偿删除。
- 不授予列举桶、删除桶、修改桶配置、访问备份桶或账户级管理权限。
- 媒体身份的 SecretId/SecretKey 只写入服务器 `/etc/hxy-blog/runtime.env`，权限为 root:root `0600`。

备份和媒体必须使用不同的桶、身份和密钥；不得为了省配置而让应用复用 `hxy-blog-backup` 身份。
隔离测试桶使用独立身份；可复制媒体策略并把 `resource` 精确替换为测试桶的 `media/integration/*`，不得把正式桶加入测试身份。

## 通用字段含义

- `version: 2.0`：腾讯云 CAM 策略语法版本，不是本项目版本。
- `statement`：权限声明数组。
- `effect: allow`：只授予明确列出的操作，其他操作默认拒绝。
- `resource`：将动作限制到指定地域、桶和对象前缀。
