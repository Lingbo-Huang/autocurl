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
    testImplementation("org.junit.jupiter:junit-jupiter:5.11.4")
    testRuntimeOnly("junit:junit:4.13.2")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
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
            <p>
              Autocurl captures outbound requests from JetBrains Run/Debug configurations
              and turns them into complete replayable cURLs.
            </p>

            <h2>快速上手</h2>
            <ol>
              <li>在 IDE 顶部选择一个已有的 Run/Debug Configuration。</li>
              <li>打开 <b>Run</b> 菜单，点击 <b>Run Selected with Autocurl</b>
                  或 <b>Debug Selected with Autocurl</b>。</li>
              <li>插件会自动打开右侧的 <b>Autocurl</b> 工具窗口。</li>
              <li>程序发出请求后，在列表中选择请求，点击 <b>Copy cURL</b>。</li>
            </ol>
            <p>
              <b>断点调试：</b>如果请求还没有真正发出，可在编辑器中选中请求 JSON，
              然后点击 <b>Tools → Generate cURL from Request JSON</b>，
              插件会直接生成并复制 cURL，不会发送网络请求。
            </p>
            <p>
              <b>没有看到请求？</b>请确认是通过 Autocurl 的 Run/Debug 菜单启动，
              而不是 IDE 原来的运行按钮。引擎设置位于
              <b>Settings → Tools → Autocurl</b>。
            </p>
            <p>
              <b>mTLS / etcd：</b>在 <b>Settings → Tools → Autocurl →
              Bypass capture</b> 中配置必须直连的域名、IP 或 CIDR。
            </p>
            <p>
              插件只为本次运行注入进程级代理和临时证书，不修改系统代理，
              也不会永久修改原 Run/Debug Configuration。
            </p>
            <p>
              <a href="https://github.com/Lingbo-Huang/autocurl/blob/main/docs/ide-plugins.zh-CN.md">
                中文完整使用指南
              </a>
              ·
              <a href="https://github.com/Lingbo-Huang/autocurl/issues">
                反馈问题
              </a>
            </p>

            <hr>

            <h2>Quick start</h2>
            <ol>
              <li>Select an existing Run/Debug Configuration.</li>
              <li>Choose <b>Run → Run Selected with Autocurl</b> or
                  <b>Debug Selected with Autocurl</b>.</li>
              <li>Open the <b>Autocurl</b> tool window.</li>
              <li>Select a captured request and click <b>Copy cURL</b>.</li>
            </ol>
            <p>
              At a breakpoint before the request is sent, copy request JSON from the
              debugger and choose <b>Tools → Generate cURL from Request JSON</b>.
            </p>
        """.trimIndent()
        changeNotes = """
            <h3>0.3.1</h3>
            <ul>
              <li>Kept the JetBrains adapter and managed engine aligned with the 0.3.1 release.</li>
            </ul>

            <h3>0.3.0</h3>
            <ul>
              <li>Added Safe and Strict Capture modes with actionable TLS and mTLS diagnostics.</li>
              <li>Added Pause Recording without interrupting application traffic.</li>
              <li>Made Stop Session terminate the associated Run/Debug process before closing the proxy.</li>
              <li>Kept Clear independent from process and network lifecycle.</li>
              <li>Added cURL generation from editor JSON, JSON files, clipboard, and common Go, Java, Python, Axios, and Fetch request shapes.</li>
              <li>Added startup diagnostics for launch failures, early exits, missing listen ports, and HTTP clients that ignore proxy variables.</li>
            </ul>

            <h3>0.2.4</h3>
            <ul>
              <li>Preserved existing NO_PROXY, no_proxy, and no_grpc_proxy values instead of clearing them.</li>
              <li>Added configurable bypass targets for mTLS, etcd, certificate-pinned, and direct infrastructure calls.</li>
              <li>Applied bypass targets consistently to Go, Python, Node.js, gRPC, and Java proxy settings.</li>
              <li>Fixed service startup hangs caused by forcing mTLS dependencies through TLS interception.</li>
            </ul>

            <h3>0.2.3</h3>
            <ul>
              <li>Fixed Go HTTPS capture when JetBrains is launched from the macOS GUI and its engine process cannot find the configured Go SDK on PATH.</li>
              <li>Fixed GoLand builds by passing the temporary Autocurl overlay to the Go compiler as well as the launched process.</li>
              <li>Aligned the overlay with GoLand's configured GOROOT even when Homebrew exposes the same SDK through different symlink and Cellar paths.</li>
              <li>Added Go SDK discovery through GOROOT, standard installation paths, and the user's login shell.</li>
              <li>Added regression tests for macOS GUI Go discovery and JetBrains Go build parameter injection.</li>
            </ul>

            <h3>0.2.2</h3>
            <ul>
              <li>Fixed Go HTTPS capture on macOS by requiring the matching engine version.</li>
              <li>Added JetBrains Marketplace signing, custom plugin icons, and automated marketplace publishing.</li>
              <li>Added release version guards for the engine and both IDE plugins.</li>
            </ul>

            <h3>0.2.1</h3>
            <ul>
              <li>Added a bilingual quick-start guide to the plugin overview.</li>
              <li>Added first-use guidance and a permanent Quick Start action.</li>
              <li>Added instructions to the empty Autocurl tool window.</li>
              <li>Fixed environment injection for Go Application and Go Test configurations.</li>
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
    signing {
        certificateChainFile = layout.file(
            providers.environmentVariable("CERTIFICATE_CHAIN_FILE").map { file(it) }
        )
        privateKeyFile = layout.file(
            providers.environmentVariable("PRIVATE_KEY_FILE").map { file(it) }
        )
        password = providers.environmentVariable("PRIVATE_KEY_PASSWORD")
    }
}

tasks {
    test {
        useJUnitPlatform()
    }
    named("verifyPluginSignature") {
        dependsOn("signPlugin")
    }
    wrapper {
        gradleVersion = "9.0.0"
    }
}
