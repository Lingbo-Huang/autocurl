package com.github.lingbohuang.autocurl;

import com.google.gson.Gson;
import com.intellij.openapi.Disposable;
import com.intellij.openapi.application.ApplicationManager;
import com.intellij.openapi.components.Service;
import com.intellij.openapi.diagnostic.Logger;
import com.intellij.openapi.project.Project;

import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.CompletableFuture;
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
            List<String> notes
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

    private static final Logger LOG = Logger.getInstance(CaptureSession.class);
    private static final Gson GSON = new Gson();
    private final List<RequestEvent> requests = new CopyOnWriteArrayList<>();
    private final List<Consumer<RequestEvent>> requestListeners = new CopyOnWriteArrayList<>();
    private final List<Runnable> stateListeners = new CopyOnWriteArrayList<>();
    private volatile Process process;
    private volatile ReadyEvent ready;
    private volatile CompletableFuture<ReadyEvent> starting;

    public static CaptureSession getInstance(Project project) {
        return project.getService(CaptureSession.class);
    }

    public synchronized CompletableFuture<ReadyEvent> start() {
        if (ready != null) return CompletableFuture.completedFuture(ready);
        if (starting != null) return starting;
        starting = new CompletableFuture<>();
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
            if (!settings.match.isBlank()) command.addAll(List.of("--match", settings.match));
            if (!settings.method.isBlank()) command.addAll(List.of("--method", settings.method));
            command.addAll(List.of("--max-body", String.valueOf(settings.maxBodyBytes)));
            if (settings.showSecrets) command.add("--show-secrets");
            settings.replayHeaders.forEach(header -> command.addAll(List.of("--replay-header", header)));
            settings.liveHeaders.forEach(header -> command.addAll(List.of("--live-header", header)));

            requests.clear();
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
            fireStateChanged();
        }
    }

    private void handleLine(String line, CompletableFuture<ReadyEvent> future) {
        try {
            EventType type = GSON.fromJson(line, EventType.class);
            if ("ready".equals(type.type)) {
                ready = GSON.fromJson(line, ReadyEvent.class);
                future.complete(ready);
                fireStateChanged();
            } else if ("request".equals(type.type)) {
                RequestEvent request = GSON.fromJson(line, RequestEvent.class);
                requests.add(request);
                ApplicationManager.getApplication().invokeLater(
                        () -> requestListeners.forEach(listener -> listener.accept(request))
                );
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

    public void clear() {
        requests.clear();
        fireStateChanged();
    }

    public void addRequestListener(Consumer<RequestEvent> listener) {
        requestListeners.add(listener);
    }

    public void addStateListener(Runnable listener) {
        stateListeners.add(listener);
    }

    private void fireStateChanged() {
        ApplicationManager.getApplication().invokeLater(
                () -> stateListeners.forEach(Runnable::run)
        );
    }

    public synchronized void stop() {
        Process child = process;
        if (child == null) return;
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

    @Override
    public void dispose() {
        stop();
    }

    private static final class EventType {
        String type;
    }
}
