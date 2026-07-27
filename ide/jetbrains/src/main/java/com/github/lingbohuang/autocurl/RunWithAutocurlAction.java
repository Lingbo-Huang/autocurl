package com.github.lingbohuang.autocurl;

import com.intellij.execution.CommonProgramRunConfigurationParameters;
import com.intellij.execution.Executor;
import com.intellij.execution.ProgramRunnerUtil;
import com.intellij.execution.RunManager;
import com.intellij.execution.RunnerAndConfigurationSettings;
import com.intellij.openapi.actionSystem.AnAction;
import com.intellij.openapi.actionSystem.AnActionEvent;
import com.intellij.openapi.application.ApplicationManager;
import com.intellij.openapi.project.Project;
import com.intellij.openapi.ui.Messages;
import com.intellij.openapi.wm.ToolWindow;
import com.intellij.openapi.wm.ToolWindowManager;
import org.jetbrains.annotations.NotNull;

abstract class RunWithAutocurlAction extends AnAction {
    protected abstract Executor executor();

    @Override
    public void update(@NotNull AnActionEvent event) {
        Project project = event.getProject();
        boolean enabled = project != null
                && RunManager.getInstance(project).getSelectedConfiguration() != null;
        event.getPresentation().setEnabled(enabled);
    }

    @Override
    public void actionPerformed(@NotNull AnActionEvent event) {
        Project project = event.getProject();
        if (project == null) return;
        run(project, executor());
    }

    static void run(Project project, Executor executor) {
        CaptureSession session = CaptureSession.getInstance(project);
        session.start().whenComplete((ready, error) -> ApplicationManager.getApplication().invokeLater(() -> {
            if (error != null) {
                Messages.showErrorDialog(project, rootMessage(error), "Autocurl Could Not Start");
                return;
            }
            RunnerAndConfigurationSettings selected =
                    RunManager.getInstance(project).getSelectedConfiguration();
            if (selected == null) {
                Messages.showWarningDialog(project, "Select a Run/Debug configuration first.", "Autocurl");
                return;
            }
            RunnerAndConfigurationSettings temporary = selected.createFactory().create();
            temporary.setTemporary(true);
            temporary.setName(selected.getName() + " [Autocurl]");
            if (!(temporary.getConfiguration() instanceof CommonProgramRunConfigurationParameters parameters)) {
                Messages.showWarningDialog(
                        project,
                        "This Run Configuration type does not expose standard environment variables. "
                                + "Java, Go, Python, Node.js, Gradle, and most common configurations are supported.",
                        "Autocurl"
                );
                return;
            }
            parameters.setEnvs(session.mergeEnvironment(parameters.getEnvs(), ready));
            ProgramRunnerUtil.executeConfiguration(project, temporary, executor);
            ToolWindow window = ToolWindowManager.getInstance(project).getToolWindow("Autocurl");
            if (window != null) window.show();
        }));
    }

    private static String rootMessage(Throwable error) {
        Throwable current = error;
        while (current.getCause() != null) current = current.getCause();
        return current.getMessage() == null ? current.toString() : current.getMessage();
    }
}
