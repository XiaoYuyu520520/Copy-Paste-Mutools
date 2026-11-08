# WebRTC 硬件编码检测（2026-11-02）

## 结果
提交 `e169f2d`：`feat(pages/codec): 增加 WebRTC 实时通信与硬件编码能力检测`
codec.html 新增「WebRTC 实时通信」表格，覆盖 VP8 / VP9 / H.264 / AV1。

## 检测方法（浏览器能给出的最接近答案）
1. **编解码列表**
   - `RTCRtpSender.getCapabilities("video")`
   - `RTCRtpReceiver.getCapabilities("video")`
   - 取 `codecs[].mimeType`，转大写后匹配 `VP8` / `VP9` / `H264` / `AV1`
2. **WebRTC 编码能力 + 硬件迹象**
   - `navigator.mediaCapabilities.encodingInfo({ type:"webrtc", video:{...} })`
   - 先带 `hardwareAcceleration:"prefer-hardware"` 调用；若抛错（部分实现不接受该字段）
     再退回不带该字段的调用
   - 返回 `{supported, smooth, powerEfficient}`
   - `powerEfficient === true` 通常表示硬件 / 低功耗编码路径；`false` 只能说明“可能软编”
3. 配置示例：`contentType: video/VP8|VP9|H264|AV1`，1280×720，2 Mbps，30 fps

## 重要结论 / 限制
- 浏览器**不暴露**具体硬件编码器名称（如 NVENC / VideoToolbox / MediaCodec），
  只能通过 `powerEfficient` / `smooth` 推断，不能当作绝对结论
- `MediaCapabilities.encodingInfo` 需要安全上下文（HTTPS / localhost）
- 某些浏览器/版本没有 `encodingInfo` 或 `type:"webrtc"` 支持 → 显示“无 API/未知”
- `hardwareAcceleration` 字段在不同实现的接受位置不同，必须做 fallback 容错
- 实际是否硬编还取决于系统、驱动、分辨率、码率与通话参数

## 代码位置
`docs/tools/codec.html`
- `RTC_CODECS`：检测清单
- `rtcCodecSet` / `rtcMatch`：收发能力匹配
- `mediaEncodeInfo`：MediaCapabilities（含 prefer-hardware fallback）
- `testRTC` / `renderRTC`：表格渲染
- 报告导出新增 `[WebRTC 视频]` 段

## 验证
- 内联脚本 `node --check` 通过
- Node mock 测试：getCapabilities 匹配、prefer-hardware 回退、软件路径、
  配置异常 → `supported:null`、无 API 均符合预期
- 同步更新 docs/index.html、docs/index.md、README 描述