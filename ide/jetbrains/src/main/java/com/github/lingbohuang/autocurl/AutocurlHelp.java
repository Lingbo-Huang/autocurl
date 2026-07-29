package com.github.lingbohuang.autocurl;

import com.intellij.openapi.project.Project;
import com.intellij.openapi.ui.Messages;

final class AutocurlHelp {
    static final String TOOL_WINDOW_GUIDE = """
            Autocurl 快速上手

            1. 在 IDE 顶部选择一个已有的 Run/Debug Configuration。
            2. 点击上方 Run Selected 或 Debug Selected。
            3. 程序发出 HTTP、HTTP/2、gRPC 或 WebSocket 握手请求。
            4. 在左侧选择请求，右侧会显示完整 cURL。
            5. 点击 Copy cURL，或双击左侧请求，复制到剪贴板。

            Body 怎么看：
            GET 请求通常没有 Body，所以只看到 URL 和 Header 是正常的。
            请选择 POST / PUT / PATCH 请求；有请求体时，完整内容会显示在
            --data-binary 后面。默认最多保留 1 MiB，可在 Settings → Tools → Autocurl
            的 Maximum body bytes 中调整；超过上限时会明确标记 truncated。

            断点调试：
            请求尚未发出时，从 Variables / Watches 复制请求对象的 JSON，
            再点击 Generate cURL from JSON。插件会优先读取编辑器选区，
            然后读取当前 JSON 文件，最后读取剪贴板；它不会发送网络请求。

            会话按钮：
            Pause Recording 只暂停记录，网络继续正常转发。
            Stop Session 先停止关联的业务程序，再关闭代理和临时 CA。
            Clear 只清空列表，不影响程序和网络。

            注意：
            必须通过 Autocurl 的 Run Selected / Debug Selected 启动。
            普通 Run/Debug 按钮不会自动捕获。
            引擎与过滤设置：Settings → Tools → Autocurl。
            遇到 mTLS、证书固定或自定义 Transport 时，使用 Safe 模式并按诊断绕过。
            """;

    private AutocurlHelp() {}

    static void show(Project project) {
        Messages.showInfoMessage(project, TOOL_WINDOW_GUIDE, "Autocurl Quick Start / 快速上手");
    }
}
