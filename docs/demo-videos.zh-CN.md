# Autocurl 演示视频录制指南

录制时需要直接照着点击的精简脚本见
[`demo-recording-runbook.zh-CN.md`](demo-recording-runbook.zh-CN.md)。
本文保留场景设计、字幕和验收标准。

这组视频只使用仓库公开的 `examples`，不打开公司项目，不出现内部域名、
Token、Cookie、IP、人员名称或业务数据。

## 统一录制规格

- 工具：PixPin 3.3.5.7 或更高版本。
- 画面：只录目标 IDE / 终端窗口，推荐 1280×720 或 1440×810。
- 格式：MP4，24 FPS，无麦克风和系统声音。
- 时长：核心演示 30–45 秒，边界场景 15–30 秒。
- 光标：开启点击提示；没有 PixPin 会员也不影响录制。
- 隐私：录制前关闭通知、聊天、邮件、公司项目和密码管理器。
- 文件：输出到 `site/assets/demos/`，使用下文给定的英文文件名。

### 只录一个窗口

最稳的手动路径：

1. 把目标 IDEA、GoLand、VS Code、Cursor 或终端窗口置顶。
2. 按 `Ctrl+1` 进入 PixPin 截图选择。
3. 把鼠标移到目标窗口内部，等待选区吸附整个窗口。
4. 不要先完成截图，直接按 `G` 切换到录屏。
5. 点击录制按钮；结束时再按 `Ctrl+1`。
6. 回放中剪掉开头、结尾的等待，导出 MP4。

也可以在 **PixPin → 配置 → 快捷键/动作 → 添加新动作** 中创建：

```javascript
pixpin.gifScreenShot(
  pixpin.getSpRect(PixConst.SpRectWindowUnderMouse)
)
```

动作名填 `录制鼠标所在窗口`，勾选“添加到托盘菜单”，再绑定一个不冲突的
全局快捷键，例如 `Ctrl+Option+R`。录制前把鼠标停在目标 IDE 窗口里，再按
这个快捷键。

PixPin 录的是窗口所在矩形中的屏幕像素。录制期间不要让其他窗口盖住目标窗口；
建议开启系统“勿扰模式”，并把录制窗口放到一个干净的桌面空间。

## 01：JetBrains Java POST、完整 Body 与 Copy cURL

输出：`01-jetbrains-java-post.mp4`，20–30 秒。

准备：

1. IDEA 打开 `examples/java-client` Maven 项目。
2. 普通运行 `AutocurlExample.main` 一次，生成标准 **Application**
   Run Configuration。
3. 打开右侧 **Autocurl** 工具窗口，点击 **Clear**。
4. 代码停在 `DEFAULT_BODY` 附近，让嵌套 JSON 可见。

镜头：

| 时间 | 操作 | 字幕 |
| --- | --- | --- |
| 0–4 秒 | 展示 Java 代码和空请求列表 | “不用改业务代码” |
| 4–9 秒 | 点击 **Run Selected** | “只给本次运行注入临时代理和 CA” |
| 9–18 秒 | 等待 GET、POST 两行出现 | “HTTP/2 请求已捕获” |
| 18–31 秒 | 点击 POST，慢慢滚动右侧 cURL | “Header、Query、嵌套 Body 都在” |
| 31–38 秒 | 点击 **Copy cURL**，粘贴到临时文本页 | “一键复制，可直接重放” |

验收：

- 控制台包含 `protocol=HTTP_2`；
- POST cURL 包含 `--http2` 和 `--data-binary`；
- `user_id`、`items`、`address`、`metadata` 均可见；
- Authorization 和 API Key 默认显示 `[REDACTED]`；
- 不能出现“普通 Run 也会自动捕获”的误导。

## 02：VS Code / Cursor Debug 自动捕获

输出：`02-vscode-node-debug-autocapture.mp4`，15–25 秒。

准备：

1. VS Code 或 Cursor 打开仓库根目录。
2. 安装 Autocurl VSIX 与对应语言扩展。
3. 确认：

```json
{
  "autocurl.autoStartOnDebug": true,
  "autocurl.injectDebugEnvironment": true
}
```

镜头：

| 时间 | 操作 | 字幕 |
| --- | --- | --- |
| 0–5 秒 | 打开 Run and Debug，选 `Autocurl demo: Node.js` | “正常按 F5，不用单独输入命令” |
| 5–14 秒 | 启动 Debug | “插件在进程启动前自动准备捕获环境” |
| 14–24 秒 | 展开 Explorer → Autocurl Requests | “请求自动出现” |
| 24–34 秒 | 点击 GET，展示并复制完整 cURL | “完整请求，可查看、可复制” |

Cursor 的录制步骤相同。公开视频只需选择一个编辑器实录，片尾加字幕：
“VS Code 与 Cursor 使用同一个 VSIX”。

验收：

- 没有先运行 `autocurl run`；
- 请求列表中有 `GET example.com/ 200`；
- 打开的 Shell 文档包含 URL、协议和 Header；
- 说明捕获必须发生在 Debug 进程启动之前。

## 03：断点处的 JSON 和 CLI 离线生成

输出：`03-breakpoint-json-to-curl.mp4`，12–20 秒。

使用公开文件 `docs/generate-curl-from-json.zh-CN.md` 中的 Generic JSON，
或新建一个临时 JSON 编辑器页：

```json
{
  "method": "POST",
  "url": "https://api.example.com/orders",
  "protocol": "HTTP/2",
  "headers": {
    "Content-Type": "application/json",
    "get-info": "true"
  },
  "body": {
    "user_id": 10086,
    "items": [{"sku": "demo-001", "quantity": 2}]
  }
}
```

IDE 镜头：

1. 展示“请求还没发出”的断点或 JSON 页。
2. 选中 JSON。
3. JetBrains 点击 **Generate cURL from JSON**；VS Code / Cursor 从命令面板
   执行 **Autocurl: Generate cURL from Request JSON**。
4. 展示生成结果和“已复制”提示。

CLI 镜头：

```bash
./build/local/autocurl render --file /tmp/autocurl-request.json
```

字幕：“不发网络请求，也能把断点里的请求对象变成 cURL”。

验收：

- 生成过程中没有实际 HTTP 请求；
- cURL 含 HTTP 方法、URL、Header 和嵌套 Body；
- 不要声称调试器能自动读取任意语言对象的私有字段。

## 04：mTLS、etcd、证书固定和直连服务的 Bypass

输出：`04-bypass-capture.mp4`，15–25 秒。

公开视频只使用保留示例地址：

```text
auth.demo.invalid
.infra.demo.invalid
192.0.2.10
198.51.100.0/24
```

JetBrains 镜头：

1. 打开 **Settings → Tools → Autocurl**。
2. 保持 Capture mode 为 `safe`。
3. 在 **Bypass capture** 中逐行填入上面的示例。
4. 点击 **Apply**。
5. 回到工具窗口重新启动示例。

VS Code / Cursor 对应配置：

```json
{
  "autocurl.captureMode": "safe",
  "autocurl.bypassTargets": [
    "auth.demo.invalid",
    ".infra.demo.invalid",
    "192.0.2.10",
    "198.51.100.0/24"
  ]
}
```

片中字幕必须写清：

- “Bypass 流量保持端到端直连，因此不会生成 cURL”；
- “其他目标仍可正常捕获”；
- “不要导出或交给代理真实客户端私钥”。

录制结束后删除这些示例值，避免影响后续视频。

## 05：自定义 HTTP Client 忽略环境代理

输出：`05-custom-client-explicit-proxy.mp4`，25–40 秒。

使用 `examples/go-custom-http-client`，创建两个标准 Go
Run Configuration：

```text
-proxy-mode ignore-env
```

和：

```text
-proxy-mode explicit-env-proxy
```

镜头：

1. 用 Autocurl 运行 `ignore-env`。
2. 控制台显示请求成功，但请求列表为空；等待
   `no_proxy_traffic_observed` 诊断。
3. 切换到 `explicit-env-proxy` 并重新 Run Selected。
4. 点击捕获到的 POST，展示完整 JSON Body。

字幕：

- “请求成功 ≠ 请求经过了捕获代理”；
- “自定义 Transport 的 `Proxy:nil` 会完全忽略 HTTP_PROXY/HTTPS_PROXY”；
- “显式读取本次进程的代理 URL 后即可捕获，不用改系统代理”。

## 06：非标准 Run Configuration

输出：`06-standard-run-configuration.mp4`，20–30 秒。

使用公开 `examples/java-client`：

1. 新建一个 **Compound** 或其他不暴露环境变量的临时配置。
2. 选中它，点击 **Run Selected**。
3. 展示提示：

```text
This Run Configuration type does not expose standard environment variables.
Java, Go, Python, Node.js, Gradle, and most common configurations are supported.
```

4. 切换到标准 **Application** 配置 `AutocurlExample`。
5. 再点击 **Run Selected**，展示请求出现。

字幕：“Autocurl 必须在进程启动前写入临时环境；选择标准语言配置即可。”

不要把“所有 Gradle 配置都不支持”录进视频；Gradle 和大多数常用配置本身已支持。

## 07：已经运行的进程不能事后接管

输出：`07-restart-under-autocurl.mp4`，25–35 秒。

使用 `examples/go-http-client`，程序参数：

```text
-repeat 10 -delay 2s
```

镜头：

1. 先点 IDE 普通 Run，让程序已经运行。
2. 打开 Autocurl，展示请求列表仍为空。
3. 停止普通 Run。
4. 点击 **Run Selected** 或 **Debug Selected** 重新启动。
5. 请求开始出现，选择 POST 查看 Body。

字幕：

- “已经运行的进程无法事后补写 HTTP_PROXY / CA”；
- “必须通过 Autocurl 重新 Run/Debug”；
- “线上或无法重启的进程，请使用离线 JSON 生成或现有可观测性工具”。

## 导出后的检查

逐个播放并确认：

- 没有内部域名、真实 Token、真实 Cookie、聊天通知或个人信息；
- 所有代码来自本仓库 `examples`；
- 字幕没有把 Bypass 描述成“仍能捕获”；
- `Copy cURL` 前确实选中了 POST；
- 视频不依赖声音也能看懂；
- 第一帧和结束帧至少停留 1 秒；
- 单个 MP4 尽量小于 3 MB，总体尽量小于 15 MB。

最后运行：

```bash
node scripts/check-site.mjs
```

官网接入视频后，还要分别在桌面和手机尺寸下检查自动播放策略、Poster、字幕和
加载失败时的文字说明。
