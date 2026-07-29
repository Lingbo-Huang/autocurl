# Autocurl 故障诊断与正确降级

这份文档用于回答两个问题：

1. 失败发生在业务程序、入站服务、Autocurl 代理，还是上游服务？
2. 当前请求能否透明捕获；不能时，怎样保证业务程序继续工作？

Autocurl 只给本次 Run/Debug 进程注入代理和临时证书。它不修改系统代理，
也不会代理 Apifox、浏览器或其他未由 Autocurl 启动的进程。

## 先运行环境检查

命令行：

```bash
autocurl doctor
```

JetBrains：打开 **Autocurl** 工具窗口，点击 **Environment Check**。

VS Code / Cursor：命令面板执行 **Autocurl: Run Environment Check**。

检查结果会列出 Go、Python、Node.js、Java 和 `keytool` 的实际路径、版本与
兼容性。尤其注意：Node 内置的环境变量代理需要 Node 22.21+ 或 24.5+；
旧版本或自定义 HTTP Agent 可能完全忽略代理环境变量。

## 服务端口访问失败

如果 Apifox 报：

```text
connect ECONNREFUSED 127.0.0.1:8080
```

它只说明当时没有进程在 `127.0.0.1:8080` 接受连接。按顺序检查：

```bash
lsof -nP -iTCP:8080 -sTCP:LISTEN
curl --noproxy '*' -v http://127.0.0.1:8080/
```

然后查看本次 Run/Debug 控制台：

- 配置根本没有启动：Autocurl 报 `application_not_started`。
- 程序提前退出：Autocurl 报 `application_exited_before_capture`。
- 程序仍运行但期望端口未监听：在设置中填写 `8080,8088`，Autocurl 等待
  指定秒数后报 `expected_port_not_listening`。
- 端口已监听但业务响应 502：入站服务正常，失败发生在服务自己的上游转发；
  它与“8080 没监听”不是同一个问题。

JetBrains 设置路径：
**Settings → Tools → Autocurl → Expected local listen ports**。

VS Code / Cursor：

```json
{
  "autocurl.expectedListenPorts": [8080, 8088],
  "autocurl.startupDiagnosticSeconds": 20
}
```

Autocurl 自己监听 `127.0.0.1` 的随机空闲端口，不会占用 8080 或 8088。

## Safe 与 Strict Capture

默认使用 **Safe**：

- 对每个 TLS 目标做一次兼容性探测，结果按主机缓存；
- 发现 mTLS、上游证书不可信或其他无法可靠拦截的 TLS 时，保持端到端直连；
- 如果客户端拒绝临时 CA，记录原因，并让该主机的下一次重试走直连；
- 兼容性优先。直连请求不会生成 cURL。

**Strict Capture** 强制所有可见 TLS 流量尝试中间代理：

- 捕获完整性优先；
- mTLS、证书固定、私有证书链不兼容时请求会失败；
- 适合定位“为什么没有被捕获”，不适合日常运行依赖复杂的服务。

CLI：

```bash
autocurl run --mode safe --all -- go run ./cmd/service
autocurl run --mode strict --all -- go run ./cmd/service
```

IDE 中在 Autocurl 设置里切换 `safe` / `strict`。

## mTLS、证书固定和私有证书链

这些场景不能承诺透明解密：

- mTLS 需要客户端私钥，代理没有也不应该导出业务进程的私钥；
- 证书固定的目的就是拒绝本地临时 CA；
- 某些公司内部证书链只存在于业务进程的专用 truststore 中；
- 自定义 Transport 可能完全绕过系统或环境代理。

Safe 模式检测到后会尽量保持业务流量可用，并给出诊断。需要永久直连时点击
**Always bypass this host and rerun**，或手动配置 Bypass。

CLI：

```bash
autocurl run --all \
  --bypass auth.internal.example \
  --bypass .infra.internal.example \
  --bypass 192.0.2.10 \
  --bypass 198.51.100.0/24 \
  -- go run ./cmd/service
```

支持单个域名、域名后缀、IP 和 CIDR；不要填写 `https://`、端口或路径。
Autocurl 会保留进程原有的 `NO_PROXY`、`no_proxy`、`no_grpc_proxy`，再去重
合并用户配置。

Java 的 `http.nonProxyHosts` 不原生支持 CIDR。Autocurl 会把可表达的 IPv4
CIDR 转换为通配规则；不能可靠表达的 IPv6 CIDR 会给出兼容性提示。Node 和
Python 的第三方客户端是否理解 CIDR 取决于其代理实现，必要时改用域名或单个 IP。

## 请求已经发出但列表为空

先确认：

1. JetBrains 使用的是 **Run Selected with Autocurl** 或
   **Debug Selected with Autocurl**，不是普通 Run/Debug。
2. VS Code / Cursor 的 `autocurl.injectDebugEnvironment` 为 `true`。
3. 业务进程是在捕获会话创建之后启动的。已经运行的进程不能事后补环境变量。
4. URL/method 过滤没有排除请求。
5. CLI 学习阶段使用了 `--all`；否则快速的 2xx 请求默认不会打印。

如果程序已经发出请求但 Autocurl 报 `no_proxy_traffic_observed`，客户端很可能
没有读取环境变量：

- Go：标准 `http.Transport` 只有 `Proxy: http.ProxyFromEnvironment` 时才读取；
  完全自定义的 `Transport.Proxy = nil` 会直连。
- Java：标准 `java.net.http` 可由 Autocurl 注入系统代理属性；自定义连接池可能
  有自己的代理设置。
- Python：标准库、`requests` 通常读取环境；`trust_env=False` 会忽略。
- Node：使用受支持的新版本内置代理，或为 Axios、undici、自定义 Agent 明确配置
  Autocurl 工具窗口所显示的代理 URL。

显式代理设置仍只应该应用到当前 Run/Debug 进程，不要改系统代理。

## TLS 报临时 CA 不受信任

典型错误：

```text
tls: failed to verify certificate: x509: certificate signed by unknown authority
```

先确认 IDE 插件和引擎都是 0.3.2：

- 插件版本在 IDE 插件页；
- 点击 **Environment Check** 查看实际引擎版本；
- 点击 **Download/Update Engine** 或清空手动 `binaryPath` 后重试。

GoLand 会把临时 x509 overlay 同时注入 Go 编译和运行阶段。已经编译好的独立
Go 二进制不能获得这个编译期适配；优先通过 Go Run/Build 配置重新构建，或将
目标加入 Bypass。

如果错误只发生在少数主机，通常是证书固定、mTLS 或专用证书链；使用 Safe
模式，并根据诊断绕过该主机。

## 协议降级表

| 场景 | 能否透明捕获 | 正确操作 |
| --- | --- | --- |
| HTTP/1.0、HTTP/1.1 | 是 | 正常运行 |
| HTTPS / HTTP/2 | 通常可以 | Safe 默认；需要强制排查时用 Strict |
| gRPC + proto/reflection | 传输可捕获 | 使用 `grpcurl` 做语义化重放 |
| gRPC 无 proto/reflection | 只能看传输层 | 查看 RPC path、metadata、status、trailer，不承诺语义 cURL |
| WebSocket `ws` / `wss` | 握手可捕获 | cURL 只代表握手；帧继续双向转发但不转成 cURL |
| HTTP/2 WebSocket | 暂不支持 | 使用协议专用调试工具 |
| HTTP/3 / QUIC | 不经过 TCP 代理 | 临时关闭 QUIC/强制 HTTP/2，或使用 QUIC 专用工具 |
| mTLS / 证书固定 | 不能透明解密 | Safe 自动直连或配置 Bypass |
| 自定义 Transport 忽略代理 | 不能零侵入强制 | 对该客户端显式设置进程级代理，或用离线 JSON 生成 |
| 已启动进程 | 不能事后注入 | 重新 Run/Debug，或离线生成 |
| 超大请求体 | 前缀捕获 | 调大 `maxBodyBytes`；注意内存与敏感数据风险 |

## 仍无法判断时，Issue 需要什么

请附上：

- `autocurl doctor` 输出；
- IDE、插件、引擎和语言运行时版本；
- Run Configuration 类型；
- Capture mode、Bypass 和 expected ports（隐藏内部域名/IP）；
- Run/Debug 控制台最后 50 行；
- Autocurl 的诊断 code 和 detail；
- “普通 Run”和“Run with Autocurl”的最小差异；
- 可公开复现的最小示例。

不要上传真实 Token、Cookie、内部域名、客户数据或未脱敏的完整 cURL。
