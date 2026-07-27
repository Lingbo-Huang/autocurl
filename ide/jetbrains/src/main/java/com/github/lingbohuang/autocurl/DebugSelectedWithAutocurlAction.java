package com.github.lingbohuang.autocurl;

import com.intellij.execution.Executor;
import com.intellij.execution.executors.DefaultDebugExecutor;

public final class DebugSelectedWithAutocurlAction extends RunWithAutocurlAction {
    @Override
    protected Executor executor() {
        return DefaultDebugExecutor.getDebugExecutorInstance();
    }
}
