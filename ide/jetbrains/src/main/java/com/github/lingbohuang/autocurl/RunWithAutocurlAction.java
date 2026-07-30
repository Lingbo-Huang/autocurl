package com.github.lingbohuang.autocurl;

import com.intellij.execution.Executor;
import com.intellij.execution.ExecutionException;
import com.intellij.execution.ProgramRunnerUtil;
import com.intellij.execution.RunManager;
import com.intellij.execution.RunnerAndConfigurationSettings;
import com.intellij.execution.runners.ExecutionEnvironment;
import com.intellij.execution.runners.ExecutionEnvironmentBuilder;
import com.intellij.execution.BeforeRunTask;
import com.intellij.execution.configurations.RunConfiguration;
import com.intellij.openapi.actionSystem.AnAction;
import com.intellij.openapi.actionSystem.AnActionEvent;
import com.intellij.openapi.application.ApplicationManager;
import com.intellij.openapi.project.Project;
import com.intellij.openapi.ui.Messages;
import com.intellij.openapi.wm.ToolWindow;
import com.intellij.openapi.wm.ToolWindowManager;
import org.jetbrains.annotations.NotNull;

import java.util.ArrayList;
import java.util.List;

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
        RunManager runManager = RunManager.getInstance(project);
        RunnerAndConfigurationSettings selected = runManager.getSelectedConfiguration();
        if (selected == null) {
            Messages.showWarningDialog(project, "Select a Run/Debug configuration first.", "Autocurl");
            return;
        }
        RunnerAndConfigurationSettings temporary = copySelectedConfiguration(runManager, selected);
        temporary.setTemporary(true);
        temporary.setEditBeforeRun(false);
        temporary.setName(selected.getName() + " [Autocurl]");
        RunConfiguration configuration = temporary.getConfiguration();
        if (!RunConfigurationEnvironment.supports(configuration)) {
            Messages.showWarningDialog(
                    project,
                    "This Run Configuration type does not expose environment variables:\n"
                            + configuration.getClass().getName()
                            + "\n\nSupported standard configurations include Java, Go, Python, Node.js, "
                            + "Gradle, Maven, and most shell configurations."
                            + "\n\nFallbacks:"
                            + "\n1. Create a standard language Run/Debug configuration;"
                            + "\n2. Run `autocurl run --all -- <command>` and attach the debugger;"
                            + "\n3. Report the class name above so an adapter can be added.",
                    "Unsupported Run Configuration"
            );
            return;
        }

        CaptureSession session = CaptureSession.getInstance(project);
        session.setRerunAction(() -> run(project, executor));
        session.start().whenComplete((ready, error) -> ApplicationManager.getApplication().invokeLater(() -> {
            if (error != null) {
                Messages.showErrorDialog(project, rootMessage(error), "Autocurl Could Not Start");
                return;
            }
            if (!RunConfigurationEnvironment.inject(
                    configuration,
                    session.mergeEnvironment(RunConfigurationEnvironment.read(configuration), ready)
            )) {
                session.stopSession();
                Messages.showErrorDialog(
                        project,
                        "Autocurl could not update the temporary Run/Debug settings for "
                                + configuration.getClass().getName() + ".",
                        "Autocurl Could Not Start"
                );
                return;
            }
            session.expectProfile(configuration);
            try {
                ExecutionEnvironment environment = ExecutionEnvironmentBuilder
                        .create(executor, temporary)
                        .activeTarget()
                        .build();
                ProgramRunnerUtil.executeConfiguration(environment, false, true);
            } catch (ExecutionException | RuntimeException launchError) {
                session.cancelExpectedProfile(configuration);
                session.stopSession();
                Messages.showErrorDialog(
                        project,
                        rootMessage(launchError),
                        "Autocurl Could Not Start the Application"
                );
                return;
            }
            ToolWindow window = ToolWindowManager.getInstance(project).getToolWindow("Autocurl");
            if (window != null) window.show();
        }));
    }

    static RunnerAndConfigurationSettings copySelectedConfiguration(
            RunManager runManager,
            RunnerAndConfigurationSettings selected
    ) {
        RunConfiguration clonedConfiguration = selected.getConfiguration().clone();
        List<BeforeRunTask<?>> beforeRunTasks =
                new ArrayList<>(clonedConfiguration.getBeforeRunTasks());
        RunnerAndConfigurationSettings copied =
                runManager.createConfiguration(clonedConfiguration, selected.getFactory());

        // RunManager initializes a new settings wrapper from the configuration template,
        // which can replace the cloned configuration's before-run tasks. Restore the
        // cloned tasks so the temporary Autocurl run behaves exactly like the original.
        copied.getConfiguration().setBeforeRunTasks(beforeRunTasks);
        copied.setActivateToolWindowBeforeRun(selected.isActivateToolWindowBeforeRun());
        copied.setFocusToolWindowBeforeRun(selected.isFocusToolWindowBeforeRun());
        copied.setSingleton(selected.isSingleton());
        copied.setFolderName(selected.getFolderName());
        return copied;
    }

    private static String rootMessage(Throwable error) {
        Throwable current = error;
        while (current.getCause() != null) current = current.getCause();
        return current.getMessage() == null ? current.toString() : current.getMessage();
    }
}
