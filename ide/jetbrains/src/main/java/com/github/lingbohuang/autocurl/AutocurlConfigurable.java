package com.github.lingbohuang.autocurl;

import com.intellij.openapi.options.Configurable;
import com.intellij.ui.components.JBCheckBox;
import com.intellij.openapi.ui.ComboBox;
import com.intellij.ui.ScrollPaneFactory;
import com.intellij.ui.components.JBTextArea;
import com.intellij.ui.components.JBTextField;
import com.intellij.util.ui.FormBuilder;
import org.jetbrains.annotations.Nls;
import org.jetbrains.annotations.Nullable;

import javax.swing.JComponent;
import javax.swing.JPanel;
import javax.swing.JScrollPane;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.util.Objects;

public final class AutocurlConfigurable implements Configurable {
    private final JBTextField binaryPath = new JBTextField();
    private final JBCheckBox autoDownload = new JBCheckBox("Download and verify the engine automatically");
    private final ComboBox<String> captureMode = new ComboBox<>(new String[]{"safe", "strict"});
    private final JBTextField match = new JBTextField();
    private final JBTextField method = new JBTextField();
    private final JBTextField maxBodyBytes = new JBTextField();
    private final JBTextField startupDiagnosticSeconds = new JBTextField();
    private final JBTextField expectedListenPorts = new JBTextField();
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
                .addLabeledComponent(
                        "Capture mode (safe keeps incompatible TLS end-to-end; strict forces interception):",
                        captureMode)
                .addLabeledComponent("URL contains:", match)
                .addLabeledComponent("HTTP method:", method)
                .addLabeledComponent("Maximum body bytes:", maxBodyBytes)
                .addLabeledComponent("Startup diagnostic delay (seconds):", startupDiagnosticSeconds)
                .addLabeledComponent(
                        "Expected local listen ports (optional, comma-separated; for example 8080,8088):",
                        expectedListenPorts)
                .addComponent(showSecrets)
                .addLabeledComponent(
                        "Bypass capture (mTLS/infrastructure; one host, IP, domain suffix, or CIDR per line):",
                        multilineInput(bypassTargets))
                .addLabeledComponent(
                        "Replay-only headers (one per line):",
                        multilineInput(replayHeaders))
                .addLabeledComponent(
                        "Live headers (one per line):",
                        multilineInput(liveHeaders))
                .addComponentFillVertically(new JPanel(), 0)
                .getPanel();
        reset();
        return panel;
    }

    static JScrollPane multilineInput(JBTextArea textArea) {
        textArea.setLineWrap(true);
        textArea.setWrapStyleWord(true);
        return ScrollPaneFactory.createScrollPane(
                textArea,
                JScrollPane.VERTICAL_SCROLLBAR_AS_NEEDED,
                JScrollPane.HORIZONTAL_SCROLLBAR_NEVER
        );
    }

    @Override
    public boolean isModified() {
        AutocurlSettings.StateData state = AutocurlSettings.getInstance().getState();
        return !Objects.equals(binaryPath.getText().trim(), state.binaryPath)
                || autoDownload.isSelected() != state.autoDownload
                || !Objects.equals(captureMode.getSelectedItem(), state.captureMode)
                || !Objects.equals(match.getText().trim(), state.match)
                || !Objects.equals(method.getText().trim(), state.method)
                || !Objects.equals(maxBodyBytes.getText().trim(), String.valueOf(state.maxBodyBytes))
                || !Objects.equals(
                        startupDiagnosticSeconds.getText().trim(),
                        String.valueOf(state.startupDiagnosticSeconds))
                || !Objects.equals(expectedListenPorts.getText().trim(), joinPorts(state.expectedListenPorts))
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
        state.captureMode = Objects.equals(captureMode.getSelectedItem(), "strict") ? "strict" : "safe";
        state.match = match.getText().trim();
        state.method = method.getText().trim().toUpperCase();
        try {
            state.maxBodyBytes = Math.max(1, Integer.parseInt(maxBodyBytes.getText().trim()));
        } catch (NumberFormatException ignored) {
            state.maxBodyBytes = 1024 * 1024;
        }
        try {
            state.startupDiagnosticSeconds = Math.max(
                    1,
                    Integer.parseInt(startupDiagnosticSeconds.getText().trim())
            );
        } catch (NumberFormatException ignored) {
            state.startupDiagnosticSeconds = 15;
        }
        state.expectedListenPorts = parsePorts(expectedListenPorts.getText());
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
        captureMode.setSelectedItem(Objects.equals(state.captureMode, "strict") ? "strict" : "safe");
        match.setText(state.match);
        method.setText(state.method);
        maxBodyBytes.setText(String.valueOf(state.maxBodyBytes));
        startupDiagnosticSeconds.setText(String.valueOf(state.startupDiagnosticSeconds));
        expectedListenPorts.setText(joinPorts(state.expectedListenPorts));
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

    private static List<Integer> parsePorts(String value) {
        List<Integer> result = new ArrayList<>();
        Arrays.stream(value.split("[,\\s]+"))
                .map(String::trim)
                .filter(item -> !item.isEmpty())
                .forEach(item -> {
                    try {
                        int port = Integer.parseInt(item);
                        if (port >= 1 && port <= 65535 && !result.contains(port)) result.add(port);
                    } catch (NumberFormatException ignored) {
                    }
                });
        return result;
    }

    private static String joinPorts(List<Integer> ports) {
        return ports == null ? "" : ports.stream()
                .map(String::valueOf)
                .collect(java.util.stream.Collectors.joining(","));
    }
}
