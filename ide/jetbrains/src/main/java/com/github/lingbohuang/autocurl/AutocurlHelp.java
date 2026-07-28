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
