import { spawn, ChildProcessWithoutNullStreams } from "node:child_process";
import * as readline from "node:readline";
import * as vscode from "vscode";
import { BinaryManager } from "./binary";

export interface ReadyEvent {
  schema_version: string;
  type: "ready";
  version: string;
  proxy_url: string;
  ca_file: string;
  environment: Record<string, string>;
  append_environment?: string[];
  notes?: string[];
}

export interface RequestEvent {
  schema_version: string;
  type: "request";
  id: number;
  timestamp: string;
  method: string;
  url: string;
  protocol?: string;
  upstream_protocol?: string;
  kind: string;
  status?: number;
  grpc_status?: string;
  duration_ms: number;
  error?: string;
  curl: string;
  redacted: boolean;
  body_truncated: boolean;
  warning?: string;
}

export class CaptureSession implements vscode.Disposable {
  private process: ChildProcessWithoutNullStreams | undefined;
  private ready: ReadyEvent | undefined;
  private startPromise: Promise<ReadyEvent> | undefined;
  private readonly requests: RequestEvent[] = [];
  private readonly changed = new vscode.EventEmitter<void>();
  private readonly status: vscode.StatusBarItem;

  readonly onDidChange = this.changed.event;

  constructor(
    private readonly binary: BinaryManager,
    private readonly output: vscode.OutputChannel,
  ) {
    this.status = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 50);
    this.status.command = "autocurl.start";
    this.status.text = "$(circle-slash) Autocurl";
    this.status.tooltip = "Start Autocurl capture";
    this.status.show();
  }

  list(): readonly RequestEvent[] {
    return this.requests;
  }

  last(): RequestEvent | undefined {
    return this.requests.at(-1);
  }

  currentReady(): ReadyEvent | undefined {
    return this.ready;
  }

  clear(): void {
    this.requests.length = 0;
    this.changed.fire();
  }

  async start(): Promise<ReadyEvent> {
    if (this.ready) {
      return this.ready;
    }
    if (this.startPromise) {
      return this.startPromise;
    }
    this.startPromise = this.startProcess();
    try {
      return await this.startPromise;
    } finally {
      this.startPromise = undefined;
    }
  }

  private async startProcess(): Promise<ReadyEvent> {
    const executable = await this.binary.resolve();
    const configuration = vscode.workspace.getConfiguration("autocurl");
    const args = ["--json", "proxy", "--lifetime-stdin"];
    const match = configuration.get<string>("match", "").trim();
    const method = configuration.get<string>("method", "").trim();
    if (match) {
      args.push("--match", match);
    }
    if (method) {
      args.push("--method", method);
    }
    args.push("--max-body", String(configuration.get<number>("maxBodyBytes", 1048576)));
    if (configuration.get<boolean>("showSecrets", false)) {
      args.push("--show-secrets");
    }
    for (const header of configuration.get<string[]>("replayHeaders", [])) {
      args.push("--replay-header", header);
    }
    for (const header of configuration.get<string[]>("liveHeaders", [])) {
      args.push("--live-header", header);
    }

    this.clear();
    this.status.text = "$(sync~spin) Autocurl starting";
    this.output.appendLine(`Starting: ${executable} ${args.join(" ")}`);
    const child = spawn(executable, args, {
      stdio: ["pipe", "pipe", "pipe"],
      windowsHide: true,
    });
    this.process = child;
    child.stderr.setEncoding("utf8");
    child.stderr.on("data", (data: string) => this.output.append(data));

    return new Promise<ReadyEvent>((resolve, reject) => {
      let settled = false;
      const timeout = setTimeout(() => {
        if (!settled) {
          settled = true;
          this.stop();
          reject(new Error("Timed out waiting for the Autocurl engine to become ready."));
        }
      }, 15000);

      const lines = readline.createInterface({ input: child.stdout });
      lines.on("line", (line) => {
        let event: ReadyEvent | RequestEvent;
        try {
          event = JSON.parse(line) as ReadyEvent | RequestEvent;
        } catch {
          this.output.appendLine(`Ignored non-JSON engine output: ${line}`);
          return;
        }
        if (event.type === "ready") {
          this.ready = event;
          this.status.text = "$(record) Autocurl";
          this.status.tooltip = `Capturing through ${event.proxy_url}`;
          this.status.command = "autocurl.stop";
          for (const note of event.notes ?? []) {
            this.output.appendLine(note);
          }
          if (!settled) {
            settled = true;
            clearTimeout(timeout);
            resolve(event);
          }
          return;
        }
        if (event.type === "request") {
          this.requests.push(event);
          this.changed.fire();
        }
      });
      child.on("error", (error) => {
        if (!settled) {
          settled = true;
          clearTimeout(timeout);
          reject(error);
        }
      });
      child.on("exit", (code, signal) => {
        this.process = undefined;
        this.ready = undefined;
        this.status.text = "$(circle-slash) Autocurl";
        this.status.tooltip = "Start Autocurl capture";
        this.status.command = "autocurl.start";
        this.changed.fire();
        if (!settled) {
          settled = true;
          clearTimeout(timeout);
          reject(new Error(`Autocurl engine exited before ready (code ${code}, signal ${signal}).`));
        }
      });
    });
  }

  environmentFor(
    existing: Record<string, string> | undefined,
    ready: ReadyEvent,
  ): Record<string, string> {
    const merged = { ...(existing ?? {}) };
    const append = new Set(ready.append_environment ?? []);
    for (const [name, value] of Object.entries(ready.environment)) {
      if (append.has(name) && merged[name]?.trim()) {
        merged[name] = `${merged[name]} ${value}`.trim();
      } else {
        merged[name] = value;
      }
    }
    return merged;
  }

  async stop(): Promise<void> {
    const child = this.process;
    if (!child) {
      return;
    }
    this.process = undefined;
    child.stdin.end();
    const forced = setTimeout(() => child.kill(), 2500);
    child.once("exit", () => clearTimeout(forced));
  }

  dispose(): void {
    void this.stop();
    this.status.dispose();
    this.changed.dispose();
  }
}
