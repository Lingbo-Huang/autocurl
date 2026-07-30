# Autocurl 宣发素材包

所有文案都基于已经实现和验证的能力，不使用虚构下载量、客户、性能数据或节省
时间数字。发布前只需要替换演示 GIF/视频链接；官网和 GitHub 地址可以直接使用。

- 官网：https://lingbo-huang.github.io/autocurl/
- 中文官网：https://lingbo-huang.github.io/autocurl/zh/
- GitHub：https://github.com/Lingbo-Huang/autocurl
- Release：https://github.com/Lingbo-Huang/autocurl/releases/latest

## 品牌核心

中文一句话：

> 别再手工拼 cURL。Autocurl 在 IDE 里捕获代码真正发出的出站请求，一键复制
> 完整 cURL，不改业务代码，不改系统代理。

英文一句话：

> Stop rebuilding cURL by hand. Autocurl captures the outbound request your
> code actually sent and turns it into a copyable cURL inside your IDE—without
> changing application code or the system proxy.

短描述：

> 面向 Go、Java、Python、Node.js 的零侵入出站请求捕获工具，支持 JetBrains
> IDE、VS Code、Cursor，以及 HTTP/1.1、HTTP/2、gRPC 和 WebSocket。

英文短描述：

> Zero-intrusion outbound request capture for Go, Java, Python, and Node.js,
> with IDE-native workflows for JetBrains, VS Code, and Cursor.

建议统一使用的三个证明点：

1. 捕获的是业务代码最终真正发出的请求，不是从源码猜出来的请求；
2. 只作用于选中的 Run/Debug 进程，不修改操作系统代理；
3. Header、Query、嵌套 Body 和协议细节可以一起复制，常见凭据默认脱敏。

## GitHub 资料

### Repository description

```text
Capture the outbound request your code actually sent and copy a replayable cURL—inside your IDE, with no SDK or system proxy changes.
```

### About / Topics

建议 Topics：

```text
curl, debugging, developer-tools, ide-plugin, jetbrains, vscode, cursor,
http, http2, grpc, websocket, golang, java, python, nodejs
```

### Release 标题

```text
Autocurl v0.3.2 — IDE-native outbound request capture
```

### Release 摘要

```markdown
Autocurl turns the outbound request your code actually sent into a complete,
replayable cURL.

This release adds IDE-native workflows for JetBrains IDEs, VS Code, and Cursor,
plus verified capture for nested request bodies and HTTP/2. The capture session
is scoped to the selected Run/Debug process: no application SDK, no permanent
CA, and no operating-system proxy changes.

Highlights:

- Go, Java, Python, and Node.js;
- HTTP/1.1, HTTP/2, gRPC, and WebSocket;
- complete Header, Query, and nested request Body capture;
- process-scoped Safe mode and bypass rules for real infrastructure;
- one-click Copy cURL inside the IDE;
- MIT licensed, local-only, and redacted by default.

Website: https://lingbo-huang.github.io/autocurl/
Quick start: https://github.com/Lingbo-Huang/autocurl#three-step-quick-start
```

## 45 秒演示脚本

画面不要用内部域名和真实 Token。使用仓库的 `examples/java-client` 或
`examples/go-http-client`。

| 时间 | 画面 | 旁白/字幕 |
| --- | --- | --- |
| 0–4 秒 | IDE 中打开一个带嵌套 JSON Body 的 POST | “代码里的真实请求出了问题，你还在手工拼 cURL？” |
| 4–10 秒 | 选择已有 Run Configuration | “不用改代码，也不用改系统代理。” |
| 10–17 秒 | 点击 `Run Selected with Autocurl` | “直接用 Autocurl 启动这个配置。” |
| 17–25 秒 | 示例发出 GET 和 POST，请求列表出现 | “它只捕获这个进程真正发出的出站请求。” |
| 25–34 秒 | 选中 POST，展示 `--http2`、Header 和 `--data-binary` | “Header、协议和嵌套 Body 都在。” |
| 34–39 秒 | 点击 `Copy cURL`，粘贴到终端/编辑器 | “一键复制，马上复现。” |
| 39–45 秒 | 官网和 GitHub | “Autocurl：JetBrains、VS Code、Cursor，MIT 开源。” |

封面字幕：

```text
别再手工拼 cURL
捕获代码真正发出的请求
```

英文封面：

```text
Stop rebuilding cURL by hand
Capture what your code actually sent
```

## 五个低成本传播动作

### 1. “真实 Body 一键复制”短演示

- 渠道：V2EX、掘金、知乎、X、LinkedIn、开发者群；
- 核心信息：不是再做一个 API Client，而是捕获业务代码最终发出的请求；
- 为什么有效：嵌套 Body 前后对比是肉眼可见的痛点和结果；
- 成本：同一份 45 秒视频，加不同平台的开头文案；
- CTA：官网安装 + GitHub Star + 提交兼容性问题。

### 2. 工程事故复盘系列

- 渠道：掘金、知乎、公众号、个人博客、Hacker News；
- 核心信息：公开讲清零侵入代理、Go 证书验证、NO_PROXY、HTTP/2/gRPC 的真实坑；
- 为什么有效：具体技术判断比“我做了一个工具”更容易被收藏和引用；
- 成本：每次修复后的设计说明、测试和 Commit 都可以变成文章；
- CTA：运行仓库中的最小示例，复现实验。

### 3. 兼容性挑战

- 渠道：GitHub Issues、Reddit、语言社区和 IDE 社区；
- 核心信息：“告诉我哪个公开 HTTP Client 还抓不到，我用最小示例补兼容性”；
- 为什么有效：把潜在质疑转化为公开测试和贡献，形成长期内容；
- 成本：只接受公开最小复现，避免私有环境无限支持；
- CTA：使用 Feature/Bug 模板提交运行时、客户端和协议。

### 4. IDE Marketplace 搜索增长

- 渠道：JetBrains Marketplace、Visual Studio Marketplace、Open VSX；
- 核心信息：“Run/Debug 后直接在 IDE 里 Copy cURL”；
- 为什么有效：用户在出现痛点时会直接搜索 HTTP、cURL、debug、gRPC；
- 成本：完成首次上架后，后续由 tag 自动更新；
- CTA：市场安装和 Review；
- 要求：描述第一屏必须说明三步使用流程，不能只讲架构。

### 5. 团队试点访谈

- 渠道：现有同事网络、平台工程社区、GitHub Team Pilot Issue、技术会议；
- 核心信息：“个人捕获永久免费，正在验证共享脱敏、工作区预设和私有分发”；
- 为什么有效：不先卖尚未完成的 SaaS，而是先找到有预算的重复问题；
- 成本：10 次 30 分钟访谈和一个 YAML 原型；
- CTA：申请产品访谈、免费设计伙伴或固定范围付费集成。

## 中文发布文案

### V2EX

标题：

```text
[开源] Autocurl：在 IDE 里捕获代码真正发出的请求，一键复制完整 cURL
```

正文：

```markdown
大家好，我做了一个解决自己实际开发痛点的开源工具 Autocurl。

调试服务向外发起的 HTTP 调用时，我经常需要把代码里的真实请求变成 cURL：
URL、动态 Header、Query、序列化后的嵌套 Body、网关调试 Header，手工拼很慢，
而且特别容易漏字段。

Autocurl 做的事情很直接：

1. 在 JetBrains 里选择已有 Run/Debug Configuration，点击
   `Run Selected with Autocurl`；
2. VS Code / Cursor 像平时一样 Debug；
3. 触发业务逻辑，选中请求，点击 `Copy cURL`。

它捕获的是代码最终真正发出的请求，不是从源码猜请求。捕获只作用于本次
Run/Debug 进程，不修改系统代理，也不会向云端上传请求。

目前支持：

- Go、Java、Python、Node.js
- JetBrains IDE、VS Code、Cursor
- HTTP/1.1、HTTP/2、gRPC、WebSocket
- Header、Query、嵌套 Body
- 默认脱敏、mTLS/基础设施绕过

项目是 MIT 开源，刚发布 v0.3.2。非常希望大家用公开最小示例来挑战兼容性：
哪个运行时、HTTP Client 或协议还抓不到，欢迎提 Issue。

官网：https://lingbo-huang.github.io/autocurl/zh/
GitHub：https://github.com/Lingbo-Huang/autocurl
```

### 掘金/知乎动态

```text
调试代码向外发出的 HTTP 请求时，最烦的往往不是请求失败，而是为了复现问题，
还要从日志、配置和源码里手工拼一条 cURL。

我把这个痛点做成了开源工具 Autocurl：

- 选择已有的 IDE Run/Debug 配置；
- 触发业务请求；
- 在 IDE 里选择请求并复制完整 cURL。

Header、Query、嵌套 JSON Body 和 HTTP/2 信息都能保留。它只代理这次运行的
进程，不改业务代码，也不改系统代理。

支持 Go / Java / Python / Node.js，以及 JetBrains / VS Code / Cursor。
MIT 开源，欢迎带公开最小示例来测试兼容性。

官网：https://lingbo-huang.github.io/autocurl/zh/
GitHub：https://github.com/Lingbo-Huang/autocurl
```

### 微信群/飞书群/公司内部技术群

```text
分享一个我最近开源的小工具 Autocurl。

它解决的是：服务代码向外发请求后，为了排查问题还要手工拼 URL、Header 和大段
Body。现在可以直接用 IDE 的 Run/Debug 启动，触发请求后在 Autocurl 面板里
选择一条并 Copy cURL。

不改业务代码，不改系统代理，支持 Go/Java/Python/Node.js 和
HTTP/1.1、HTTP/2、gRPC、WebSocket。

有兴趣的同学可以用仓库里的公开示例试一下。如果遇到兼容性问题，请只提交脱敏
后的最小复现，不要贴内部域名和真实请求。

https://github.com/Lingbo-Huang/autocurl
```

### 朋友圈

```text
把一个日常开发痛点做成了开源工具：

Autocurl 能在 IDE 里捕获代码真正发出的出站请求，并一键复制完整 cURL。
不用改业务代码，不用改系统代理，支持 Go、Java、Python、Node.js。

从 CLI 做到 JetBrains / VS Code / Cursor，再补 HTTP/2、gRPC、WebSocket 和
本地证书兼容，比想象中踩了更多坑。接下来会把这些实现过程逐篇写出来。

GitHub：https://github.com/Lingbo-Huang/autocurl
```

### 公众号/掘金长文标题

首发建议：

```text
我受够了手工拼 cURL，于是做了一个零侵入 IDE 工具
```

技术系列：

```text
只代理一个进程：Autocurl 的零侵入请求捕获设计
```

```text
为什么本地 HTTPS 代理在 Go 里会遇到 x509 certificate is not trusted
```

```text
从 HTTP/1.1 到 HTTP/2、gRPC、WebSocket：请求到底该怎样被复现
```

```text
一次 ECONNREFUSED 排查：IDE Run Configuration、NO_PROXY 与本地监听
```

## 英文发布文案

### Hacker News / Show HN

标题：

```text
Show HN: Autocurl – Capture the outbound request your code sent inside your IDE
```

正文：

```text
I built Autocurl after repeatedly reconstructing cURL commands from source,
logs, configuration, middleware, and serialized request bodies.

Autocurl observes the request that the selected Run/Debug process actually
sends and renders a complete replayable cURL. It does not require an
application SDK, does not change the operating-system proxy, and does not
upload captured requests.

The current release supports:

- Go, Java, Python, and Node.js
- JetBrains IDEs, VS Code, and Cursor
- HTTP/1.1, HTTP/2, gRPC, and WebSocket
- headers, query parameters, and nested request bodies
- redaction by default and bypass rules for mTLS/infrastructure targets

It is MIT licensed. I would especially value public minimal reproductions for
HTTP clients or protocol combinations that are not captured correctly.

Website: https://lingbo-huang.github.io/autocurl/
GitHub: https://github.com/Lingbo-Huang/autocurl
```

### Reddit

推荐社区：`r/golang`、`r/java`、`r/Python`、`r/node`、
`r/programming`。先按对应语言运行公开示例，再针对社区改第一段，不要同日复制
群发。

```text
I kept losing time rebuilding outbound requests as cURL from application code,
especially after middleware, dynamic headers, and JSON serialization had
changed the final request.

I made an MIT-licensed tool called Autocurl. It scopes a temporary capture
session to one IDE Run/Debug process, then shows the actual outbound request as
a copyable cURL. No SDK and no system proxy changes.

The interesting implementation problems were process-scoped TLS trust,
HTTP/2/gRPC, WebSocket handshakes, mTLS bypass, and making the workflow feel
native in JetBrains and VS Code/Cursor.

I included small public examples for Go, Java, Python, and Node.js. Feedback on
unsupported clients and protocol edge cases is very welcome:
https://github.com/Lingbo-Huang/autocurl
```

### X / Twitter

```text
Stop rebuilding cURL by hand.

I built Autocurl to capture the outbound request your code actually sent and
copy it from your IDE—headers, nested body, and protocol included.

No SDK. No system proxy changes. Local-only and MIT licensed.

Go · Java · Python · Node.js
JetBrains · VS Code · Cursor
HTTP/1.1 · HTTP/2 · gRPC · WebSocket

https://lingbo-huang.github.io/autocurl/
```

### LinkedIn

```text
I turned a recurring backend debugging problem into an open-source developer
tool.

When an outbound request fails, reconstructing a cURL from source code is often
wrong: configuration, middleware, authentication, and serialization have
already changed the request.

Autocurl captures the request that the selected Run/Debug process actually
sends and makes the complete cURL copyable inside JetBrains IDEs, VS Code, and
Cursor.

The project is MIT licensed, processes requests locally, and supports Go, Java,
Python, Node.js, HTTP/1.1, HTTP/2, gRPC, and WebSocket.

Building it involved much more than an HTTP proxy: process-scoped TLS trust,
runtime-specific certificate behavior, mTLS bypass, IDE launch integration,
and safe redaction. I will publish the engineering lessons as a series.

Project: https://github.com/Lingbo-Huang/autocurl
```

### Product Hunt

名称：

```text
Autocurl
```

Tagline：

```text
Copy the cURL your code actually sent
```

描述：

```text
Autocurl captures outbound HTTP, HTTP/2, gRPC, and WebSocket requests from one
IDE Run/Debug process and turns them into safe, replayable cURLs. No application
SDK, no operating-system proxy changes, and no cloud upload.
```

首条评论：

```text
I built Autocurl after repeatedly reconstructing requests from source and
missing a dynamic header or serialized body field. The first design constraint
was strict: capture the real request without changing application code or the
machine-wide proxy.

The result is an MIT-licensed local tool for JetBrains IDEs, VS Code, and Cursor,
with a shared Go engine for Go, Java, Python, and Node.js. I would love feedback
on the first-run experience and public compatibility reproductions.
```

## 文章模板

```markdown
# 一个具体、可搜索的问题标题

## 失败现场

展示一个完全公开的最小示例和错误，不出现内部域名、Token 或业务数据。

## 为什么直觉方案失效

画出最小请求链路，解释运行时、IDE、代理和 TLS 的边界。

## 实现选择

说明 Autocurl 为什么选择进程级临时代理、Safe 模式、默认脱敏或特定协议处理。

## 真实验证

给出仓库示例、测试命令和看到完整 Body/cURL 的步骤。

## 仍然做不到什么

主动说明 mTLS、证书固定、断点停在发送前等边界和降级方式。

## 试用和反馈

官网、GitHub、公开最小复现 Issue。
```

## 七天发布节奏

| 日期 | 内容 | 目标 |
| --- | --- | --- |
| D-2 | 官网、README、市场页和 Release 自查 | 所有入口指向同一 Quick Start |
| D-1 | 45 秒 GIF/视频，找 3 人无指导试用 | 修正第一处卡点 |
| D0 | GitHub Release + V2EX + X/LinkedIn | 集中第一波访问 |
| D1 | 掘金/知乎技术长文 | 获取收藏和搜索流量 |
| D2 | 对所有有效评论逐一回复 | 把质疑变成 Issue/文档 |
| D4 | 发布第一个兼容性修复或 FAQ | 证明项目在维护 |
| D7 | 公布一周真实复盘，不夸大数字 | 建立长期可信度 |

## 发布前检查

- 演示中没有内部域名、账号、Cookie、Token、客户数据；
- 选择 POST 而不是无 Body 的 GET 来展示核心价值；
- cURL 中显示 `[REDACTED]`；
- 官网、Release、README、市场版本一致；
- JetBrains 和 VS Code/Cursor 下载按钮可用；
- Issue 模板明确要求公开最小复现和脱敏；
- 每个平台的首段适配受众，没有同文群发；
- 作者当天预留时间回复评论和修复第一批问题。
