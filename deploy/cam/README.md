# COS 备份策略说明

[`cos-backup-policy.json`](./cos-backup-policy.json) 是腾讯云 CAM 要求的标准 JSON，JSON 语法不允许注释，因此在本文件解释每个字段。修改策略后需运行 `make check` 验证 JSON 结构。

## 字段含义

- `version: 2.0`：腾讯云 CAM 策略语法版本，不是本项目版本。
- `statement`：权限声明数组；当前只有一条允许规则。
- `effect: allow`：只授予明确列出的操作，其他操作默认拒绝。
- `resource`：权限只覆盖南京地域、指定备份桶的 `mysql/*` 对象，不授予其他桶或前缀。

`action` 分为三组：

- 普通对象：`PutObject`、`HeadObject`、`GetObject`，用于上传、存在性检查和回读验证。
- 分片上传：创建任务、列举当前任务/分片、上传分片、完成或中止分片，供较大备份使用。
- 没有授予：`DeleteObject`、`DeleteBucket`、桶配置和账户级权限。服务器凭据泄露时，攻击者不能用该身份删除远端备份。

## 使用边界

该策略只关联到无控制台登录权限的专用 CAM 子用户 `hxy-blog-backup`。SecretId/SecretKey 只存放在服务器 root 可读的 COSCLI 配置中，不写入本策略、代码库或 GitHub Secrets。
