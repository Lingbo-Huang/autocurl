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
        event.getPresentation().setEnabled(
                project != null && FileEditorManager.getInstance(project).getSelectedTextEditor() != null
        );
    }

    @Override
    public void actionPerformed(@NotNull AnActionEvent event) {
        Project project = event.getProject();
        if (project == null) return;
        renderEditorRequest(project);
    }

    static void renderEditorRequest(Project project) {
        Editor editor = FileEditorManager.getInstance(project).getSelectedTextEditor();
        if (editor == null) {
            Messages.showInfoMessage(project, "Open a request JSON document first.", "Autocurl");
            return;
        }
        String requestJson = editor.getSelectionModel().getSelectedText();
        if (requestJson == null || requestJson.isBlank()) {
            requestJson = editor.getDocument().getText();
        }
        if (requestJson.isBlank()) {
            Messages.showInfoMessage(project, "Select request JSON or open it in the editor first.", "Autocurl");
            return;
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
                    Messages.showInfoMessage(
                            project,
                            "The generated cURL was copied to the clipboard.",
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
