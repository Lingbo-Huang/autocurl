plugins {
    java
    id("org.jetbrains.intellij.platform") version "2.18.1"
}

group = providers.gradleProperty("pluginGroup").get()
version = providers.gradleProperty("pluginVersion").get()

repositories {
    mavenCentral()
    intellijPlatform {
        defaultRepositories()
    }
}

dependencies {
    implementation("com.google.code.gson:gson:2.11.0")
    intellijPlatform {
        val localIde = providers.environmentVariable("AUTOCURL_LOCAL_IDE")
        if (localIde.isPresent) {
            local(localIde.get())
        } else {
            intellijIdea(providers.gradleProperty("platformVersion"))
        }
    }
}

java {
    sourceCompatibility = JavaVersion.VERSION_21
    targetCompatibility = JavaVersion.VERSION_21
}

tasks.withType<JavaCompile>().configureEach {
    options.release = 21
}

intellijPlatform {
    pluginConfiguration {
        name = providers.gradleProperty("pluginName")
        version = providers.gradleProperty("pluginVersion")
        ideaVersion {
            sinceBuild = providers.gradleProperty("platformSinceBuild")
        }
        description = """
            Capture outbound HTTP, HTTP/2, gRPC, and WebSocket handshake requests
            from JetBrains Run/Debug configurations and copy complete replayable cURLs.
        """.trimIndent()
        changeNotes = """
            <ul>
              <li>Run or debug the selected configuration with process-scoped capture.</li>
              <li>Inspect captured requests in the Autocurl tool window.</li>
              <li>Copy a complete redacted cURL with one click.</li>
              <li>Download and verify the matching Go engine from GitHub Releases.</li>
            </ul>
        """.trimIndent()
        vendor {
            name = "Huang Lingbo"
            url = "https://github.com/Lingbo-Huang/autocurl"
        }
    }
    publishing {
        token = providers.environmentVariable("PUBLISH_TOKEN")
    }
}

tasks {
    wrapper {
        gradleVersion = "9.0.0"
    }
}
