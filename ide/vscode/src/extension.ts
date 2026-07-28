import { spawn } from "node:child_process";
import * as vscode from "vscode";
import { BinaryManager } from "./binary";
import { CaptureSession, DiagnosticEvent, RequestEvent } from "./session";

class RequestItem extends vscode.TreeItem {
  constructor(readonly request: RequestEvent) {
    let path = request.url;
    try {
      const url = new URL(request.url);
      path = `${url.host}${url.pathname}${url.search}`;
    } catch {
      // Keep the engine-provided display URL.
    }
    super(`${request.method} ${path}`, vscode.TreeItemCollapsibleState.None);
    const status = request.grpc_status
      ? `gRPC ${request.grpc_status}`
      : request.status
        ? String(request.status)
        : request.error
          ? "error"
          : "";
    this.description = [status, `${request.duration_ms} ms`].filter(Boolean).join(" · ");
    this.tooltip = new vscode.MarkdownString(
      [
        `**${request.method} ${request.url}**`,
        "",
        `${request.kind} · client ${request.protocol ?? "unknown"} · upstream ${request.upstream_protocol ?? "unknown"}`,
        request.warning ? `\n⚠️ ${request.warning}` : "",
        request.redacted ? "\nSensitive values are redacted." : "",
      ].join("\n"),
    );
    this.contextValue = "autocurl.request";
    this.iconPath = new vscode.ThemeIcon(request.error ? "error" : "globe");
    this.command = {
      command: "autocurl.showRequest",
      title: "Show cURL",
      arguments: [this],
    };
  }
}

class RequestsProvider implements vscode.TreeDataProvider<RequestItem> {
  private readonly changed = new vscode.EventEmitter<RequestItem | undefined | void>();
  readonly onDidChangeTreeData = this.changed.event;

  constructor(private readonly session: CaptureSession) {
    session.onDidChange(() => this.changed.fire());
  }

  getTreeItem(element: RequestItem): vscode.TreeItem {
    return element;
  }

  getChildren(): RequestItem[] {
    return [...this.session.list()].reverse().map((request) => new RequestItem(request));
  }
}

export function activate(context: vscode.ExtensionContext): void {
  const output = vscode.window.createOutputChannel("Autocurl", { log: true });
  const binary = new BinaryManager(context, output);
  const session = new CaptureSession(binary, output);
  const provider = new RequestsProvider(session);
  let lastLaunch:
    | { folder: vscode.WorkspaceFolder | undefined; configuration: vscode.DebugConfiguration }
    | undefined;

  context.subscriptions.push(
    output,
    session,
    vscode.window.registerTreeDataProvider("autocurl.requests", provider),
    vscode.commands.registerCommand("autocurl.start", async () => {
      await runWithErrors(async () => {
        const ready = await session.start();
        void vscode.window.showInformationMessage(
          `Autocurl is capturing. Start debugging normally; requests will appear in Explorer (${ready.proxy_url}).`,
        );
      }, output);
    }),
    vscode.commands.registerCommand("autocurl.stop", async () => {
      await session.stopSession();
    }),
    vscode.commands.registerCommand("autocurl.pause", () => session.pause()),
    vscode.commands.registerCommand("autocurl.resume", () => session.resume()),
    vscode.commands.registerCommand("autocurl.clear", () => session.clear()),
    vscode.commands.registerCommand("autocurl.copyLast", async () => {
      const request = session.last();
      if (!request) {
        void vscode.window.showInformationMessage("Autocurl has not captured a request yet.");
        return;
      }
      await copyRequest(request);
    }),
    vscode.commands.registerCommand("autocurl.copyRequest", async (item: RequestItem) => {
      await copyRequest(item.request);
    }),
    vscode.commands.registerCommand("autocurl.showRequest", async (item: RequestItem) => {
      const document = await vscode.workspace.openTextDocument({
        language: "shellscript",
        content: `${item.request.curl}\n`,
      });
      await vscode.window.showTextDocument(document, { preview: true });
    }),
    vscode.commands.registerCommand("autocurl.generateFromRequestJson", async () => {
      await runWithErrors(async () => {
        const editor = vscode.window.activeTextEditor;
        let requestJSON = editor?.document.getText(editor.selection).trim() ?? "";
        if (!requestJSON && editor?.document.languageId === "json") {
          requestJSON = editor.document.getText().trim();
        }
        if (!requestJSON) {
          requestJSON = (await vscode.env.clipboard.readText()).trim();
        }
        if (!requestJSON) {
          const template = await chooseRequestTemplate();
          if (!template) {
            return;
          }
          const document = await vscode.workspace.openTextDocument({
            language: "json",
            content: `${template}\n`,
          });
          await vscode.window.showTextDocument(document, { preview: false });
          void vscode.window.showInformationMessage(
            "Edit the request values, then run “Autocurl: Generate cURL from Request JSON” again.",
          );
          return;
        }
        const executable = await binary.resolve();
        const stdout = await renderRequest(executable, requestJSON);
        const result = JSON.parse(stdout) as { curl?: string; error?: string };
        if (!result.curl) {
          throw new Error(result.error ?? "The engine returned no cURL.");
        }
        await vscode.env.clipboard.writeText(result.curl);
        const document = await vscode.workspace.openTextDocument({
          language: "shellscript",
          content: `${result.curl}\n`,
        });
        await vscode.window.showTextDocument(document, { preview: true });
        void vscode.window.showInformationMessage("Generated cURL copied to clipboard.");
      }, output);
    }),
    vscode.commands.registerCommand("autocurl.renderSelection", async () => {
      await vscode.commands.executeCommand("autocurl.generateFromRequestJson");
    }),
    vscode.commands.registerCommand("autocurl.openQuickStart", async () => {
      const document = await vscode.workspace.openTextDocument(
        vscode.Uri.joinPath(context.extensionUri, "README.md"),
      );
      await vscode.window.showTextDocument(document, { preview: true });
    }),
    vscode.commands.registerCommand("autocurl.downloadBinary", async () => {
      await runWithErrors(async () => {
        await binary.resolve(true);
        void vscode.window.showInformationMessage("Autocurl engine is installed and verified.");
      }, output);
    }),
    vscode.commands.registerCommand("autocurl.runDiagnostics", async () => {
      await runWithErrors(async () => {
        const executable = await binary.resolve();
        const report = await runDoctor(executable);
        output.appendLine(report);
        const document = await vscode.workspace.openTextDocument({
          language: "plaintext",
          content: `${report}\n`,
        });
        await vscode.window.showTextDocument(document, { preview: true });
      }, output);
    }),
    vscode.debug.registerDebugConfigurationProvider("*", {
      async resolveDebugConfiguration(
        folder: vscode.WorkspaceFolder | undefined,
        configuration: vscode.DebugConfiguration,
      ): Promise<vscode.DebugConfiguration> {
        lastLaunch = {
          folder,
          configuration: {
            ...configuration,
            env: { ...(configuration.env as Record<string, string> | undefined) },
          },
        };
        const settings = vscode.workspace.getConfiguration("autocurl");
        if (!settings.get<boolean>("injectDebugEnvironment", true)) {
          return configuration;
        }
        let ready = session.currentReady();
        if (!ready && !settings.get<boolean>("autoStartOnDebug", true)) {
          return configuration;
        }
        try {
          ready ??= await session.start();
          return {
            ...configuration,
            env: session.environmentFor(configuration.env as Record<string, string> | undefined, ready),
          };
        } catch (error) {
          const message = error instanceof Error ? error.message : String(error);
          output.appendLine(`Could not prepare debug capture: ${message}`);
          void vscode.window.showErrorMessage(`Autocurl did not start: ${message}`);
          return configuration;
        }
      },
    }),
    session.onDidDiagnostic((diagnostic) => {
      void handleDiagnostic(diagnostic, session, output, lastLaunch);
    }),
  );
}

async function chooseRequestTemplate(): Promise<string | undefined> {
  const templates: Array<{ label: string; description: string; value: string }> = [
    {
      label: "Generic request",
      description: "Canonical method, URL, headers, body",
      value: `{
  "method": "POST",
  "url": "https://api.example.com/orders",
  "protocol": "HTTP/2",
  "headers": {"Content-Type": "application/json"},
  "body": {"order_id": "demo-42"}
}`,
    },
    {
      label: "Go http.Request",
      description: "Exported Go request fields",
      value: `{
  "Method": "POST",
  "URL": {"Scheme": "https", "Host": "api.example.com", "Path": "/orders"},
  "Header": {"Content-Type": ["application/json"]},
  "Body": {"order_id": "demo-42"}
}`,
    },
    {
      label: "Java HttpRequest",
      description: "URI and HttpHeaders map",
      value: `{
  "method": "POST",
  "uri": "https://api.example.com/orders",
  "headers": {"map": {"Content-Type": ["application/json"]}},
  "body": {"order_id": "demo-42"}
}`,
    },
    {
      label: "Python PreparedRequest",
      description: "requests-style request object",
      value: `{
  "method": "POST",
  "url": "https://api.example.com/orders",
  "headers": {"Content-Type": "application/json"},
  "body": {"order_id": "demo-42"}
}`,
    },
    {
      label: "Node.js Axios",
      description: "baseURL, url, headers, data",
      value: `{
  "method": "post",
  "baseURL": "https://api.example.com",
  "url": "/orders",
  "headers": {"Content-Type": "application/json"},
  "data": {"order_id": "demo-42"}
}`,
    },
    {
      label: "Node.js fetch",
      description: "URL and fetch options",
      value: `{
  "url": "https://api.example.com/orders",
  "options": {
    "method": "POST",
    "headers": {"Content-Type": "application/json"},
    "body": "{\\"order_id\\":\\"demo-42\\"}"
  }
}`,
    },
  ];
  const selected = await vscode.window.showQuickPick(templates, {
    title: "Generate cURL from Request JSON",
    placeHolder: "Choose a debugger request shape",
  });
  return selected?.value;
}

async function handleDiagnostic(
  diagnostic: DiagnosticEvent,
  session: CaptureSession,
  output: vscode.OutputChannel,
  lastLaunch:
    | { folder: vscode.WorkspaceFolder | undefined; configuration: vscode.DebugConfiguration }
    | undefined,
): Promise<void> {
  if (diagnostic.severity === "information") {
    vscode.window.setStatusBarMessage(`Autocurl: ${diagnostic.summary}`, 5000);
    return;
  }
  const bypassAction = diagnostic.bypass_target
    ? "Always bypass this host and restart"
    : undefined;
  const actions = [bypassAction, "Open Autocurl Output"].filter(
    (value): value is string => value !== undefined,
  );
  const message = diagnostic.suggested_action
    ? `${diagnostic.summary}. ${diagnostic.suggested_action}`
    : diagnostic.summary;
  const selected = diagnostic.severity === "error"
    ? await vscode.window.showErrorMessage(message, ...actions)
    : await vscode.window.showWarningMessage(message, ...actions);

  if (selected === "Open Autocurl Output") {
    output.show(true);
    return;
  }
  if (selected !== bypassAction || !diagnostic.bypass_target) {
    return;
  }
  const settings = vscode.workspace.getConfiguration("autocurl");
  const targets = settings.get<string[]>("bypassTargets", []);
  if (!targets.includes(diagnostic.bypass_target)) {
    const scope = vscode.workspace.workspaceFile || vscode.workspace.workspaceFolders?.length
      ? vscode.ConfigurationTarget.Workspace
      : vscode.ConfigurationTarget.Global;
    await settings.update(
      "bypassTargets",
      [...targets, diagnostic.bypass_target],
      scope,
    );
  }
  await session.stopSession();
  if (lastLaunch) {
    await vscode.debug.startDebugging(lastLaunch.folder, lastLaunch.configuration);
  }
}

function renderRequest(executable: string, requestJSON: string): Promise<string> {
  return new Promise((resolve, reject) => {
    const child = spawn(executable, ["--json", "render", "--request", "-"], {
      stdio: ["pipe", "pipe", "pipe"],
      windowsHide: true,
    });
    const stdout: Buffer[] = [];
    const stderr: Buffer[] = [];
    let stdoutBytes = 0;
    const timeout = setTimeout(() => {
      child.kill();
      reject(new Error("Rendering request JSON timed out."));
    }, 15000);
    child.stdout.on("data", (chunk: Buffer) => {
      stdoutBytes += chunk.length;
      if (stdoutBytes > 20 * 1024 * 1024) {
        child.kill();
        reject(new Error("Rendered output exceeded 20 MiB."));
        return;
      }
      stdout.push(chunk);
    });
    child.stderr.on("data", (chunk: Buffer) => stderr.push(chunk));
    child.on("error", (error) => {
      clearTimeout(timeout);
      reject(error);
    });
    child.on("exit", (code) => {
      clearTimeout(timeout);
      if (code === 0) {
        resolve(Buffer.concat(stdout).toString("utf8"));
      } else {
        reject(new Error(
          Buffer.concat(stderr).toString("utf8").trim() ||
          `Autocurl render exited with code ${code}.`,
        ));
      }
    });
    child.stdin.end(requestJSON);
  });
}

function runDoctor(executable: string): Promise<string> {
  return new Promise((resolve, reject) => {
    const child = spawn(executable, ["doctor"], {
      stdio: ["ignore", "pipe", "pipe"],
      windowsHide: true,
    });
    const stdout: Buffer[] = [];
    const stderr: Buffer[] = [];
    const timeout = setTimeout(() => {
      child.kill();
      reject(new Error("Environment check timed out."));
    }, 15000);
    child.stdout.on("data", (chunk: Buffer) => stdout.push(chunk));
    child.stderr.on("data", (chunk: Buffer) => stderr.push(chunk));
    child.on("error", (error) => {
      clearTimeout(timeout);
      reject(error);
    });
    child.on("exit", (code) => {
      clearTimeout(timeout);
      if (code === 0) {
        resolve(Buffer.concat(stdout).toString("utf8").trim());
      } else {
        reject(new Error(
          Buffer.concat(stderr).toString("utf8").trim() ||
          `Autocurl doctor exited with code ${code}.`,
        ));
      }
    });
  });
}

async function copyRequest(request: RequestEvent): Promise<void> {
  await vscode.env.clipboard.writeText(request.curl);
  void vscode.window.setStatusBarMessage(`Autocurl #${request.id} copied`, 2500);
}

async function runWithErrors(
  action: () => Promise<void>,
  output: vscode.OutputChannel,
): Promise<void> {
  try {
    await action();
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    output.appendLine(message);
    output.show(true);
    void vscode.window.showErrorMessage(`Autocurl: ${message}`);
  }
}

export function deactivate(): void {
  // Disposables registered in activate own process cleanup.
}
