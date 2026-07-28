package com.github.lingbohuang.autocurl;

import com.google.gson.Gson;
import com.intellij.openapi.application.PathManager;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.attribute.PosixFilePermission;
import java.security.MessageDigest;
import java.time.Duration;
import java.util.Comparator;
import java.util.HexFormat;
import java.util.List;
import java.util.Set;
import java.util.concurrent.TimeUnit;
import java.util.stream.Stream;

public final class EngineManager {
    private static final String REPOSITORY = "Lingbo-Huang/autocurl";
    // Keep this aligned with the plugin version. A merely API-compatible older
    // engine can still miss runtime fixes such as platform-specific Go CA trust.
    private static final String MINIMUM_VERSION = "0.2.2";
    private static final Gson GSON = new Gson();
    private static final HttpClient HTTP = HttpClient.newBuilder()
            .followRedirects(HttpClient.Redirect.ALWAYS)
            .connectTimeout(Duration.ofSeconds(20))
            .build();

    private record Asset(String name, String browser_download_url) {}
    private record Release(String tag_name, List<Asset> assets) {}

    private EngineManager() {}

    public static String resolve() throws Exception {
        AutocurlSettings.StateData settings = AutocurlSettings.getInstance().getState();
        if (!settings.binaryPath.isBlank()) {
            requireCompatible(settings.binaryPath);
            return settings.binaryPath;
        }
        Path managed = managedBinary();
        if (isCompatible(managed.toString())) {
            return managed.toString();
        }
        if (isCompatible("autocurl")) {
            return "autocurl";
        }
        if (!settings.autoDownload) {
            throw new IOException("No compatible autocurl engine found. Configure its path in Settings | Tools | Autocurl.");
        }
        return downloadLatest().toString();
    }

    private static boolean isCompatible(String executable) {
        try {
            requireCompatible(executable);
            return true;
        } catch (Exception ignored) {
            return false;
        }
    }

    private static void requireCompatible(String executable) throws Exception {
        Process process = new ProcessBuilder(executable, "version").start();
        if (!process.waitFor(5, TimeUnit.SECONDS)) {
            process.destroyForcibly();
            throw new IOException("Timed out checking " + executable);
        }
        String version = new String(process.getInputStream().readAllBytes(), StandardCharsets.UTF_8)
                .trim()
                .replaceFirst("^v", "");
        if (process.exitValue() != 0 || compareVersions(version, MINIMUM_VERSION) < 0) {
            throw new IOException("Autocurl " + MINIMUM_VERSION + " or newer is required; found " + version);
        }
    }

    private static Path downloadLatest() throws Exception {
        Release release = GSON.fromJson(
                new String(download("https://api.github.com/repos/" + REPOSITORY + "/releases/latest", true),
                        StandardCharsets.UTF_8),
                Release.class
        );
        String version = release.tag_name().replaceFirst("^v", "");
        if (compareVersions(version, MINIMUM_VERSION) < 0) {
            throw new IOException("Latest GitHub release is " + release.tag_name()
                    + ", but this plugin requires v" + MINIMUM_VERSION + " or newer.");
        }
        String platform = platformName();
        String architecture = architectureName();
        String suffix = isWindows() ? ".zip" : ".tar.gz";
        String assetName = "autocurl-" + release.tag_name() + "-" + platform + "-" + architecture + suffix;
        Asset archiveAsset = release.assets().stream()
                .filter(asset -> asset.name().equals(assetName))
                .findFirst()
                .orElseThrow(() -> new IOException("No release asset for " + platform + "/" + architecture));
        Asset sumsAsset = release.assets().stream()
                .filter(asset -> asset.name().equals("SHA256SUMS"))
                .findFirst()
                .orElseThrow(() -> new IOException("Release has no SHA256SUMS"));

        byte[] archive = download(archiveAsset.browser_download_url(), false);
        String sums = new String(download(sumsAsset.browser_download_url(), false), StandardCharsets.UTF_8);
        String expected = checksumFor(sums, assetName);
        String actual = HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(archive));
        if (expected == null || !actual.equalsIgnoreCase(expected)) {
            throw new IOException("SHA-256 verification failed for " + assetName);
        }

        Path storage = managedBinary().getParent().getParent();
        Files.createDirectories(storage);
        Path staging = Files.createTempDirectory(storage, "staging-");
        try {
            Path archivePath = staging.resolve(assetName);
            Files.write(archivePath, archive);
            Process extraction = new ProcessBuilder(
                    isWindows() ? "tar.exe" : "tar",
                    "-xf", archivePath.toString(), "-C", staging.toString()
            ).start();
            if (!extraction.waitFor(30, TimeUnit.SECONDS) || extraction.exitValue() != 0) {
                extraction.destroyForcibly();
                throw new IOException("Could not extract " + assetName);
            }
            String wanted = isWindows() ? "autocurl.exe" : "autocurl";
            Path extracted;
            try (Stream<Path> paths = Files.walk(staging)) {
                extracted = paths.filter(path -> path.getFileName().toString().equals(wanted))
                        .findFirst()
                        .orElseThrow(() -> new IOException("Archive contains no " + wanted));
            }
            Path managed = managedBinary();
            Files.createDirectories(managed.getParent());
            Files.copy(extracted, managed, java.nio.file.StandardCopyOption.REPLACE_EXISTING);
            if (!isWindows()) {
                Files.setPosixFilePermissions(managed, Set.of(
                        PosixFilePermission.OWNER_READ,
                        PosixFilePermission.OWNER_WRITE,
                        PosixFilePermission.OWNER_EXECUTE,
                        PosixFilePermission.GROUP_READ,
                        PosixFilePermission.GROUP_EXECUTE,
                        PosixFilePermission.OTHERS_READ,
                        PosixFilePermission.OTHERS_EXECUTE
                ));
            }
            requireCompatible(managed.toString());
            return managed;
        } finally {
            deleteTree(staging);
        }
    }

    private static byte[] download(String url, boolean githubJson) throws Exception {
        HttpRequest request = HttpRequest.newBuilder(URI.create(url))
                .header("Accept", githubJson ? "application/vnd.github+json" : "application/octet-stream")
                .header("User-Agent", "autocurl-jetbrains-plugin")
                .header("X-GitHub-Api-Version", "2022-11-28")
                .timeout(Duration.ofSeconds(60))
                .GET()
                .build();
        HttpResponse<byte[]> response = HTTP.send(request, HttpResponse.BodyHandlers.ofByteArray());
        if (response.statusCode() != 200) {
            throw new IOException("Download failed with HTTP " + response.statusCode() + ": " + url);
        }
        return response.body();
    }

    private static Path managedBinary() {
        return Path.of(
                PathManager.getSystemPath(),
                "autocurl",
                "bin",
                isWindows() ? "autocurl.exe" : "autocurl"
        );
    }

    private static String platformName() throws IOException {
        String os = System.getProperty("os.name").toLowerCase();
        if (os.contains("mac")) return "darwin";
        if (os.contains("linux")) return "linux";
        if (os.contains("win")) return "windows";
        throw new IOException("Unsupported operating system: " + os);
    }

    private static String architectureName() throws IOException {
        String architecture = System.getProperty("os.arch").toLowerCase();
        if (architecture.equals("x86_64") || architecture.equals("amd64")) return "amd64";
        if (architecture.equals("aarch64") || architecture.equals("arm64")) return "arm64";
        throw new IOException("Unsupported CPU architecture: " + architecture);
    }

    private static boolean isWindows() {
        return System.getProperty("os.name").toLowerCase().contains("win");
    }

    private static int compareVersions(String left, String right) {
        String[] a = left.split("[.-]");
        String[] b = right.split("[.-]");
        for (int index = 0; index < 3; index++) {
            int av = index < a.length ? parseNumber(a[index]) : 0;
            int bv = index < b.length ? parseNumber(b[index]) : 0;
            if (av != bv) return Integer.compare(av, bv);
        }
        return 0;
    }

    private static int parseNumber(String value) {
        try {
            return Integer.parseInt(value);
        } catch (NumberFormatException ignored) {
            return 0;
        }
    }

    private static String checksumFor(String contents, String assetName) {
        for (String line : contents.split("\\R")) {
            String[] parts = line.trim().split("\\s+", 2);
            if (parts.length == 2 && parts[1].replaceFirst("^\\*", "").equals(assetName)) {
                return parts[0].toLowerCase();
            }
        }
        return null;
    }

    private static void deleteTree(Path root) {
        if (!Files.exists(root)) return;
        try (Stream<Path> paths = Files.walk(root)) {
            paths.sorted(Comparator.reverseOrder()).forEach(path -> {
                try {
                    Files.deleteIfExists(path);
                } catch (IOException ignored) {
                }
            });
        } catch (IOException ignored) {
        }
    }
}
