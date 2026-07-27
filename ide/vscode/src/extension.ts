import { spawn } from "node:child_process";
import * as vscode from "vscode";
import { BinaryManager } from "./binary";
import { CaptureSession, RequestEvent } from "./session";

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
      await session.stop();
    }),
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
    vscode.commands.registerCommand("autocurl.renderSelection", async () => {
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
          throw new Error("Select a request JSON object or copy one to the clipboard first.");
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
        void vscode.window.showInformationMessage("Rendered cURL copied to clipboard.");
      }, output);
    }),
    vscode.commands.registerCommand("autocurl.downloadBinary", async () => {
      await runWithErrors(async () => {
        await binary.resolve(true);
        void vscode.window.showInformationMessage("Autocurl engine is installed and verified.");
      }, output);
    }),
    vscode.debug.registerDebugConfigurationProvider("*", {
      async resolveDebugConfiguration(
        _folder: vscode.WorkspaceFolder | undefined,
        configuration: vscode.DebugConfiguration,
      ): Promise<vscode.DebugConfiguration> {
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
  );
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
