package com.github.lingbohuang.autocurl;

import com.intellij.execution.Executor;
import com.intellij.execution.executors.DefaultRunExecutor;

public final class RunSelectedWithAutocurlAction extends RunWithAutocurlAction {
    @Override
    protected Executor executor() {
        return DefaultRunExecutor.getRunExecutorInstance();
    }
}
