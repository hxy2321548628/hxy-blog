# Git 与提交规范

## 分支

- 主分支：`main`，始终保持可部署。
- 禁止直接在 `main` 开发或推送；所有变更必须从短生命周分支提交 Pull Request。
- Pull Request 必须通过 GitHub Actions 的 `check` 状态检查后才能合并。
- 短生命周分支：`feat/<topic>`、`fix/<topic>`、`chore/<topic>`。
- 每个分支只对应一个故事或 Bug，优先在 1–2 天内合并。

## 提交信息

使用 Conventional Commits：

```text
<type>(<scope>): <summary>
```

`type` 可用 `feat`、`fix`、`docs`、`test`、`refactor`、`build`、`ci`、`chore`。`summary` 用中文，不加句号。

示例：

```text
feat(post): 新增文章草稿创建接口
fix(auth): 拒绝过期的管理员会话
```

## 提交前

1. 检查 `git diff`，移除无关变更。
2. 运行 `task check`。
3. 确认无 `.env`、凭据、个人数据和大型生成物。
4. 提交信息只描述已完成的变更。

## 开发步骤与分支清理

- 每完成一个可独立验收的开发步骤，必须在检查通过后创建一次独立的 Git 提交，不得将多个已完成步骤积压到同一个提交中。
- 开发分支合并进入 `main` 后，必须及时删除已合并且不再承载独立工作的本地和远程分支；删除前应核对合并状态，并保留 `main` 及仍在开发的分支。

AI 不得未经用户要求执行 `push`、强制推送、重写历史或合并主分支。
