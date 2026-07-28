package com.github.lingbohuang.autocurl;

import com.google.gson.Gson;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

class RunConfigurationEnvironmentTest {
    private static final Gson GSON = new Gson();

    @Test
    void extractsOnlyTheOverlayFlagFromGoFlags() {
        assertEquals(
                "-overlay=/tmp/autocurl-go-overlay.json",
                RunConfigurationEnvironment.extractGoOverlayFlag(
                        "-mod=readonly -overlay=/tmp/autocurl-go-overlay.json -tags=integration"
                )
        );
        assertEquals(
                "-overlay=\"/tmp/path with spaces/autocurl-go-overlay.json\"",
                RunConfigurationEnvironment.extractGoOverlayFlag(
                        "-overlay=\"/tmp/path with spaces/autocurl-go-overlay.json\""
                )
        );
        assertNull(RunConfigurationEnvironment.extractGoOverlayFlag("-mod=readonly"));
    }

    @Test
    void appendsTheOverlayToGoBuildParameters() {
        FakeGoConfiguration configuration = new FakeGoConfiguration("-race");

        assertTrue(RunConfigurationEnvironment.injectGoBuildOverlay(
                configuration,
                "-mod=readonly -overlay=/tmp/autocurl-go-overlay.json"
        ));

        assertEquals(
                "-race -overlay=/tmp/autocurl-go-overlay.json",
                configuration.getGoToolParams()
        );
    }

    @Test
    void doesNotAppendTheSameOverlayTwice() {
        FakeGoConfiguration configuration = new FakeGoConfiguration(
                "-race -overlay=/tmp/autocurl-go-overlay.json"
        );

        assertTrue(RunConfigurationEnvironment.injectGoBuildOverlay(
                configuration,
                "-overlay=/tmp/autocurl-go-overlay.json"
        ));

        assertEquals(
                "-race -overlay=/tmp/autocurl-go-overlay.json",
                configuration.getGoToolParams()
        );
    }

    @Test
    void leavesNonGoConfigurationsUntouched() {
        assertTrue(RunConfigurationEnvironment.injectGoBuildOverlay(
                new Object(),
                "-overlay=/tmp/autocurl-go-overlay.json"
        ));
    }

    @Test
    void bypassTargetsAreTrimmedAndDeduplicated() {
        AutocurlSettings settings = new AutocurlSettings();

        assertTrue(settings.addBypassTarget(" api.internal.example "));
        assertFalse(settings.addBypassTarget("api.internal.example"));
        assertFalse(settings.addBypassTarget(" "));
        assertEquals(
                java.util.List.of("api.internal.example"),
                settings.getState().bypassTargets
        );
    }

    @Test
    void addsTheConfiguredGoSdkAliasToTheOverlay(@TempDir Path tempDirectory) throws Exception {
        Path discoveredGoRoot = tempDirectory.resolve("Cellar/go/1.24.3/libexec");
        Path configuredGoRoot = tempDirectory.resolve("opt/go/libexec");
        Path source = discoveredGoRoot.resolve("src/crypto/x509/root_darwin.go");
        Path configuredSource = configuredGoRoot.resolve("src/crypto/x509/root_darwin.go");
        Path replacement = tempDirectory.resolve("autocurl-go-roots.go");
        Path overlay = tempDirectory.resolve("autocurl-go-overlay.json");
        Files.createDirectories(source.getParent());
        Files.createDirectories(configuredSource.getParent());
        Files.writeString(source, "package x509", StandardCharsets.UTF_8);
        Files.writeString(configuredSource, "package x509", StandardCharsets.UTF_8);
        Files.writeString(replacement, "package x509", StandardCharsets.UTF_8);
        Files.writeString(
                overlay,
                GSON.toJson(Map.of(
                        "Replace",
                        Map.of(source.toString(), replacement.toString())
                )),
                StandardCharsets.UTF_8
        );

        assertTrue(RunConfigurationEnvironment.alignGoOverlayWithSdk(
                "-overlay=" + overlay,
                configuredGoRoot.toString(),
                "Mac OS X"
        ));

        JsonObject replacements = JsonParser.parseString(
                Files.readString(overlay, StandardCharsets.UTF_8)
        ).getAsJsonObject().getAsJsonObject("Replace");
        assertEquals(replacement.toString(), replacements.get(source.toString()).getAsString());
        assertEquals(
                replacement.toString(),
                replacements.get(configuredSource.toString()).getAsString()
        );
    }

    public static final class FakeGoConfiguration {
        private String goToolParams;

        FakeGoConfiguration(String goToolParams) {
            this.goToolParams = goToolParams;
        }

        public String getGoToolParams() {
            return goToolParams;
        }

        public void setGoToolParams(String goToolParams) {
            this.goToolParams = goToolParams;
        }
    }
}
