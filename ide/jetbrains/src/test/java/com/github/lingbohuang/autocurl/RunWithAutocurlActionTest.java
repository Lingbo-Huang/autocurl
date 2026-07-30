package com.github.lingbohuang.autocurl;

import com.intellij.execution.BeforeRunTask;
import com.intellij.execution.RunManager;
import com.intellij.execution.RunnerAndConfigurationSettings;
import com.intellij.execution.configurations.ConfigurationFactory;
import com.intellij.execution.configurations.RunConfiguration;
import org.junit.jupiter.api.Test;

import java.util.List;

import static org.junit.jupiter.api.Assertions.assertSame;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

class RunWithAutocurlActionTest {
    @Test
    void copiesTheSelectedConfigurationInsteadOfCreatingAnEmptyOne() {
        RunManager runManager = mock(RunManager.class);
        RunnerAndConfigurationSettings selected = mock(RunnerAndConfigurationSettings.class);
        RunnerAndConfigurationSettings copied = mock(RunnerAndConfigurationSettings.class);
        RunConfiguration originalConfiguration = mock(RunConfiguration.class);
        RunConfiguration clonedConfiguration = mock(RunConfiguration.class);
        ConfigurationFactory factory = mock(ConfigurationFactory.class);
        BeforeRunTask<?> beforeRunTask = mock(BeforeRunTask.class);
        List<BeforeRunTask<?>> beforeRunTasks = List.of(beforeRunTask);

        when(selected.getConfiguration()).thenReturn(originalConfiguration);
        when(originalConfiguration.clone()).thenReturn(clonedConfiguration);
        when(clonedConfiguration.getBeforeRunTasks()).thenReturn(beforeRunTasks);
        when(selected.getFactory()).thenReturn(factory);
        when(runManager.createConfiguration(clonedConfiguration, factory)).thenReturn(copied);
        when(copied.getConfiguration()).thenReturn(clonedConfiguration);
        when(selected.isActivateToolWindowBeforeRun()).thenReturn(true);
        when(selected.isFocusToolWindowBeforeRun()).thenReturn(true);
        when(selected.isSingleton()).thenReturn(true);
        when(selected.getFolderName()).thenReturn("Examples");

        RunnerAndConfigurationSettings result =
                RunWithAutocurlAction.copySelectedConfiguration(runManager, selected);

        assertSame(copied, result);
        verify(originalConfiguration).clone();
        verify(selected, never()).createFactory();
        verify(clonedConfiguration).setBeforeRunTasks(beforeRunTasks);
        verify(copied).setActivateToolWindowBeforeRun(true);
        verify(copied).setFocusToolWindowBeforeRun(true);
        verify(copied).setSingleton(true);
        verify(copied).setFolderName("Examples");
    }
}
