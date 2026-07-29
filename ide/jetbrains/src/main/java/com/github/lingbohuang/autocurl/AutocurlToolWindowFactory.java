package com.github.lingbohuang.autocurl;

import com.intellij.execution.executors.DefaultDebugExecutor;
import com.intellij.execution.executors.DefaultRunExecutor;
import com.intellij.notification.Notification;
import com.intellij.notification.NotificationAction;
import com.intellij.notification.NotificationGroupManager;
import com.intellij.notification.NotificationType;
import com.intellij.openapi.options.ShowSettingsUtil;
import com.intellij.openapi.project.DumbAware;
import com.intellij.openapi.project.Project;
import com.intellij.openapi.application.ApplicationManager;
import com.intellij.openapi.ide.CopyPasteManager;
import com.intellij.openapi.ui.Messages;
import com.intellij.openapi.wm.ToolWindow;
import com.intellij.openapi.wm.ToolWindowFactory;
import com.intellij.ui.JBSplitter;
import com.intellij.ui.components.JBList;
import com.intellij.ui.components.JBScrollPane;
import com.intellij.ui.components.JBTextArea;
import com.intellij.ui.content.Content;
import com.intellij.ui.content.ContentFactory;
import com.intellij.util.ui.JBUI;
import org.jetbrains.annotations.NotNull;

import javax.swing.BoxLayout;
import javax.swing.DefaultListModel;
import javax.swing.JButton;
import javax.swing.JLabel;
import javax.swing.JPanel;
import java.awt.BorderLayout;
import java.awt.Component;
import java.awt.FlowLayout;
import java.awt.datatransfer.StringSelection;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.concurrent.TimeUnit;

public final class AutocurlToolWindowFactory implements ToolWindowFactory, DumbAware {
    @Override
    public void createToolWindowContent(@NotNull Project project, @NotNull ToolWindow toolWindow) {
        CaptureSession session = CaptureSession.getInstance(project);
        DefaultListModel<CaptureSession.RequestEvent> model = new DefaultListModel<>();
        session.requests().forEach(model::addElement);
        JBList<CaptureSession.RequestEvent> list = new JBList<>(model);
        list.getEmptyText().setText(
                "暂无请求：先选择 Run/Debug Configuration，再点击 Run Selected 或 Debug Selected"
        );
        JBTextArea curl = new JBTextArea();
        curl.setEditable(false);
        curl.setLineWrap(false);
        curl.setBorder(JBUI.Borders.empty(8));
        curl.setText(AutocurlHelp.TOOL_WINDOW_GUIDE);
        list.addListSelectionListener(event -> {
            CaptureSession.RequestEvent selected = list.getSelectedValue();
            curl.setText(selected == null ? AutocurlHelp.TOOL_WINDOW_GUIDE : selected.curl());
            curl.setCaretPosition(0);
        });
        list.addMouseListener(new java.awt.event.MouseAdapter() {
            @Override
            public void mouseClicked(java.awt.event.MouseEvent event) {
                if (event.getClickCount() == 2) copySelected(list);
            }
        });

        JLabel status = new JLabel();
        JButton run = new JButton("Run Selected");
        run.addActionListener(event -> RunWithAutocurlAction.run(
                project,
                DefaultRunExecutor.getRunExecutorInstance()
        ));
        JButton debug = new JButton("Debug Selected");
        debug.addActionListener(event -> RunWithAutocurlAction.run(
                project,
                DefaultDebugExecutor.getDebugExecutorInstance()
        ));
        JButton pause = new JButton("Pause Recording");
        pause.addActionListener(event -> session.setRecording(!session.isRecording()));
        JButton stop = new JButton("Stop Session");
        stop.addActionListener(event -> session.stopSession());
        JButton copy = new JButton("Copy cURL");
        copy.addActionListener(event -> copySelected(list));
        JButton render = new JButton("Generate cURL from JSON");
        render.addActionListener(event -> RenderSelectedRequestAction.renderEditorRequest(project));
        JButton help = new JButton("Quick Start / 使用说明");
        help.addActionListener(event -> AutocurlHelp.show(project));
        JButton doctor = new JButton("Environment Check");
        doctor.addActionListener(event -> runDiagnostics(project, session));
        JButton clear = new JButton("Clear");
        clear.addActionListener(event -> {
            session.clear();
            model.clear();
            curl.setText(AutocurlHelp.TOOL_WINDOW_GUIDE);
            curl.setCaretPosition(0);
        });

        JPanel toolbar = buildToolbar(
                run,
                debug,
                pause,
                stop,
                copy,
                render,
                help,
                doctor,
                clear,
                status
        );

        JBSplitter split = new JBSplitter(false, 0.45f);
        split.setFirstComponent(new JBScrollPane(list));
        split.setSecondComponent(new JBScrollPane(curl));
        JPanel panel = new JPanel(new BorderLayout());
        panel.add(toolbar, BorderLayout.NORTH);
        panel.add(split, BorderLayout.CENTER);

        Runnable updateState = () -> {
            if (!session.isRunning()) {
                status.setText("○ Stopped");
                pause.setText("Pause Recording");
                pause.setEnabled(false);
                stop.setEnabled(false);
            } else if (session.isRecording()) {
                status.setText("● Recording");
                pause.setText("Pause Recording");
                pause.setEnabled(true);
                stop.setEnabled(true);
            } else {
                status.setText("Ⅱ Paused · traffic still passes");
                pause.setText("Resume Recording");
                pause.setEnabled(true);
                stop.setEnabled(true);
            }
        };
        updateState.run();
        session.addStateListener(updateState);
        session.addRequestListener(request -> {
            model.addElement(request);
            list.setSelectedIndex(model.size() - 1);
            list.ensureIndexIsVisible(model.size() - 1);
        });
        session.addDiagnosticListener(diagnostic -> {
            if (list.getSelectedValue() == null) {
                curl.setText(formatDiagnostic(diagnostic));
                curl.setCaretPosition(0);
            }
            showDiagnostic(project, session, diagnostic);
        });
        session.addRenderedCurlListener(rendered -> {
            list.clearSelection();
            curl.setText(rendered);
            curl.setCaretPosition(0);
        });

        Content content = ContentFactory.getInstance().createContent(panel, "", false);
        toolWindow.getContentManager().addContent(content);
    }

    static JPanel buildToolbar(
            JButton run,
            JButton debug,
            JButton pause,
            JButton stop,
            JButton copy,
            JButton render,
            JButton help,
            JButton doctor,
            JButton clear,
            JLabel status
    ) {
        JPanel toolbar = new JPanel();
        toolbar.setLayout(new BoxLayout(toolbar, BoxLayout.Y_AXIS));
        toolbar.add(toolbarRow(run, debug, copy));
        toolbar.add(toolbarRow(pause, stop, clear));
        toolbar.add(toolbarRow(render, help));
        toolbar.add(toolbarRow(doctor, status));
        return toolbar;
    }

    private static JPanel toolbarRow(Component... components) {
        JPanel row = new JPanel(new FlowLayout(FlowLayout.LEFT, 6, 2));
        row.setAlignmentX(Component.LEFT_ALIGNMENT);
        for (Component component : components) {
            row.add(component);
        }
        return row;
    }

    private static void runDiagnostics(Project project, CaptureSession session) {
        ApplicationManager.getApplication().executeOnPooledThread(() -> {
            try {
                Process process = new ProcessBuilder(EngineManager.resolve(), "doctor").start();
                byte[] stdout = process.getInputStream().readNBytes(1024 * 1024);
                byte[] stderr = process.getErrorStream().readNBytes(1024 * 1024);
                if (!process.waitFor(10, TimeUnit.SECONDS)) {
                    process.destroyForcibly();
                    throw new IOException("Environment check timed out.");
                }
                String result = new String(stdout, StandardCharsets.UTF_8).trim();
                if (process.exitValue() != 0) {
                    String error = new String(stderr, StandardCharsets.UTF_8).trim();
                    throw new IOException(error.isBlank() ? "Environment check failed." : error);
                }
                session.showRenderedCurl(result);
            } catch (Exception error) {
                ApplicationManager.getApplication().invokeLater(() ->
                        Messages.showErrorDialog(
                                project,
                                error.getMessage() == null ? error.toString() : error.getMessage(),
                                "Autocurl Environment Check Failed"
                        )
                );
            }
        });
    }

    private static String formatDiagnostic(CaptureSession.DiagnosticEvent diagnostic) {
        StringBuilder result = new StringBuilder();
        result.append("Autocurl diagnostic: ").append(diagnostic.summary()).append("\n\n");
        if (diagnostic.host() != null && !diagnostic.host().isBlank()) {
            result.append("Host: ").append(diagnostic.host()).append("\n");
        }
        if (diagnostic.detail() != null && !diagnostic.detail().isBlank()) {
            result.append("Detail: ").append(diagnostic.detail()).append("\n");
        }
        if (diagnostic.suggested_action() != null && !diagnostic.suggested_action().isBlank()) {
            result.append("\nNext step: ").append(diagnostic.suggested_action()).append("\n");
        }
        if (diagnostic.auto_applied()) {
            result.append("\nSafe mode has already kept this host end-to-end for the current session.\n");
        }
        return result.toString();
    }

    private static void showDiagnostic(
            Project project,
            CaptureSession session,
            CaptureSession.DiagnosticEvent diagnostic
    ) {
        if ("information".equalsIgnoreCase(diagnostic.severity())) return;
        NotificationType type = "error".equalsIgnoreCase(diagnostic.severity())
                ? NotificationType.ERROR
                : "warning".equalsIgnoreCase(diagnostic.severity())
                ? NotificationType.WARNING
                : NotificationType.INFORMATION;
        String content = diagnostic.detail() == null ? "" : diagnostic.detail();
        if (diagnostic.suggested_action() != null && !diagnostic.suggested_action().isBlank()) {
            content += "\n" + diagnostic.suggested_action();
        }
        Notification notification = NotificationGroupManager.getInstance()
                .getNotificationGroup("Autocurl diagnostics")
                .createNotification(diagnostic.summary(), content, type);
        if (diagnostic.bypass_target() != null && !diagnostic.bypass_target().isBlank()) {
            notification.addAction(NotificationAction.createSimple(
                    "Always bypass this host and rerun",
                    () -> {
                        notification.expire();
                        session.bypassAndRerun(diagnostic.bypass_target());
                    }
            ));
        }
        notification.addAction(NotificationAction.createSimple(
                "Open Autocurl settings",
                () -> ShowSettingsUtil.getInstance().showSettingsDialog(project, "Autocurl")
        ));
        notification.notify(project);
    }

    private static void copySelected(JBList<CaptureSession.RequestEvent> list) {
        CaptureSession.RequestEvent selected = list.getSelectedValue();
        if (selected == null) {
            Messages.showInfoMessage("Select a captured request first.", "Autocurl");
            return;
        }
        CopyPasteManager.getInstance().setContents(new StringSelection(selected.curl()));
    }
}
