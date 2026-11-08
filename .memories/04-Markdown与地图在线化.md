# Markdown 预览 / 2D 激光地图在线化（2026-11-02）

## 结果
提交 `3fd0d4f`（工具）+ `aeed76b`（README 入口）已推送，Pages 构建成功。

- Markdown：https://xiaoyuyu520520.github.io/Copy-Paste-Mutools/tools/markdown.html
- 地图：https://xiaoyuyu520520.github.io/Copy-Paste-Mutools/tools/mapview.html

## 做法：复用桌面版资产，去掉服务端
桌面版页面在 `internal/render/mdeditor.go` / `mapview.go` 里是 Go 原始字符串 +
内嵌 JSON + 本地 HTTP 服务（SSE、/api/map 上传）。网页化时：

1. 静态资产直接复制 `internal/render/assets/` → `docs/tools/assets/`
   （marked.min.js、highlight.min.js、github-markdown-light/dark、github/gihub-dark CSS），
   与桌面版保持同源，零额外依赖。
2. Markdown 页 `markdown.html`：
   - 删除 SSE `/api/events`、`/api/doc`、INIT_DOC、地图 iframe 逻辑
   - 保留：拖入/粘贴、220ms 去抖渲染、hljs 高亮、代码块复制按钮、
     亮暗主题（显式互斥两套 CSS）、全屏/左右对照（Ctrl+E/D/O）、打印
   - 新增轻量 sanitize（template + 遍历属性）：去 script/iframe/on* 属性/
     javascript: 链接，保留 GitHub 渲染效果
3. 地图页 `mapview.html`：
   - 用 JS 实现 PGM 解码（P5/P2、8/16 位、注释容忍；`tokenize` 逻辑照抄 Go 版）
     与 map_server YAML 容错解析（resolution/origin/negate/thresholds/raw 保留）
   - 功能对齐桌面版：比例尺（niceM）、测距、橡皮（negate!=0 → 擦成 0，否则 255）、
     导出 P5 PGM + 替换 image 行的 YAML
   - 改进：mapCv 只在尺寸变化时重置，窗口缩放不丢擦除结果（桌面版会丢）
   - 文件来源：input/drag&drop，全部本地 FileReader
4. 五个工具页顶部导航统一（Base64/二维码/编解码检测/Markdown/激光地图）；
   README 增加「在线工具」章节 + 文档导航行 + merger 功能两条内链。

## 验证
- 全部 5 页内联脚本 `node --check` 通过
- 从 mapview.html 抽出 decodePGM/parseYAML 在 Node 里实测：
  P5（含数据偏移一个空白）、16 位大端、P2 比例缩放、YAML origin 均正确
- 引用完整性脚本：5 页 href/src 本地相对路径全部存在
- 线上 curl 200 + `<title>` 比对（等待 Pages `built`，CDN 加 `?v=<commit>`）

## 注意
- markdown.html 不处理 .pgm/.yaml（跨页面传文件需要 IndexedDB，未做），
  拖入时 toast 提示去地图工具
- P5 且 maxv≠255 时按原样拷贝像素（与 Go 版行为一致，未做比例缩放）
- `docs/tools/` 目录本身无 index.html，访问目录路径 404 属正常