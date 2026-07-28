package com.github.lingbohuang.autocurl;

import com.intellij.openapi.options.Configurable;
import com.intellij.ui.components.JBCheckBox;
import com.intellij.ui.components.JBTextArea;
import com.intellij.ui.components.JBTextField;
import com.intellij.util.ui.FormBuilder;
import org.jetbrains.annotations.Nls;
import org.jetbrains.annotations.Nullable;

import javax.swing.JComponent;
import javax.swing.JPanel;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.util.Objects;

public final class AutocurlConfigurable implements Configurable {
    private final JBTextField binaryPath = new JBTextField();
    private final JBCheckBox autoDownload = new JBCheckBox("Download and verify the engine automatically");
    private final JBTextField match = new JBTextField();
    private final JBTextField method = new JBTextField();
    private final JBTextField maxBodyBytes = new JBTextField();
    private final JBCheckBox showSecrets = new JBCheckBox("Show secrets (unsafe)");
    private final JBTextArea replayHeaders = new JBTextArea(3, 40);
    private final JBTextArea liveHeaders = new JBTextArea(3, 40);
    private final JBTextArea bypassTargets = new JBTextArea(3, 40);
    private JPanel panel;

    @Override
    public @Nls String getDisplayName() {
        return "Autocurl";
    }

    @Override
    public @Nullable JComponent createComponent() {
        panel = FormBuilder.createFormBuilder()
                .addLabeledComponent("Engine path (optional):", binaryPath)
                .addComponent(autoDownload)
                .addLabeledComponent("URL contains:", match)
                .addLabeledComponent("HTTP method:", method)
                .addLabeledComponent("Maximum body bytes:", maxBodyBytes)
                .addComponent(showSecrets)
                .addLabeledComponent(
                        "Bypass capture (mTLS/infrastructure; one host, IP, domain suffix, or CIDR per line):",
                        bypassTargets)
                .addLabeledComponent("Replay-only headers (one per line):", replayHeaders)
                .addLabeledComponent("Live headers (one per line):", liveHeaders)
                .addComponentFillVertically(new JPanel(), 0)
                .getPanel();
        reset();
        return panel;
    }

    @Override
    public boolean isModified() {
        AutocurlSettings.StateData state = AutocurlSettings.getInstance().getState();
        return !Objects.equals(binaryPath.getText().trim(), state.binaryPath)
                || autoDownload.isSelected() != state.autoDownload
                || !Objects.equals(match.getText().trim(), state.match)
                || !Objects.equals(method.getText().trim(), state.method)
                || !Objects.equals(maxBodyBytes.getText().trim(), String.valueOf(state.maxBodyBytes))
                || showSecrets.isSelected() != state.showSecrets
                || !lines(bypassTargets.getText()).equals(state.bypassTargets)
                || !lines(replayHeaders.getText()).equals(state.replayHeaders)
                || !lines(liveHeaders.getText()).equals(state.liveHeaders);
    }

    @Override
    public void apply() {
        AutocurlSettings.StateData state = AutocurlSettings.getInstance().getState();
        state.binaryPath = binaryPath.getText().trim();
        state.autoDownload = autoDownload.isSelected();
        state.match = match.getText().trim();
        state.method = method.getText().trim().toUpperCase();
        try {
            state.maxBodyBytes = Math.max(1, Integer.parseInt(maxBodyBytes.getText().trim()));
        } catch (NumberFormatException ignored) {
            state.maxBodyBytes = 1024 * 1024;
        }
        state.showSecrets = showSecrets.isSelected();
        state.bypassTargets = lines(bypassTargets.getText());
        state.replayHeaders = lines(replayHeaders.getText());
        state.liveHeaders = lines(liveHeaders.getText());
    }

    @Override
    public void reset() {
        AutocurlSettings.StateData state = AutocurlSettings.getInstance().getState();
        binaryPath.setText(state.binaryPath);
        autoDownload.setSelected(state.autoDownload);
        match.setText(state.match);
        method.setText(state.method);
        maxBodyBytes.setText(String.valueOf(state.maxBodyBytes));
        showSecrets.setSelected(state.showSecrets);
        bypassTargets.setText(String.join("\n", state.bypassTargets));
        replayHeaders.setText(String.join("\n", state.replayHeaders));
        liveHeaders.setText(String.join("\n", state.liveHeaders));
    }

    private static List<String> lines(String value) {
        List<String> result = new ArrayList<>();
        Arrays.stream(value.split("\\R"))
                .map(String::trim)
                .filter(line -> !line.isEmpty())
                .forEach(result::add);
        return result;
    }
}
