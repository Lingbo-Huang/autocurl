package com.github.lingbohuang.autocurl;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

class RunConfigurationEnvironmentTest {
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
