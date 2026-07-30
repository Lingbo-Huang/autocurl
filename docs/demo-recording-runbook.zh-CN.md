# Autocurl 演示视频：人工点击录制脚本

这份文档是录制时直接照着执行的短脚本。完整的场景说明、字幕建议和验收标准见
[`demo-videos.zh-CN.md`](demo-videos.zh-CN.md)。

所有视频只使用仓库中的公开 `examples`。不打开公司项目，不出现内部域名、
真实 Token、Cookie、IP、人员名称或业务数据。

## 协作方式

- Codex 只负责录制前准备：打开公开示例、选择配置、清空历史请求、关闭通知、
  预热依赖并完成一次不录屏的验证。
- 录制开始后由你完成全部点击。Codex 不再控制鼠标键盘，避免自动操作延迟被录入。
- 每段开始前，Codex 必须明确回复“已准备好”，并说明当前窗口、起始画面和预期结果。
- 如果出现弹窗、报错或等待超过 10 秒，立即停止并丢弃本段；不要把排查过程录进去。
- 每段第一帧和结果帧各停留约 2 秒，停止后剪掉 PixPin 开始与结束操作。

## 推荐顺序

1. JetBrains Java POST：完整 Body 和 Copy cURL
2. VS Code Node.js Debug：自动捕获
3. 请求尚未发送：JSON 离线生成 cURL
4. mTLS、etcd、证书固定：Bypass capture
5. 自定义 HTTP Client：忽略环境代理与显式代理
6. PyCharm 非标准 Run Configuration：为什么不支持
7. Cursor 已运行进程：为什么必须重新 Debug

---

## 01：JetBrains Java POST

文件名：`01-jetbrains-java-post.mp4`

目标时长：20–30 秒。

Codex 录制前准备：

- IDEA 打开 `examples/java-client/AutocurlExample.java`；
- 选择标准 `Application` 配置 `AutocurlExample`；
- 关闭“开始前显示运行配置设置”；
- 打开右侧 Autocurl，清空请求；
- 让 `DEFAULT_BODY` 和空请求列表同时可见。

你录制时的点击脚本：

1. 开始录制，停留 2 秒。
2. 点击 **Run Selected**。
3. 等 GET、POST 两行出现。
4. 点击 POST。
5. 停留 3 秒，展示 `--http2`、脱敏 Header 和 `--data-binary`。
6. 点击 **Copy cURL**。
7. 停留 2 秒，停止录制。

验收：POST 的嵌套 `items`、`address`、`metadata` 可见；Authorization 和 API Key
显示 `[REDACTED]`。

---

## 02：VS Code Node.js Debug 自动捕获

文件名：`02-vscode-node-debug-autocapture.mp4`

目标时长：15–25 秒。

Codex 录制前准备：

- VS Code 打开 `examples/node-client/client.js`；
- 选择 `Autocurl demo: Node.js`；
- 清空 Explorer → Autocurl Requests；
- 隐藏 Debug Console、通知和辅助侧栏；
- 预检 Node.js Debug 能在 10 秒内完成。

你录制时的点击脚本：

1. 开始录制，停留 2 秒。
2. 点击左侧 **Run and Debug**。
3. 确认配置是 `Autocurl demo: Node.js`，点击绿色三角形或按 `F5`。
4. 点击左侧 **Explorer**。
5. 等待 `GET example.com/ 200` 出现。
6. 点击该请求，展示自动打开的完整 cURL。
7. 点击请求右侧 **Copy cURL**。
8. 等 `Autocurl #1 copied` 出现，停留 2 秒后停止。

验收：录制前没有运行 `autocurl run`；Debug 启动后请求自动出现。

---

## 03：JSON 离线生成 cURL

文件名：`03-breakpoint-json-to-curl.mp4`

目标时长：12–20 秒。

Codex 录制前准备：

- VS Code 打开 `examples/request-json/request.json`；
- Autocurl Requests 保持为空；
- 隐藏终端和通知；
- 预检离线生成命令能打开 cURL 并复制到剪贴板。

你录制时的点击脚本：

1. 开始录制，停留 JSON 画面 2 秒。
2. 按 `Cmd+A` 选中整个请求 JSON。
3. 按 `Cmd+Shift+P`。
4. 执行 **Autocurl: Generate cURL from Request JSON**。
5. 展示生成结果中的 `POST`、`--http2`、Header 和 `--data-binary`。
6. 等 `Generated cURL copied to clipboard.` 出现，停留 2 秒后停止。

验收：整个过程不启动 Debug、不发送网络请求，左侧请求列表保持为空。

---

## 04：Bypass capture

文件名：`04-bypass-capture.mp4`

目标时长：15–25 秒。

只使用以下保留示例值：

```text
auth.demo.invalid
.infra.demo.invalid
192.0.2.10
198.51.100.0/24
```

Codex 录制前准备：

- IDEA 打开 **Settings → Tools → Autocurl**；
- Capture mode 设为 `safe`；
- Bypass capture 输入框为空；
- 将上面的四行示例放入剪贴板。

你录制时的点击脚本：

1. 开始录制，停留设置页 2 秒。
2. 点击 **Bypass capture** 输入框，按 `Cmd+V`。
3. 点击 **Apply**。
4. 停留 3 秒，停止录制。

必须配字幕或说明：

- Bypass 流量保持端到端直连，因此不会生成 cURL；
- 其他目标仍可正常捕获；
- 不要把真实客户端私钥交给代理。

录制后由 Codex 删除四个示例值。

---

## 05：自定义 HTTP Client 显式代理

文件名：`05-custom-client-explicit-proxy.mp4`

目标时长：25–40 秒。

Codex 录制前准备：

- GoLand 打开 `examples/go-custom-http-client/main.go`；
- 创建并预检两个标准 Go 配置：
  - `ignore-env`
  - `explicit-env-proxy`
- 打开 Autocurl 并清空请求。

你录制时的点击脚本：

1. 开始录制。
2. 选择 `ignore-env`，点击 **Run Selected**。
3. 展示“请求成功，但 Autocurl 请求列表为空”。
4. 选择 `explicit-env-proxy`，点击 **Run Selected**。
5. 等 POST 出现，点击 POST。
6. 展示 `--data-binary` 和完整 JSON，停留 2 秒后停止。

必须说明：自定义 Transport 的 `Proxy:nil` 会忽略环境代理；显式读取本次进程的
代理 URL 后即可捕获，不需要修改系统代理。

---

## 06：PyCharm 非标准 Run Configuration

文件名：`06-pycharm-standard-run-configuration.mp4`

目标时长：20–30 秒。

Codex 录制前准备：

- PyCharm 打开 `examples/python-client/client.py`；
- 保留非标准 Compound 配置 `Unsupported Compound`；
- 保留标准 Python 配置 `Autocurl Python Demo`；
- 打开 Autocurl 工具窗口并清空请求；
- 确认当前 Python 配置使用公开目标 `http://example.com/`。

你录制时的点击脚本：

1. 开始录制，在 `client.py` 和空的 Autocurl 请求列表上停留 2 秒。
2. 在顶部 Run Configuration 下拉框选择 `Unsupported Compound`。
3. 在 Autocurl 工具窗口点击 **Run Selected**。
4. 展示提示
   `This Run Configuration type does not expose environment variables`，
   停留约 2 秒后点击 **OK**。
5. 在顶部下拉框切换为标准 Python 配置 `Autocurl Python Demo`。
6. 在 Autocurl 工具窗口再次点击 **Run Selected**。
7. 等 `GET example.com/ 200` 出现，停留 2 秒后停止录制。

必须配字幕或说明：

- Autocurl 必须在进程启动前写入临时代理和 CA 环境；
- Compound 等不暴露环境变量的配置无法安全注入；
- 标准 Python、Java、Go、Node.js、Gradle 等常用配置已支持。

验收：非标准配置先出现明确提示；切换为 `Autocurl Python Demo` 后请求正常出现，
全程不修改系统代理。

---

## 07：Cursor 已运行进程必须重新 Debug

文件名：`07-cursor-restart-under-autocurl.mp4`

目标时长：25–35 秒。

Codex 录制前准备：

- Cursor 打开 `examples/go-http-client/main.go`；
- Explorer → **Autocurl Requests** 已展开且请求列表为空；
- 窗口底部状态栏显示已选配置
  `Autocurl demo: Go (restart required)`；
- 集成终端可用，普通运行命令为：

  ```bash
  go run ./examples/go-http-client -repeat 10 -delay 2s
  ```

- 普通终端运行和 Cursor Debug 自动捕获均已完成预检。

你录制时的点击脚本：

1. 开始录制，在 `main.go` 和空的 **Autocurl Requests** 上停留 2 秒。
2. 打开 Cursor 集成终端，输入并执行：

   ```bash
   go run ./examples/go-http-client -repeat 10 -delay 2s
   ```

3. 等终端出现请求成功日志；展示 **Autocurl Requests** 仍为空。
4. 在终端按 `Ctrl+C`，停止这个已经运行的普通进程。
5. 确认窗口底部状态栏显示配置
   `Autocurl demo: Go (restart required)`。
6. 点击 macOS 顶部菜单栏 **Run → Start Debugging**，由 Cursor 重新启动
   进程。不要找绿色三角形，也不使用 `F5`；不需要先执行 `autocurl run`，
   也不需要手动点击 **Start Capture**。
7. 点击左侧 **Explorer**，等待 GET、POST 请求出现。
8. 点击 `POST example.com/ 405`，展示生成 cURL 中的 `--data-binary`
   和完整 JSON Body。
9. 停留 2 秒后停止录制。

必须配字幕或说明：

- 已经运行的进程无法事后补写 `HTTP_PROXY` 和临时 CA；
- 必须由 Cursor 通过 Autocurl 重新 Debug，插件才会在启动前注入临时环境；
- 无法重启的线上进程应使用离线 JSON 或现有可观测性工具。

验收：终端普通运行期间请求列表始终为空；执行
**Run → Start Debugging** 后出现 GET 200 和 POST 405，POST cURL 包含完整 Body。

---

## 每段保存后

1. 使用本文给定的文件名。
2. 回放一次，剪掉开始、结束和超过 2 秒的无操作等待。
3. 确认没有内部信息或桌面通知。
4. 单段尽量小于 3 MB。
5. 告诉 Codex“保存好了”，由 Codex配置下一段。
