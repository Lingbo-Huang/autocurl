package com.github.lingbohuang.autocurl;

import com.intellij.notification.Notification;
import com.intellij.notification.NotificationAction;
import com.intellij.notification.NotificationType;
import com.intellij.openapi.project.DumbAware;
import com.intellij.openapi.project.Project;
import com.intellij.openapi.startup.StartupActivity;
import com.intellij.openapi.wm.ToolWindow;
import com.intellij.openapi.wm.ToolWindowManager;
import org.jetbrains.annotations.NotNull;

public final class AutocurlStartupActivity implements StartupActivity, DumbAware {
    @Override
    public void runActivity(@NotNull Project project) {
        if (!AutocurlSettings.getInstance().markQuickStartShown()) return;

        Notification notification = new Notification(
                "Autocurl onboarding",
                "Autocurl 已安装",
                "选择一个 Run/Debug Configuration，然后使用 Run Selected with Autocurl 开始捕获请求。",
                NotificationType.INFORMATION
        );
        notification.addAction(NotificationAction.createSimpleExpiring(
                "打开 Autocurl",
                () -> {
                    ToolWindow window = ToolWindowManager.getInstance(project).getToolWindow("Autocurl");
                    if (window != null) window.show();
                }
        ));
        notification.addAction(NotificationAction.createSimpleExpiring(
                "查看快速上手",
                () -> AutocurlHelp.show(project)
        ));
        notification.notify(project);
    }
}
