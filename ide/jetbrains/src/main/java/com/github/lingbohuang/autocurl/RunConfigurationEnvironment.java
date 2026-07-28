package com.github.lingbohuang.autocurl;

import com.intellij.execution.CommonProgramRunConfigurationParameters;
import com.intellij.execution.configurations.RunConfiguration;
import com.intellij.openapi.diagnostic.Logger;

import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

final class RunConfigurationEnvironment {
    private static final Logger LOG = Logger.getInstance(RunConfigurationEnvironment.class);
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
                && injectGoBuildOverlay(configuration, environment.get("GOFLAGS"));
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
