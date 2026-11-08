# Skill: 用 GitHub PAT 访问仓库

## 适用场景
需要拉取/推送私有仓库，或在 CI/本地用细粒度 PAT（`github_pat_...`）操作。

## 步骤
1. 读取令牌（不要写死到代码/日志），例如：
   ```bash
   TOKEN=$(head -1 "path/to/token.txt")
   ```
2. 验证账号与权限（REST 用 Bearer）：
   ```bash
   curl -s -H "Authorization: Bearer $TOKEN" https://api.github.com/user
   curl -s -H "Authorization: Bearer $TOKEN" \
     https://api.github.com/repos/<owner>/<repo>
   ```
   看返回的 `permissions.push` 是否为 `true`。
3. git over HTTPS 必须用 Basic（Bearer 会 `invalid credentials`）：
   ```bash
   B64=$(printf 'x-access-token:%s' "$TOKEN" | base64 -w0)
   git -c http.extraHeader="Authorization: Basic $B64" ls-remote <repo-url>
   ```
   推送前可先 `git push --dry-run <url> HEAD:refs/heads/__probe__` 验证写权限。
4. 确认无误后再正式 `git fetch` / `git push`。

## 注意
- Windows Git Bash 下 `base64 -w0` 可用；若不可用改用无换行变体
- 不要把令牌拼进会回显的命令输出、URL 日志或提交内容
- 细粒度 PAT 无 `X-OAuth-Scopes`，权限看仓库 `permissions` 字段