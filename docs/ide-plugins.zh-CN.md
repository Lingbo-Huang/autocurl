# IDE 插件使用、打包与发布

Autocurl 采用“一套 Go 核心、两个薄插件”的结构：

```mermaid
flowchart LR
    V["VS Code / Cursor 插件"] --> P["autocurl proxy JSONL 会话"]
    J["JetBrains 插件"] --> P
    P --> E["仅注入当前调试进程的环境"]
    E --> A["Go / Python / Java / Node 服务"]
    A --> C["请求事件 + 完整 cURL"]
    C --> V
    C --> J
```

HTTP/2、gRPC、WebSocket、TLS、脱敏和 cURL 生成都在同一个 Go 引擎中实现。
IDE 插件只负责开始/停止、注入调试环境、展示请求和复制。

## VS Code / Cursor

安装 `autocurl-0.2.2.vsix` 后，默认直接按 F5：

1. 插件自动启动后台捕获会话。
2. 在调试程序启动前注入临时代理和证书环境。
3. 请求出现在 **Explorer → Autocurl Requests**。
4. 点击请求查看 cURL，点击右侧按钮复制。
5. 停止捕获后，临时代理和 CA 自动删除。

常用命令：

- **Autocurl: Start Capture**
- **Autocurl: Stop Capture**
- **Autocurl: Copy Last cURL**
- **Autocurl: Render Selected Request JSON**
- **Autocurl: Download/Update Engine**

常用配置：

| 配置 | 默认值 | 作用 |
| --- | --- | --- |
| `autocurl.autoStartOnDebug` | `true` | Debug 前自动开始捕获 |
| `autocurl.injectDebugEnvironment` | `true` | 注入当前调试进程 |
| `autocurl.binaryPath` | 空 | 指定本地引擎路径 |
| `autocurl.autoDownload` | `true` | 自动下载并校验 Release |
| `autocurl.match` | 空 | 只显示 URL 包含该文本的请求 |
| `autocurl.method` | 空 | 只显示指定方法 |
| `autocurl.replayHeaders` | `[]` | 只给生成的 cURL 增加 Header |
| `autocurl.liveHeaders` | `[]` | 给真实请求增加 Header，谨慎使用 |
| `autocurl.showSecrets` | `false` | 关闭脱敏，不建议 |

打包：

```bash
cd ide/vscode
npm ci
npm run package
```

产物：`ide/vscode/autocurl-0.2.2.vsix`。

发布 VS Code Marketplace 需要创建 publisher 和凭据。Cursor 使用相同的
VS Code 扩展格式，其扩展市场以 Open VSX 为底层来源。首次入驻、GitHub
Secrets 和打 tag 自动发布的完整步骤见
[IDE 市场上架与自动升级](marketplace-publishing.zh-CN.md)。

## IntelliJ IDEA / GoLand / PyCharm / WebStorm

插件支持 2025.1（build 251）及以上版本。

安装 ZIP 后：

1. 选择一个已有的 Run/Debug Configuration。
2. 点击 **Run Selected with Autocurl** 或
   **Debug Selected with Autocurl**。
3. 插件创建临时配置、注入环境并用 IDE 原生 Runner 启动。
4. 在 **Autocurl** Tool Window 查看、选择、复制请求。

想立即验证 GoLand 插件，可以直接运行仓库中的
[`examples/go-http-client`](../examples/go-http-client/README.md)。它会发出一个
带 Query 的 GET 和一个带嵌套 JSON Body 的 POST，并携带 `get-info: true`。

原始 Run Configuration 不会被写入代理配置。常见 Java、Go、Python、
Node.js、Gradle 等配置使用的环境变量接口并不完全相同；插件分别适配通用
`getEnvs/setEnvs` 接口和 GoLand 的 `getCustomEnvironment/setCustomEnvironment`
接口。极少数自定义配置不提供环境变量时，插件会显示其真实类名，便于继续适配。

断点在发送之前时，选中请求 JSON，右键执行
**Render Selected Request JSON as cURL**。没有选区时会读取整个当前文档。
这个操作不发网络，只生成并复制 cURL。

打包：

```bash
cd ide/jetbrains
./gradlew buildPlugin verifyPluginStructure verifyPluginProjectConfiguration
```

产物：
`ide/jetbrains/build/distributions/autocurl-jetbrains-0.2.2.zip`。

使用本机 IDE 快速验证：

```bash
AUTOCURL_LOCAL_IDE="/Applications/GoLand.app" \
  ./gradlew buildPlugin verifyPluginStructure verifyPluginProjectConfiguration
```

发布 JetBrains Marketplace 使用 `./gradlew publishPlugin`，通过环境变量
`PUBLISH_TOKEN` 提供凭据。公开上架还需要 JetBrains vendor profile、协议、
商店资料和人工审核。JetBrains 强制首个版本人工上传，后续版本已经由 Release
工作流自动发布；详见
[IDE 市场上架与自动升级](marketplace-publishing.zh-CN.md)。

### 服务端启动后访问 127.0.0.1 被拒绝

`connect ECONNREFUSED 127.0.0.1:8080` 表示 8080 当时没有监听进程，不是
Autocurl 生成 cURL 失败。先在终端检查：

```bash
lsof -nP -iTCP:8080 -sTCP:LISTEN
curl --noproxy '*' -v http://127.0.0.1:8080/
```

如果普通 Run 能监听、只有 **Run Selected with Autocurl** 不能监听，请查看
该 Run 窗口最末尾的启动异常，并在 Issue 中附上 Run Configuration 的类型和
错误信息。Autocurl 自己监听 `127.0.0.1` 的随机空闲端口，不会占用 8080。
它捕获的是服务收到入站请求后“向外发出的 HTTP 调用”，不会把 Apifox 发给
服务的入站请求显示为捕获项。

### Go HTTPS 报临时 CA 不受信任

如果 Go 程序出现下面的错误：

```text
tls: failed to verify certificate: x509: “example.com” certificate is not trusted
```

先确认使用的是 0.2.2 或更高版本的 IDE 插件和引擎。0.2.1 插件曾错误地继续
复用 0.2.0 引擎，而 0.2.0 在 macOS 上没有 Go build-process CA overlay。
0.2.2 开始，插件与引擎版本必须匹配，旧的托管引擎会被自动替换。

## 引擎自动下载与安全

插件按以下顺序查找引擎：

1. 手动配置路径；
2. IDE 托管目录中已经下载的版本；
3. `PATH` 中的 `autocurl`；
4. GitHub 最新兼容 Release。

下载时按 macOS/Linux/Windows 与 amd64/arm64 选择压缩包，并核对
`SHA256SUMS`。IDE 包要求匹配的引擎版本，避免继续复用缺少运行时修复的旧缓存。

IDE 协议的 `ready.environment` 只包含新生成的覆盖值，不会把 IDE 父进程的
环境变量或密钥输出到 JSON。`GOFLAGS` 和 `JAVA_TOOL_OPTIONS` 标记为追加，
避免覆盖用户原有配置。

插件停止时关闭引擎 stdin；引擎关闭代理并删除临时 CA、Java truststore 和
Go overlay。不会修改系统代理或系统钥匙串。

## 必须说清楚的边界

- 请求真正发出：通过插件启动 Run/Debug，捕获真实请求。
- 断点停在发送之前：使用编辑器里能看到的完整请求 JSON 离线生成。
- 已经运行的进程无法在事后补加代理环境。
- 如果只有 body，没有 URL 和 Header，插件无法凭空推断。
- 证书固定、完全忽略代理的自定义客户端、HTTP/3、WebSocket 消息帧和
  protobuf 语义解析仍受主 README 中的限制。
