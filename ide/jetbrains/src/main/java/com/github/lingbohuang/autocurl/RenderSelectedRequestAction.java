package com.github.lingbohuang.autocurl;

import com.google.gson.Gson;
import com.intellij.openapi.actionSystem.AnAction;
import com.intellij.openapi.actionSystem.AnActionEvent;
import com.intellij.openapi.application.ApplicationManager;
import com.intellij.openapi.editor.Editor;
import com.intellij.openapi.fileEditor.FileEditorManager;
import com.intellij.openapi.ide.CopyPasteManager;
import com.intellij.openapi.project.Project;
import com.intellij.openapi.ui.Messages;
import org.jetbrains.annotations.NotNull;

import java.awt.datatransfer.StringSelection;
import java.awt.datatransfer.DataFlavor;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.TimeUnit;

public final class RenderSelectedRequestAction extends AnAction {
    private static final Gson GSON = new Gson();
    private record RenderResult(String curl, String error) {}

    @Override
    public void update(@NotNull AnActionEvent event) {
        Project project = event.getProject();
        event.getPresentation().setEnabled(project != null);
    }

    @Override
    public void actionPerformed(@NotNull AnActionEvent event) {
        Project project = event.getProject();
        if (project == null) return;
        renderEditorRequest(project);
    }

    static void renderEditorRequest(Project project) {
        Editor editor = FileEditorManager.getInstance(project).getSelectedTextEditor();
        String requestJson = editor == null ? null : editor.getSelectionModel().getSelectedText();
        if ((requestJson == null || requestJson.isBlank()) && editor != null) {
            String document = editor.getDocument().getText().trim();
            if (looksLikeJsonObject(document)) requestJson = document;
        }
        if (requestJson == null || requestJson.isBlank()) {
            String clipboard = CopyPasteManager.getInstance().getContents(DataFlavor.stringFlavor);
            if (looksLikeJsonObject(clipboard)) requestJson = clipboard;
        }
        if (requestJson == null || requestJson.isBlank()) {
            requestJson = requestJsonDialog(project);
            if (requestJson == null || requestJson.isBlank()) return;
        }
        String finalRequestJson = requestJson;
        ApplicationManager.getApplication().executeOnPooledThread(() -> {
            try {
                String executable = EngineManager.resolve();
                Process process = new ProcessBuilder(
                        executable, "--json", "render", "--request", "-"
                ).start();
                CompletableFuture<byte[]> stdout = CompletableFuture.supplyAsync(
                        () -> readAll(process.getInputStream())
                );
                CompletableFuture<byte[]> stderr = CompletableFuture.supplyAsync(
                        () -> readAll(process.getErrorStream())
                );
                process.getOutputStream().write(finalRequestJson.getBytes(StandardCharsets.UTF_8));
                process.getOutputStream().close();
                if (!process.waitFor(15, TimeUnit.SECONDS)) {
                    process.destroyForcibly();
                    throw new IOException("Rendering request JSON timed out.");
                }
                String output = new String(stdout.get(), StandardCharsets.UTF_8);
                String errorOutput = new String(stderr.get(), StandardCharsets.UTF_8);
                RenderResult result = GSON.fromJson(output, RenderResult.class);
                if (process.exitValue() != 0 || result == null || result.curl() == null) {
                    String message = result != null && result.error() != null
                            ? result.error()
                            : errorOutput.isBlank() ? "The engine returned no cURL." : errorOutput.trim();
                    throw new IOException(message);
                }
                ApplicationManager.getApplication().invokeLater(() -> {
                    CopyPasteManager.getInstance().setContents(new StringSelection(result.curl()));
                    CaptureSession.getInstance(project).showRenderedCurl(result.curl());
                    Messages.showInfoMessage(
                            project,
                            "The generated cURL is shown in the Autocurl window and copied to the clipboard.",
                            "Autocurl"
                    );
                });
            } catch (Exception error) {
                ApplicationManager.getApplication().invokeLater(() ->
                        Messages.showErrorDialog(project, rootMessage(error), "Autocurl Render Failed")
                );
            }
        });
    }

    private static String requestJsonDialog(Project project) {
        String[] choices = {
                "Generic request",
                "Go http.Request",
                "Java HttpRequest",
                "Python PreparedRequest",
                "Node.js Axios",
                "Node.js fetch"
        };
        int choice = Messages.showChooseDialog(
                project,
                "Choose the closest request shape. Replace the example values with data visible in the debugger.",
                "Generate cURL from Request JSON",
                Messages.getQuestionIcon(),
                choices,
                choices[0]
        );
        if (choice < 0) return null;
        return Messages.showMultilineInputDialog(
                project,
                "Paste or edit the complete method, URL, headers, and body. This does not send a request.",
                "Generate cURL from Request JSON",
                template(choice),
                Messages.getQuestionIcon(),
                null
        );
    }

    private static String template(int choice) {
        return switch (choice) {
            case 1 -> """
                    {
                      "Method": "POST",
                      "URL": {"Scheme": "https", "Host": "api.example.com", "Path": "/orders"},
                      "Header": {"Content-Type": ["application/json"]},
                      "Body": {"order_id": "demo-42"}
                    }
                    """;
            case 2 -> """
                    {
                      "method": "POST",
                      "uri": "https://api.example.com/orders",
                      "headers": {"map": {"Content-Type": ["application/json"]}},
                      "body": {"order_id": "demo-42"}
                    }
                    """;
            case 3 -> """
                    {
                      "method": "POST",
                      "url": "https://api.example.com/orders",
                      "headers": {"Content-Type": "application/json"},
                      "body": {"order_id": "demo-42"}
                    }
                    """;
            case 4 -> """
                    {
                      "method": "post",
                      "baseURL": "https://api.example.com",
                      "url": "/orders",
                      "headers": {"Content-Type": "application/json"},
                      "data": {"order_id": "demo-42"}
                    }
                    """;
            case 5 -> """
                    {
                      "url": "https://api.example.com/orders",
                      "options": {
                        "method": "POST",
                        "headers": {"Content-Type": "application/json"},
                        "body": "{\\"order_id\\":\\"demo-42\\"}"
                      }
                    }
                    """;
            default -> """
                    {
                      "method": "POST",
                      "url": "https://api.example.com/orders",
                      "protocol": "HTTP/2",
                      "headers": {"Content-Type": "application/json"},
                      "body": {"order_id": "demo-42"}
                    }
                    """;
        };
    }

    private static boolean looksLikeJsonObject(String value) {
        return value != null && value.trim().startsWith("{");
    }

    private static String rootMessage(Throwable error) {
        Throwable current = error;
        while (current.getCause() != null) current = current.getCause();
        return current.getMessage() == null ? current.toString() : current.getMessage();
    }

    private static byte[] readAll(java.io.InputStream input) {
        try {
            return input.readAllBytes();
        } catch (IOException error) {
            throw new java.io.UncheckedIOException(error);
        }
    }
}
