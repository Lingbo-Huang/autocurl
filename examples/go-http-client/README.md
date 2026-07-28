# Go HTTP client demo

这个示例会使用 Go 标准库依次发出：

- 一个带 Query 参数和 `get-info: true` 的 GET 请求；
- 一个带嵌套 JSON Body 的 POST 请求；
- `Authorization`、`X-API-Key` 等用于验证脱敏的演示 Header。

## 在 GoLand 中验证 JetBrains 插件

1. 打开 `main.go`，点击行号旁边的运行图标，先创建一个普通的
   **Go Build** Run Configuration。
2. 在 IDE 顶部确认这个 Configuration 已被选中。
3. 点击 **Run → Run Selected with Autocurl**。
4. 等终端输出 `Done` 后，打开右侧 **Autocurl** 工具窗口。
5. 左侧应该出现 GET 和 POST 两条请求；选择 POST 后点击 **Copy cURL**。

默认请求发送到 `https://example.com/`。它是公开文档示例域名，POST 可能返回
`405 Method Not Allowed`；这不影响验证完整请求 Body 是否被捕获。如果公司网络
无法访问，
可以改成自己的测试接口：

```bash
go run ./examples/go-http-client -url https://your-test-service.example/api
```

也可以在 Run Configuration 中设置环境变量：

```text
AUTOCURL_DEMO_URL=https://your-test-service.example/api
```

示例中的 Token 和 API Key 都是假数据，请不要换成真实凭证后提交代码。
