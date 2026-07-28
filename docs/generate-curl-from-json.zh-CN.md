# 在请求尚未发送时生成 cURL

断点停在网络调用之前时，请求还没有经过代理，任何抓包工具都无法捕获它。
Autocurl 可以从调试器可见的请求描述离线生成 cURL；这个过程不会发送网络请求。

## IDE 中的操作

JetBrains：

1. 在 Variables / Watches 中找到准备发送的请求对象。
2. 使用 IDE 的 **Copy Value**，复制为 JSON；如果 IDE 只能复制 body，请另外
   准备 method、URL 和 headers。
3. 点击 Autocurl 工具窗口的 **Generate cURL from JSON**。
4. 插件按顺序读取：编辑器选区 → 当前 JSON 文件 → 剪贴板。
5. 选择与对象最接近的模板，确认生成结果后从工具窗口复制。

VS Code / Cursor：

1. 选中 JSON，或先将调试器对象复制到剪贴板。
2. 命令面板执行 **Autocurl: Generate cURL from Request JSON**。
3. 没有输入时，命令会创建可编辑模板；填完后再次执行。

当前 IntelliJ Platform 和 VS Code Debug Adapter 没有一个适用于所有语言和
调试器的稳定 API，可以让普通插件任意读取 Variables / Watches 中的对象。
因此 0.3.0 使用“Copy Value → 剪贴板”作为通用降级，不声称能够直接读取任意
调试器变量。未来可以为特定语言调试器增加右键 **Copy as cURL** 适配器。

## 支持的请求形状

通用：

```json
{
  "method": "POST",
  "url": "https://api.example.com/orders",
  "protocol": "HTTP/2",
  "headers": {"Content-Type": "application/json"},
  "body": {"order_id": "demo-42"}
}
```

Go `http.Request` 导出形状：

```json
{
  "Method": "POST",
  "URL": {
    "Scheme": "https",
    "Host": "api.example.com",
    "Path": "/orders"
  },
  "Header": {"Content-Type": ["application/json"]},
  "Body": {"order_id": "demo-42"}
}
```

Java `HttpRequest` 风格：

```json
{
  "method": "POST",
  "uri": "https://api.example.com/orders",
  "headers": {
    "map": {"Content-Type": ["application/json"]}
  },
  "body": {"order_id": "demo-42"}
}
```

Python `PreparedRequest` 的 `method`、`url`、`headers`、`body` 可以直接识别。
Node.js Axios 支持 `baseURL`、`url`、`method`、`headers`、`data`；Fetch 支持
`input` 或 `url` 加 `options`。

CLI 可以输出模板：

```bash
autocurl render --template generic
autocurl render --template go
autocurl render --template java
autocurl render --template python
autocurl render --template node
autocurl render --template fetch
```

## 无法自动恢复的信息

- Go 的 `Body` 常常是一次性 `io.ReadCloser`。如果调试器只显示 `{}` 或指针，
  Autocurl 无法倒推出已序列化字节。请在发送前复制原始 payload，放到小写
  `body` 字段。
- Java `BodyPublisher` 也可能不暴露内容，需要复制构造 publisher 时的 DTO 或
  字节数组。
- 只有 body 时不能推断 URL、Header 和认证中间件最终加上的字段。
- 流式、multipart 和二进制 body 不一定能安全放进 shell 参数；必要时保存到
  文件，再用 cURL 的 `--data-binary @file` 手工调整。
- 默认生成结果会脱敏。只有在私密本机调试中才使用 `--show-secrets`。
