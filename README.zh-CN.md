# autocurl 中文使用指南

[English README](README.md)

`autocurl` 在不修改业务代码的情况下，捕获应用真正发出的 HTTP 请求，并生成可以直接复制、编辑和重放的完整 cURL。

## IDE 插件：以后不用每次输入命令

### VS Code / Cursor

1. 从 [GitHub Releases](https://github.com/Lingbo-Huang/autocurl/releases)
   下载 `autocurl-0.2.0.vsix`。
2. 在命令面板执行 **Extensions: Install from VSIX...**。
3. 像平时一样点击 Debug 或按 F5。
4. 打开 **Explorer → Autocurl Requests**。点击某个请求查看完整 cURL，
   点击请求右侧按钮直接复制。

默认开启 `autocurl.autoStartOnDebug`，所以不需要先开终端，也不需要先执行
`autocurl run`。Cursor 基于 VS Code 扩展体系，同一份 VSIX 可以直接安装。

### IntelliJ IDEA / GoLand / PyCharm / WebStorm

1. 从 Releases 下载 `autocurl-jetbrains-0.2.0.zip`。
2. 打开 **Settings → Plugins → ⚙ → Install Plugin from Disk**。
3. 选择已有的 Run/Debug Configuration。
4. 使用 **Run → Run Selected with Autocurl** 或
   **Run → Debug Selected with Autocurl**。
5. 在 **Autocurl** Tool Window 中选择请求，点击 **Copy cURL**。

JetBrains 插件会复制一个临时运行配置并注入代理环境，不会永久修改原有
Run Configuration。

两个插件首次使用时都会从 GitHub Release 下载匹配当前操作系统和 CPU 的
Go 引擎，并使用 `SHA256SUMS` 校验。如果公司网络不能访问 GitHub，可以只安装
一次二进制，然后在 VS Code/Cursor 的 `autocurl.binaryPath`，或 JetBrains 的
**Settings → Tools → Autocurl** 中配置路径。

断点停在真正发送之前时，在编辑器里选择包含 `method`、`url`、`headers`、
`body` 的请求 JSON，然后执行 **Render Selected Request JSON**。插件只生成并
复制 cURL，不会发送网络请求。

完整安装、配置、打包和发布方法见
[IDE 插件指南](docs/ide-plugins.zh-CN.md)。

## 最快上手：捕获并复制某一个请求

例如要查看 URL 中包含 `/orders` 的 POST 请求：

```bash
autocurl run \
  --all \
  --match '/orders' \
  --method POST \
  --copy \
  -- java -jar app.jar
```

然后：

1. 不要关闭这个终端。
2. 使用 Apifox、浏览器、测试用例或其他服务触发业务逻辑。
3. 当应用真正向外发送匹配的请求时，终端会出现：

   ```text
   [autocurl #1] POST https://api.example.com/orders -> 200 (86ms)
   [autocurl #1] http · client HTTP/2.0 · upstream HTTP/2.0
   curl \
     --request 'POST' \
     --url 'https://api.example.com/orders' \
     --header 'Content-Type: application/json' \
     --data-binary '{"order_id":"demo-42"}'
   [autocurl #1] copied cURL to clipboard
   ```

4. 直接按 <kbd>Cmd</kbd>+<kbd>V</kbd> 或 <kbd>Ctrl</kbd>+<kbd>V</kbd> 粘贴完整 cURL。

第一次使用一定建议加 `--all`。如果不加，默认只输出：

- 网络或代理错误；
- HTTP 状态码大于等于 400；
- 耗时超过两秒的成功请求；
- `grpc-status` 非零的 gRPC 请求。

所以“程序明明发请求了，但终端什么都没显示”，通常只是因为请求成功且不慢。

### 选择和保存请求

```text
--match '/orders'          URL 中包含 /orders
--method POST              只选择 POST
--copy                     自动复制；多个匹配时保留最后一个
--output ./request.sh      将最后一个匹配请求写入文件
```

保存示例：

```bash
autocurl run --all --match '/orders' --output ./order.sh -- python3 app.py
```

## 添加网关调试 Header

只将 Header 添加到生成的 cURL，不改变应用真实请求：

```bash
autocurl run \
  --all \
  --match '/orders' \
  --replay-header 'x-debug-info: true' \
  --copy \
  -- python3 app.py
```

如果需要真实请求也携带 Header，使用 `--live-header`。

## IDE Debug 怎么使用

### 请求最终会真正发送

让 `autocurl` 启动调试进程，然后 IDE 使用 Attach 模式连接。

Java：

```bash
autocurl run --all --match '/orders' --copy -- \
  java -agentlib:jdwp=transport=dt_socket,server=y,suspend=y,address=127.0.0.1:5005 \
  -jar app.jar
```

然后 IntelliJ IDEA 使用 Remote JVM Debug 连接 `127.0.0.1:5005`。

Python：

```bash
autocurl run --all --match '/orders' --copy -- \
  python3 -m debugpy --listen 127.0.0.1:5678 --wait-for-client app.py
```

Go：

```bash
autocurl run --all --match '/orders' --copy -- \
  dlv debug --headless --listen=127.0.0.1:2345 --api-version=2 ./cmd/service
```

### 断点停在发送之前

这时网络中还没有请求，代理无法捕获。但如果调试器里已经能看到 method、URL、headers 和 body，可以使用 `render`。它只生成 cURL，不发送请求。

创建 `request.json`：

```json
{
  "method": "POST",
  "url": "https://api.example.com/orders",
  "protocol": "HTTP/2",
  "headers": {
    "Content-Type": "application/json",
    "Authorization": "Bearer local-debug-token"
  },
  "body": {
    "order_id": "demo-42",
    "items": [{"sku": "A-42", "quantity": 2}]
  }
}
```

生成并复制：

```bash
autocurl render --request request.json --copy
```

也可以把调试器中复制出来的 body 保存到文件：

```bash
autocurl render \
  --method POST \
  --url https://api.example.com/orders \
  --header 'Content-Type: application/json' \
  --body-file body.json \
  --copy
```

macOS 上可以直接读取剪贴板中的 body：

```bash
pbpaste | autocurl render \
  --method POST \
  --url https://api.example.com/orders \
  --header 'Content-Type: application/json' \
  --body-file - \
  --copy
```

必须能从调试器中取得完整的 method、URL、headers 和 body。只有请求体时，工具无法凭空推断 URL 和 Header。

已经运行的进程不能在启动后被补加代理和证书环境变量。可以重新通过 `autocurl run` 启动并让 IDE Attach，或者使用 `render`。

## 我们如何测试

自动化测试覆盖：

- HTTP/1.0、HTTP/1.1、HTTP/2 双向协议协商和转换；
- TLS 与 h2c gRPC frame、trailer、`grpc-status`；
- `ws` 与 `wss` 101 握手和双向字节传输；
- Go、Python、Java 的代理与临时证书配置；
- 请求筛选、编号、剪贴板、输出文件和离线 `render`；
- 敏感字段脱敏和 body 截断。

另外使用真实 HTTPS 请求验证过 Python 标准库、Go `net/http` 和 Java 24 `java.net.http`。Java 测试中，客户端到代理和代理到服务端均为 HTTP/2。

## 安装

```bash
go install github.com/Lingbo-Huang/autocurl/cmd/autocurl@latest
autocurl doctor
```

更多协议、安全和架构信息请查看 [English README](README.md)、
[调试指南](docs/debugging.md) 与 [IDE 插件指南](docs/ide-plugins.zh-CN.md)。
