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
            请求尚未发出时，在编辑器中选中请求 JSON，点击 Render JSON；
            它会直接生成并复制 cURL，不会发送网络请求。

            注意：
            必须通过 Autocurl 的 Run Selected / Debug Selected 启动。
            普通 Run/Debug 按钮不会自动捕获。
            引擎与过滤设置：Settings → Tools → Autocurl。
            """;

    private AutocurlHelp() {}

    static void show(Project project) {
        Messages.showInfoMessage(project, TOOL_WINDOW_GUIDE, "Autocurl Quick Start / 快速上手");
    }
}
