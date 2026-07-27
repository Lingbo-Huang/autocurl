package com.github.lingbohuang.autocurl;

import com.intellij.openapi.application.ApplicationManager;
import com.intellij.openapi.components.PersistentStateComponent;
import com.intellij.openapi.components.Service;
import com.intellij.openapi.components.State;
import com.intellij.openapi.components.Storage;
import org.jetbrains.annotations.NotNull;

import java.util.ArrayList;
import java.util.List;

@Service(Service.Level.APP)
@State(name = "AutocurlSettings", storages = @Storage("autocurl.xml"))
public final class AutocurlSettings implements PersistentStateComponent<AutocurlSettings.StateData> {
    public static final class StateData {
        public String binaryPath = "";
        public boolean autoDownload = true;
        public String match = "";
        public String method = "";
        public int maxBodyBytes = 1024 * 1024;
        public boolean showSecrets = false;
        public List<String> replayHeaders = new ArrayList<>();
        public List<String> liveHeaders = new ArrayList<>();
    }

    private StateData state = new StateData();

    public static AutocurlSettings getInstance() {
        return ApplicationManager.getApplication().getService(AutocurlSettings.class);
    }

    @Override
    public @NotNull StateData getState() {
        return state;
    }

    @Override
    public void loadState(@NotNull StateData state) {
        this.state = state;
    }
}
