package com.github.lingbohuang.autocurl;

import com.google.gson.Gson;
import com.intellij.execution.ExecutionListener;
import com.intellij.execution.ExecutionManager;
import com.intellij.execution.configurations.RunConfiguration;
import com.intellij.execution.configurations.RunProfile;
import com.intellij.execution.process.ProcessHandler;
import com.intellij.execution.runners.ExecutionEnvironment;
import com.intellij.openapi.Disposable;
import com.intellij.openapi.application.ApplicationManager;
import com.intellij.openapi.components.Service;
import com.intellij.openapi.diagnostic.Logger;
import com.intellij.openapi.project.Project;

import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.net.Socket;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.TimeUnit;
import java.util.function.Consumer;

@Service(Service.Level.PROJECT)
public final class CaptureSession implements Disposable {
    public record ReadyEvent(
            String type,
            String version,
            String proxy_url,
            String ca_file,
            Map<String, String> environment,
            List<String> append_environment,
            List<String> notes,
            String mode
    ) {}

    public record RequestEvent(
            String type,
            long id,
            String timestamp,
            String method,
            String url,
            String protocol,
            String upstream_protocol,
            String kind,
            Integer status,
            String grpc_status,
            long duration_ms,
            String error,
            String curl,
            boolean redacted,
            boolean body_truncated,
            String warning
    ) {
        @Override
        public String toString() {
            String outcome = grpc_status != null && !grpc_status.isBlank()
                    ? "gRPC " + grpc_status
                    : status != null && status > 0 ? String.valueOf(status) : error != null ? "error" : "";
            return method + " " + url + (outcome.isBlank() ? "" : "  [" + outcome + "]")
                    + "  " + duration_ms + " ms";
        }
    }

    public record DiagnosticEvent(
            String type,
            String timestamp,
            String code,
            String severity,
            String host,
            String summary,
            String detail,
            String suggested_action,
            String bypass_target,
            boolean auto_applied,
            boolean retry_required
    ) {}

    private record StateEvent(String type, boolean recording) {}

    private static final Logger LOG = Logger.getInstance(CaptureSession.class);
    private static final Gson GSON = new Gson();
    private final List<RequestEvent> requests = new CopyOnWriteArrayList<>();
    private final List<Consumer<RequestEvent>> requestListeners = new CopyOnWriteArrayList<>();
    private final List<Consumer<DiagnosticEvent>> diagnosticListeners = new CopyOnWriteArrayList<>();
    private final List<Consumer<String>> renderedCurlListeners = new CopyOnWriteArrayList<>();
    private final List<Runnable> stateListeners = new CopyOnWriteArrayList<>();
    private final Set<RunProfile> expectedProfiles =
            Collections.newSetFromMap(new ConcurrentHashMap<>());
    private final Set<RunProfile> failedProfiles =
            Collections.newSetFromMap(new ConcurrentHashMap<>());
    private final Set<ProcessHandler> targetProcesses =
            Collections.newSetFromMap(new ConcurrentHashMap<>());
    private volatile Process process;
    private volatile ReadyEvent ready;
    private volatile CompletableFuture<ReadyEvent> starting;
    private volatile boolean recording;
    private volatile boolean engineStopRequested;
    private volatile Runnable rerunAction;

    public CaptureSession(Project project) {
        project.getMessageBus().connect(this).subscribe(
                ExecutionManager.EXECUTION_TOPIC,
                new ExecutionListener() {
                    @Override
                    public void processStarted(
                            String executorId,
                            ExecutionEnvironment environment,
                            ProcessHandler handler
                    ) {
                        if (claimProfile(environment.getRunProfile())) {
                            targetProcesses.add(handler);
                            scheduleStartupDiagnostics(handler);
                            fireStateChanged();
                        }
                    }

                    @Override
                    public void processNotStarted(
                            String executorId,
                            ExecutionEnvironment environment
                    ) {
                        handleNotStarted(environment, null);
                    }

                    @Override
                    public void processNotStarted(
                            String executorId,
                            ExecutionEnvironment environment,
                            Throwable cause
                    ) {
                        handleNotStarted(environment, cause);
                    }

                    private void handleNotStarted(
                            ExecutionEnvironment environment,
                            Throwable cause
                    ) {
                        RunProfile profile = environment.getRunProfile();
                        if (!claimProfile(profile) || !failedProfiles.add(profile)) return;
                        emitLocalDiagnostic(new DiagnosticEvent(
                                "diagnostic",
                                "",
                                "application_not_started",
                                "error",
                                "",
                                "The selected Run/Debug configuration did not start",
                                cause == null ? "" : rootMessage(cause),
                                "Open the Run/Debug console, fix the launch error, and run with Autocurl again.",
                                "",
                                false,
                                true
                        ));
                        stopEngine();
                    }

                    @Override
                    public void processTerminated(
                            String executorId,
                            ExecutionEnvironment environment,
                            ProcessHandler handler,
                            int exitCode
                    ) {
                        if (!targetProcesses.remove(handler)) return;
                        if (exitCode != 0 && requests.isEmpty()) {
                            emitLocalDiagnostic(new DiagnosticEvent(
                                    "diagnostic",
                                    "",
                                    "application_exited_before_capture",
                                    "error",
                                    "",
                                    "The application exited before Autocurl captured a request",
                                    "Process exit code: " + exitCode,
                                    "Inspect the Run/Debug console for startup, dependency, or port-binding errors.",
                                    "",
                                    false,
                                    true
                            ));
                        }
                        if (targetProcesses.isEmpty()) stopEngine();
                        fireStateChanged();
                    }
                }
        );
    }

    private boolean claimProfile(RunProfile profile) {
        if (expectedProfiles.remove(profile)) return true;
        ReadyEvent event = ready;
        if (event == null || !(profile instanceof RunConfiguration configuration)) return false;
        if (!RunConfigurationEnvironment.supports(configuration)) return false;
        Map<String, String> environment = RunConfigurationEnvironment.read(configuration);
        return event.proxy_url().equals(environment.get("HTTP_PROXY"))
                || event.proxy_url().equals(environment.get("http_proxy"));
    }

    public static CaptureSession getInstance(Project project) {
        return project.getService(CaptureSession.class);
    }

    public synchronized CompletableFuture<ReadyEvent> start() {
        if (ready != null) return CompletableFuture.completedFuture(ready);
        if (starting != null) return starting;
        starting = new CompletableFuture<>();
        engineStopRequested = false;
        ApplicationManager.getApplication().executeOnPooledThread(this::runEngine);
        return starting;
    }

    private void runEngine() {
        CompletableFuture<ReadyEvent> future = starting;
        try {
            String executable = EngineManager.resolve();
            List<String> command = new ArrayList<>();
            command.add(executable);
            command.addAll(List.of("--json", "proxy", "--lifetime-stdin"));
            AutocurlSettings.StateData settings = AutocurlSettings.getInstance().getState();
            command.addAll(List.of(
                    "--mode",
                    "strict".equals(settings.captureMode) ? "strict" : "safe"
            ));
            if (!settings.match.isBlank()) command.addAll(List.of("--match", settings.match));
            if (!settings.method.isBlank()) command.addAll(List.of("--method", settings.method));
            command.addAll(List.of("--max-body", String.valueOf(settings.maxBodyBytes)));
            if (settings.showSecrets) command.add("--show-secrets");
            settings.bypassTargets.forEach(target -> command.addAll(List.of("--bypass", target)));
            settings.replayHeaders.forEach(header -> command.addAll(List.of("--replay-header", header)));
            settings.liveHeaders.forEach(header -> command.addAll(List.of("--live-header", header)));

            requests.clear();
            recording = false;
            fireStateChanged();
            Process child = new ProcessBuilder(command).start();
            process = child;
            ApplicationManager.getApplication().executeOnPooledThread(() -> logErrors(child));
            try (BufferedReader reader = new BufferedReader(
                    new InputStreamReader(child.getInputStream(), StandardCharsets.UTF_8))) {
                String line;
                while ((line = reader.readLine()) != null) {
                    handleLine(line, future);
                }
            }
            int code = child.waitFor();
            if (!future.isDone()) {
                future.completeExceptionally(new IOException("Autocurl engine exited before ready with code " + code));
            }
        } catch (Exception error) {
            if (future != null && !future.isDone()) future.completeExceptionally(error);
            LOG.warn("Autocurl capture session failed", error);
        } finally {
            process = null;
            ready = null;
            starting = null;
            recording = false;
            engineStopRequested = false;
            fireStateChanged();
        }
    }

    private void handleLine(String line, CompletableFuture<ReadyEvent> future) {
        try {
            EventType type = GSON.fromJson(line, EventType.class);
            if ("ready".equals(type.type)) {
                ready = GSON.fromJson(line, ReadyEvent.class);
                recording = true;
                future.complete(ready);
                fireStateChanged();
            } else if ("request".equals(type.type)) {
                RequestEvent request = GSON.fromJson(line, RequestEvent.class);
                requests.add(request);
                ApplicationManager.getApplication().invokeLater(
                        () -> requestListeners.forEach(listener -> listener.accept(request))
                );
            } else if ("diagnostic".equals(type.type)) {
                emitLocalDiagnostic(GSON.fromJson(line, DiagnosticEvent.class));
            } else if ("state".equals(type.type)) {
                recording = GSON.fromJson(line, StateEvent.class).recording();
                fireStateChanged();
            }
        } catch (RuntimeException error) {
            LOG.warn("Ignored invalid Autocurl engine event: " + line, error);
        }
    }

    private void logErrors(Process child) {
        try (BufferedReader reader = new BufferedReader(
                new InputStreamReader(child.getErrorStream(), StandardCharsets.UTF_8))) {
            String line;
            while ((line = reader.readLine()) != null) LOG.warn(line);
        } catch (IOException ignored) {
        }
    }

    private void scheduleStartupDiagnostics(ProcessHandler handler) {
        AutocurlSettings.StateData settings = AutocurlSettings.getInstance().getState();
        int delay = Math.max(1, settings.startupDiagnosticSeconds);
        int requestCountAtStart = requests.size();
        CompletableFuture.delayedExecutor(delay, TimeUnit.SECONDS).execute(() -> {
            if (handler.isProcessTerminated() || !targetProcesses.contains(handler)) return;

            List<Integer> expectedPorts = settings.expectedListenPorts == null
                    ? List.of()
                    : List.copyOf(settings.expectedListenPorts);
            List<Integer> missingPorts = expectedPorts.stream()
                    .filter(port -> !isLocalPortListening(port))
                    .toList();
            if (!missingPorts.isEmpty()) {
                emitLocalDiagnostic(new DiagnosticEvent(
                        "diagnostic",
                        "",
                        "expected_port_not_listening",
                        "error",
                        "127.0.0.1",
                        "The application is running but an expected local port is not listening",
                        "Not listening after " + delay + " seconds: " + missingPorts,
                        "Inspect startup dependencies and the Run/Debug console. Increase the delay if this service starts slowly.",
                        "",
                        false,
                        true
                ));
                return;
            }
            if (requests.size() == requestCountAtStart) {
                emitLocalDiagnostic(new DiagnosticEvent(
                        "diagnostic",
                        "",
                        "no_proxy_traffic_observed",
                        "information",
                        "",
                        "Autocurl has not observed outbound proxy traffic yet",
                        "Trigger an outbound request. If one was already sent, that HTTP client may ignore proxy environment variables.",
                        "Use a standard client or configure its explicit proxy with the URL shown by Autocurl. Already-running processes must be restarted.",
                        "",
                        false,
                        false
                ));
            }
        });
    }

    private static boolean isLocalPortListening(int port) {
        try (Socket socket = new Socket()) {
            socket.connect(new InetSocketAddress("127.0.0.1", port), 300);
            return true;
        } catch (IOException ignored) {
            return false;
        }
    }

    public Map<String, String> mergeEnvironment(Map<String, String> existing, ReadyEvent event) {
        Map<String, String> merged = new LinkedHashMap<>(
                existing == null ? Map.of() : existing
        );
        Set<String> append = Set.copyOf(
                event.append_environment() == null ? List.of() : event.append_environment()
        );
        event.environment().forEach((name, value) -> {
            String previous = merged.get(name);
            if (append.contains(name) && previous != null && !previous.isBlank()) {
                merged.put(name, (previous + " " + value).trim());
            } else {
                merged.put(name, value);
            }
        });
        return merged;
    }

    public List<RequestEvent> requests() {
        return List.copyOf(requests);
    }

    public boolean isRunning() {
        return ready != null;
    }

    public boolean isRecording() {
        return ready != null && recording;
    }

    public boolean hasRunningTarget() {
        return targetProcesses.stream().anyMatch(handler -> !handler.isProcessTerminated());
    }

    public void expectProfile(RunProfile profile) {
        expectedProfiles.add(profile);
    }

    public void cancelExpectedProfile(RunProfile profile) {
        expectedProfiles.remove(profile);
    }

    public void setRerunAction(Runnable action) {
        rerunAction = action;
    }

    public CompletableFuture<Void> bypassAndRerun(String target) {
        AutocurlSettings.getInstance().addBypassTarget(target);
        return stopSession().thenRun(() -> {
            Runnable action = rerunAction;
            if (action != null) ApplicationManager.getApplication().invokeLater(action);
        });
    }

    public void clear() {
        requests.clear();
        fireStateChanged();
    }

    public void addRequestListener(Consumer<RequestEvent> listener) {
        requestListeners.add(listener);
    }

    public void addDiagnosticListener(Consumer<DiagnosticEvent> listener) {
        diagnosticListeners.add(listener);
    }

    public void addRenderedCurlListener(Consumer<String> listener) {
        renderedCurlListeners.add(listener);
    }

    public void showRenderedCurl(String curl) {
        ApplicationManager.getApplication().invokeLater(
                () -> renderedCurlListeners.forEach(listener -> listener.accept(curl))
        );
    }

    public void addStateListener(Runnable listener) {
        stateListeners.add(listener);
    }

    private void fireStateChanged() {
        ApplicationManager.getApplication().invokeLater(
                () -> stateListeners.forEach(Runnable::run)
        );
    }

    public synchronized void setRecording(boolean shouldRecord) {
        Process child = process;
        if (child == null || ready == null || recording == shouldRecord) return;
        String command = shouldRecord ? "resume" : "pause";
        try {
            OutputStream input = child.getOutputStream();
            input.write(("{\"command\":\"" + command + "\"}\n").getBytes(StandardCharsets.UTF_8));
            input.flush();
        } catch (IOException error) {
            LOG.warn("Could not " + command + " Autocurl recording", error);
            emitLocalDiagnostic(new DiagnosticEvent(
                    "diagnostic",
                    "",
                    "capture_control_failed",
                    "error",
                    "",
                    "Autocurl could not change the recording state",
                    rootMessage(error),
                    "Stop the session and start it again.",
                    "",
                    false,
                    true
            ));
        }
    }

    public CompletableFuture<Void> stopSession() {
        List<ProcessHandler> active = targetProcesses.stream()
                .filter(handler -> !handler.isProcessTerminated())
                .toList();
        active.forEach(ProcessHandler::destroyProcess);
        return CompletableFuture.runAsync(() -> {
            for (ProcessHandler handler : active) {
                if (!handler.waitFor(5000) && !handler.isProcessTerminated()) {
                    handler.destroyProcess();
                }
            }
            targetProcesses.removeAll(active);
            stopEngine();
        });
    }

    private synchronized void stopEngine() {
        Process child = process;
        if (child == null || engineStopRequested) return;
        engineStopRequested = true;
        try {
            child.getOutputStream().close();
        } catch (IOException error) {
            child.destroyForcibly();
            return;
        }
        ApplicationManager.getApplication().executeOnPooledThread(() -> {
            try {
                if (!child.waitFor(2500, TimeUnit.MILLISECONDS)) child.destroyForcibly();
            } catch (InterruptedException error) {
                Thread.currentThread().interrupt();
                child.destroyForcibly();
            }
        });
    }

    private void emitLocalDiagnostic(DiagnosticEvent diagnostic) {
        ApplicationManager.getApplication().invokeLater(
                () -> diagnosticListeners.forEach(listener -> listener.accept(diagnostic))
        );
    }

    @Override
    public void dispose() {
        stopSession();
    }

    private static String rootMessage(Throwable error) {
        Throwable current = error;
        while (current.getCause() != null) current = current.getCause();
        return current.getMessage() == null ? current.toString() : current.getMessage();
    }

    private static final class EventType {
        String type;
    }
}
