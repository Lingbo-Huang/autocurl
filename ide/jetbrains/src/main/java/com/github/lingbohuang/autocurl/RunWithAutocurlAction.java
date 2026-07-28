package com.github.lingbohuang.autocurl;

import com.intellij.execution.Executor;
import com.intellij.execution.ProgramRunnerUtil;
import com.intellij.execution.RunManager;
import com.intellij.execution.RunnerAndConfigurationSettings;
import com.intellij.execution.configurations.RunConfiguration;
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
        RunnerAndConfigurationSettings selected =
                RunManager.getInstance(project).getSelectedConfiguration();
        if (selected == null) {
            Messages.showWarningDialog(project, "Select a Run/Debug configuration first.", "Autocurl");
            return;
        }
        RunnerAndConfigurationSettings temporary = selected.createFactory().create();
        temporary.setTemporary(true);
        temporary.setName(selected.getName() + " [Autocurl]");
        RunConfiguration configuration = temporary.getConfiguration();
        if (!RunConfigurationEnvironment.supports(configuration)) {
            Messages.showWarningDialog(
                    project,
                    "Autocurl cannot inject environment variables into this Run Configuration type:\n"
                            + configuration.getClass().getName()
                            + "\n\nPlease report this class name at github.com/Lingbo-Huang/autocurl/issues.",
                    "Unsupported Run Configuration"
            );
            return;
        }

        CaptureSession session = CaptureSession.getInstance(project);
        session.start().whenComplete((ready, error) -> ApplicationManager.getApplication().invokeLater(() -> {
            if (error != null) {
                Messages.showErrorDialog(project, rootMessage(error), "Autocurl Could Not Start");
                return;
            }
            if (!RunConfigurationEnvironment.inject(
                    configuration,
                    session.mergeEnvironment(RunConfigurationEnvironment.read(configuration), ready)
            )) {
                session.stop();
                Messages.showErrorDialog(
                        project,
                        "Autocurl could not update environment variables for "
                                + configuration.getClass().getName() + ".",
                        "Autocurl Could Not Start"
                );
                return;
            }
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
