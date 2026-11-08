# Skill: 给本仓库添加 GitHub Pages 在线工具

## 背景
本仓库 Pages 使用 **分支部署**：`main` 分支 `/docs` 目录，`build_type=legacy`。
没有 Actions 部署 job，推送 `docs/` 变更即自动构建。

## 步骤
1. 在 `docs/tools/` 新增自包含 HTML；公共样式/脚本放
   `docs/tools/common.css`、`docs/tools/common.js`。
2. 页面内用相对路径：
   - 引用公共资源：`common.css`、`common.js`
   - 返回首页：`../index.html`
   - 顶部导航保持四个工具一致
3. 更新 `docs/index.html` 与 `docs/index.md` 的「在线工具」入口。
4. 本地校验（不启动服务器也能做）：
   ```bash
   node --check docs/tools/common.js
   # 提取 <script> 内联代码后 node --check（见 .memories/03-在线工具部署.md）
   ```
5. 提交并推送：
   ```bash
   git add docs && git commit -m "feat(pages): ..."
   TOKEN=$(head -1 "我的Github令牌-千万不要上传.txt")
   B64=$(printf 'x-access-token:%s' "$TOKEN" | base64 -w0)
   git -c http.extraHeader="Authorization: Basic $B64" push origin main
   ```
6. 等构建完成并验证：
   ```bash
   TOKEN=$(head -1 "我的Github令牌-千万不要上传.txt")
   curl -s -H "Authorization: Bearer $TOKEN" \
     https://api.github.com/repos/XiaoYuyu520520/Copy-Paste-Mutools/pages/builds/latest
   curl -s -o /dev/null -w "%{http_code}" \
     "https://xiaoyuyu520520.github.io/Copy-Paste-Mutools/tools/<name>.html?v=<commit>"
   ```

## 外部库
- 优先下载并内置到 `docs/tools/`（如 `qrcode.js`），避免运行时依赖 CDN。
- `qrcode-generator` 默认按 Latin-1 编码，中文必须先设置：
  `qrcode.stringToBytes = qrcode.stringToBytesFuncs["UTF-8"]`

## 坑
- 刚 push 后立即访问可能 404：等 `pages/builds/latest.status == built`
- 目录 `docs/tools/` 无 index 时访问目录路径 404 正常
- 新页面 URL 含中文目录需 URL 编码；工具目录用英文名更稳