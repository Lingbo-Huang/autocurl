package com.github.lingbohuang.autocurl;

import com.google.gson.Gson;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import com.intellij.execution.CommonProgramRunConfigurationParameters;
import com.intellij.execution.configurations.RunConfiguration;
import com.intellij.openapi.diagnostic.Logger;

import java.io.IOException;
import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;
import java.nio.charset.StandardCharsets;
import java.nio.file.AtomicMoveNotSupportedException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

final class RunConfigurationEnvironment {
    private static final Logger LOG = Logger.getInstance(RunConfigurationEnvironment.class);
    private static final Gson GSON = new Gson();
    private static final Pattern GO_OVERLAY_FLAG = Pattern.compile(
            "(?:^|\\s)(-overlay=(?:\"(?:\\\\.|[^\"])*\"|'[^']*'|\\S+))"
    );
    private static final Accessor[] REFLECTIVE_ACCESSORS = {
            new Accessor("getCustomEnvironment", "setCustomEnvironment"),
            new Accessor("getEnvs", "setEnvs")
    };

    private RunConfigurationEnvironment() {}

    static boolean supports(RunConfiguration configuration) {
        if (configuration instanceof CommonProgramRunConfigurationParameters) return true;
        for (Accessor accessor : REFLECTIVE_ACCESSORS) {
            if (accessor.supports(configuration.getClass())) return true;
        }
        return false;
    }

    static Map<String, String> read(RunConfiguration configuration) {
        if (configuration instanceof CommonProgramRunConfigurationParameters parameters) {
            return copy(parameters.getEnvs());
        }
        for (Accessor accessor : REFLECTIVE_ACCESSORS) {
            Map<String, String> environment = accessor.read(configuration);
            if (environment != null) return environment;
        }
        return Map.of();
    }

    static boolean inject(RunConfiguration configuration, Map<String, String> environment) {
        boolean environmentInjected = false;
        if (configuration instanceof CommonProgramRunConfigurationParameters parameters) {
            parameters.setEnvs(new LinkedHashMap<>(environment));
            environmentInjected = true;
        } else {
            for (Accessor accessor : REFLECTIVE_ACCESSORS) {
                if (accessor.write(configuration, environment)) {
                    environmentInjected = true;
                    break;
                }
            }
        }
        return environmentInjected
                && alignGoOverlayWithSdk(configuration, environment.get("GOFLAGS"))
                && injectGoBuildOverlay(configuration, environment.get("GOFLAGS"));
    }

    static Map<String, String> mergeCaptureEnvironment(
            Map<String, String> existing,
            Map<String, String> overrides,
            Set<String> appendNames
    ) {
        Map<String, String> merged = new LinkedHashMap<>(
                existing == null ? Map.of() : existing
        );
        Set<String> append = appendNames == null ? Set.of() : appendNames;
        overrides.forEach((name, value) -> {
            if (isBypassVariable(name)) return;
            String previous = merged.get(name);
            if (append.contains(name) && previous != null && !previous.isBlank()) {
                merged.put(name, (previous + " " + value).trim());
            } else {
                merged.put(name, value);
            }
        });

        boolean hasBypassOverride = overrides.keySet().stream()
                .anyMatch(RunConfigurationEnvironment::isBypassVariable);
        if (hasBypassOverride) {
            LinkedHashSet<String> targets = new LinkedHashSet<>();
            for (String name : List.of("NO_PROXY", "no_proxy", "no_grpc_proxy")) {
                addCommaSeparated(targets, existing == null ? null : existing.get(name));
            }
            for (String name : List.of("NO_PROXY", "no_proxy", "no_grpc_proxy")) {
                addCommaSeparated(targets, overrides.get(name));
            }
            String bypass = String.join(",", targets);
            merged.put("NO_PROXY", bypass);
            merged.put("no_proxy", bypass);
            merged.put("no_grpc_proxy", bypass);
        }
        return merged;
    }

    private static boolean isBypassVariable(String name) {
        return "NO_PROXY".equals(name) || "no_proxy".equals(name) ||
                "no_grpc_proxy".equals(name);
    }

    private static void addCommaSeparated(Set<String> destination, String value) {
        if (value == null || value.isBlank()) return;
        for (String target : value.split(",")) {
            String normalized = target.trim();
            if (!normalized.isEmpty()) destination.add(normalized);
        }
    }

    private static boolean alignGoOverlayWithSdk(Object configuration, String goFlags) {
        String sdkHome = resolveGoSdkHome(configuration);
        if (sdkHome == null) return true;
        return alignGoOverlayWithSdk(goFlags, sdkHome, System.getProperty("os.name"));
    }

    static boolean alignGoOverlayWithSdk(String goFlags, String sdkHome, String osName) {
        String overlayFlag = extractGoOverlayFlag(goFlags);
        if (overlayFlag == null || sdkHome == null || sdkHome.isBlank()) return true;

        String platform = goPlatform(osName);
        if (platform == null) return true;
        Path overlayPath = Path.of(unquote(overlayFlag.substring("-overlay=".length())));
        Path sdkTarget = Path.of(
                sdkHome,
                "src",
                "crypto",
                "x509",
                "root_" + platform + ".go"
        );
        if (!Files.isRegularFile(sdkTarget)) {
            LOG.warn("Autocurl could not find the Go SDK x509 source at " + sdkTarget);
            return false;
        }

        Path staging = null;
        try {
            JsonObject root = JsonParser.parseString(
                    Files.readString(overlayPath, StandardCharsets.UTF_8)
            ).getAsJsonObject();
            JsonObject replacements = root.getAsJsonObject("Replace");
            if (replacements == null) {
                LOG.warn("Autocurl Go overlay has no Replace map: " + overlayPath);
                return false;
            }
            String target = sdkTarget.toString();
            if (replacements.has(target)) return true;

            String replacement = findPlatformRootReplacement(
                    replacements,
                    "root_" + platform + ".go"
            );
            if (replacement == null) {
                LOG.warn("Autocurl Go overlay has no platform root replacement: " + overlayPath);
                return false;
            }
            replacements.addProperty(target, replacement);

            Path parent = overlayPath.toAbsolutePath().getParent();
            staging = Files.createTempFile(parent, "autocurl-go-overlay-", ".json");
            Files.writeString(staging, GSON.toJson(root), StandardCharsets.UTF_8);
            moveReplacing(staging, overlayPath);
            staging = null;
            return true;
        } catch (IOException | RuntimeException error) {
            LOG.warn("Could not align the Autocurl Go overlay with SDK " + sdkHome, error);
            return false;
        } finally {
            if (staging != null) {
                try {
                    Files.deleteIfExists(staging);
                } catch (IOException ignored) {
                }
            }
        }
    }

    static boolean injectGoBuildOverlay(Object configuration, String goFlags) {
        String overlayFlag = extractGoOverlayFlag(goFlags);
        if (overlayFlag == null) return true;

        Method getter;
        Method setter;
        try {
            getter = configuration.getClass().getMethod("getGoToolParams");
            setter = configuration.getClass().getMethod("setGoToolParams", String.class);
        } catch (NoSuchMethodException ignored) {
            return true;
        }

        try {
            Object currentValue = getter.invoke(configuration);
            String currentParams = currentValue == null ? "" : currentValue.toString().trim();
            if (containsGoOverlayFlag(currentParams, overlayFlag)) return true;
            setter.invoke(configuration, appendParameter(currentParams, overlayFlag));
            return true;
        } catch (IllegalAccessException | InvocationTargetException error) {
            LOG.warn("Could not inject the Autocurl Go build overlay", error);
            return false;
        }
    }

    static String extractGoOverlayFlag(String flags) {
        if (flags == null || flags.isBlank()) return null;
        Matcher matcher = GO_OVERLAY_FLAG.matcher(flags);
        return matcher.find() ? matcher.group(1) : null;
    }

    private static String resolveGoSdkHome(Object configuration) {
        try {
            Method getConfigurationModule = configuration.getClass()
                    .getMethod("getConfigurationModule");
            Object configurationModule = getConfigurationModule.invoke(configuration);
            if (configurationModule == null) return null;
            Object module = configurationModule.getClass().getMethod("getModule")
                    .invoke(configurationModule);
            if (module == null) return null;

            Object project = configuration.getClass().getMethod("getProject")
                    .invoke(configuration);
            ClassLoader goClassLoader = configuration.getClass().getClassLoader();
            Class<?> projectClass = goClassLoader.loadClass(
                    "com.intellij.openapi.project.Project"
            );
            Class<?> moduleClass = goClassLoader.loadClass(
                    "com.intellij.openapi.module.Module"
            );
            Class<?> serviceClass = goClassLoader.loadClass("com.goide.sdk.GoSdkService");
            Object service = serviceClass.getMethod("getInstance", projectClass)
                    .invoke(null, project);
            Object sdk = serviceClass.getMethod("getSdk", moduleClass)
                    .invoke(service, module);
            if (sdk == null) return null;
            Class<?> sdkClass = goClassLoader.loadClass("com.goide.sdk.GoSdk");
            Object home = sdkClass.getMethod("getHomePath").invoke(sdk);
            return home == null ? null : home.toString();
        } catch (ClassNotFoundException | NoSuchMethodException ignored) {
            return null;
        } catch (IllegalAccessException | InvocationTargetException error) {
            LOG.warn("Could not read the Go SDK home from the Run Configuration", error);
            return null;
        }
    }

    private static String findPlatformRootReplacement(
            JsonObject replacements,
            String sourceFileName
    ) {
        for (Map.Entry<String, JsonElement> entry : replacements.entrySet()) {
            if (Path.of(entry.getKey()).getFileName().toString().equals(sourceFileName)
                    && entry.getValue().isJsonPrimitive()) {
                return entry.getValue().getAsString();
            }
        }
        return null;
    }

    private static String goPlatform(String osName) {
        if (osName == null) return null;
        String normalized = osName.toLowerCase();
        if (normalized.contains("mac") || normalized.contains("darwin")) return "darwin";
        if (normalized.contains("win")) return "windows";
        return null;
    }

    private static String unquote(String value) {
        if (value.length() < 2) return value;
        char first = value.charAt(0);
        char last = value.charAt(value.length() - 1);
        if ((first == '"' && last == '"') || (first == '\'' && last == '\'')) {
            return value.substring(1, value.length() - 1);
        }
        return value;
    }

    private static void moveReplacing(Path source, Path target) throws IOException {
        try {
            Files.move(
                    source,
                    target,
                    StandardCopyOption.ATOMIC_MOVE,
                    StandardCopyOption.REPLACE_EXISTING
            );
        } catch (AtomicMoveNotSupportedException ignored) {
            Files.move(source, target, StandardCopyOption.REPLACE_EXISTING);
        }
    }

    private static boolean containsGoOverlayFlag(String currentParams, String overlayFlag) {
        Matcher matcher = GO_OVERLAY_FLAG.matcher(currentParams);
        while (matcher.find()) {
            if (matcher.group(1).equals(overlayFlag)) return true;
        }
        return false;
    }

    private static String appendParameter(String currentParams, String parameter) {
        return currentParams.isBlank() ? parameter : currentParams + " " + parameter;
    }

    private static Map<String, String> copy(Map<String, String> environment) {
        return new LinkedHashMap<>(environment == null ? Map.of() : environment);
    }

    private record Accessor(String getterName, String setterName) {
        boolean supports(Class<?> configurationClass) {
            try {
                Method getter = configurationClass.getMethod(getterName);
                Method setter = configurationClass.getMethod(setterName, Map.class);
                return Map.class.isAssignableFrom(getter.getReturnType())
                        && setter.getReturnType() == Void.TYPE;
            } catch (NoSuchMethodException ignored) {
                return false;
            }
        }

        Map<String, String> read(Object configuration) {
            if (!supports(configuration.getClass())) return null;
            try {
                Object value = configuration.getClass().getMethod(getterName).invoke(configuration);
                if (!(value instanceof Map<?, ?> values)) return new LinkedHashMap<>();
                Map<String, String> result = new LinkedHashMap<>();
                values.forEach((key, item) -> {
                    if (key != null && item != null) result.put(key.toString(), item.toString());
                });
                return result;
            } catch (IllegalAccessException | InvocationTargetException | NoSuchMethodException error) {
                LOG.warn("Could not read environment via " + getterName, error);
                return null;
            }
        }

        boolean write(Object configuration, Map<String, String> environment) {
            if (!supports(configuration.getClass())) return false;
            try {
                configuration.getClass()
                        .getMethod(setterName, Map.class)
                        .invoke(configuration, new LinkedHashMap<>(environment));
                return true;
            } catch (IllegalAccessException | InvocationTargetException | NoSuchMethodException error) {
                LOG.warn("Could not write environment via " + setterName, error);
                return false;
            }
        }
    }
}
