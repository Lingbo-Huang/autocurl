import { spawn, ChildProcessWithoutNullStreams } from "node:child_process";
import { randomUUID } from "node:crypto";
import { createConnection } from "node:net";
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
  mode: "safe" | "strict";
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

export interface DiagnosticEvent {
  schema_version: string;
  type: "diagnostic";
  timestamp: string;
  code: string;
  severity: "information" | "warning" | "error";
  host?: string;
  summary: string;
  detail?: string;
  suggested_action?: string;
  bypass_target?: string;
  auto_applied?: boolean;
  retry_required?: boolean;
}

interface StateEvent {
  schema_version: string;
  type: "state";
  recording: boolean;
}

type EngineEvent = ReadyEvent | RequestEvent | DiagnosticEvent | StateEvent;

export class CaptureSession implements vscode.Disposable {
  private process: ChildProcessWithoutNullStreams | undefined;
  private ready: ReadyEvent | undefined;
  private startPromise: Promise<ReadyEvent> | undefined;
  private recording = false;
  private sessionToken = "";
  private stoppingSession = false;
  private readonly requests: RequestEvent[] = [];
  private readonly debugSessions = new Set<vscode.DebugSession>();
  private readonly changed = new vscode.EventEmitter<void>();
  private readonly diagnostics = new vscode.EventEmitter<DiagnosticEvent>();
  private readonly status: vscode.StatusBarItem;
  private readonly debugDisposables: vscode.Disposable[];

  readonly onDidChange = this.changed.event;
  readonly onDidDiagnostic = this.diagnostics.event;

  constructor(
    private readonly binary: BinaryManager,
    private readonly output: vscode.OutputChannel,
  ) {
    this.status = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 50);
    this.debugDisposables = [
      vscode.debug.onDidStartDebugSession((session) => this.trackDebugSession(session)),
      vscode.debug.onDidTerminateDebugSession((session) => {
        if (!this.debugSessions.delete(session)) {
          return;
        }
        if (!this.stoppingSession && this.requests.length === 0) {
          this.emitDiagnostic({
            schema_version: "1",
            type: "diagnostic",
            timestamp: new Date().toISOString(),
            code: "application_exited_before_capture",
            severity: "warning",
            summary: "The debug session ended before Autocurl captured a request",
            detail: "The application may have failed during startup or simply sent no outbound request.",
            suggested_action: "Inspect the Debug Console and verify any configured listen port.",
            retry_required: true,
          });
        }
        if (!this.stoppingSession && this.debugSessions.size === 0) {
          void this.stopEngine();
        }
      }),
    ];
    this.updateStatus();
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

  isRunning(): boolean {
    return this.ready !== undefined;
  }

  isRecording(): boolean {
    return this.ready !== undefined && this.recording;
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
    const mode = configuration.get<"safe" | "strict">("captureMode", "safe");
    const args = ["--json", "proxy", "--lifetime-stdin", "--mode", mode];
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
    for (const target of configuration.get<string[]>("bypassTargets", [])) {
      args.push("--bypass", target);
    }

    this.clear();
    this.sessionToken = randomUUID();
    this.recording = false;
    this.updateStatus(true);
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
          void this.stopEngine();
          reject(new Error("Timed out waiting for the Autocurl engine to become ready."));
        }
      }, 15000);

      const lines = readline.createInterface({ input: child.stdout });
      lines.on("line", (line) => {
        let event: EngineEvent;
        try {
          event = JSON.parse(line) as EngineEvent;
        } catch {
          this.output.appendLine(`Ignored non-JSON engine output: ${line}`);
          return;
        }
        if (event.type === "ready") {
          this.ready = event;
          this.recording = true;
          this.updateStatus();
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
          return;
        }
        if (event.type === "diagnostic") {
          this.emitDiagnostic(event);
          return;
        }
        if (event.type === "state") {
          this.recording = event.recording;
          this.updateStatus();
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
        if (this.process === child) {
          this.process = undefined;
        }
        this.ready = undefined;
        this.recording = false;
        this.updateStatus();
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
    merged.AUTOCURL_SESSION_ID = this.sessionToken;
    return merged;
  }

  pause(): void {
    this.sendControl("pause");
  }

  resume(): void {
    this.sendControl("resume");
  }

  private sendControl(command: "pause" | "resume"): void {
    const child = this.process;
    if (!child || !this.ready) {
      return;
    }
    child.stdin.write(`${JSON.stringify({ command })}\n`, (error) => {
      if (error) {
        this.emitDiagnostic({
          schema_version: "1",
          type: "diagnostic",
          timestamp: new Date().toISOString(),
          code: "capture_control_failed",
          severity: "error",
          summary: "Autocurl could not change the recording state",
          detail: error.message,
          suggested_action: "Stop the session and start it again.",
          retry_required: true,
        });
      }
    });
  }

  async stopSession(): Promise<void> {
    if (this.stoppingSession) {
      return;
    }
    this.stoppingSession = true;
    try {
      const sessions = [...this.debugSessions];
      await Promise.all(sessions.map(async (session) => {
        await vscode.debug.stopDebugging(session);
      }));
      this.debugSessions.clear();
      await this.stopEngine();
    } finally {
      this.stoppingSession = false;
    }
  }

  private async stopEngine(): Promise<void> {
    const child = this.process;
    if (!child) {
      return;
    }
    this.process = undefined;
    child.stdin.end();
    await new Promise<void>((resolve) => {
      const forced = setTimeout(() => {
        child.kill();
        resolve();
      }, 2500);
      child.once("exit", () => {
        clearTimeout(forced);
        resolve();
      });
    });
  }

  private trackDebugSession(session: vscode.DebugSession): void {
    const environment = session.configuration.env as Record<string, unknown> | undefined;
    if (environment?.AUTOCURL_SESSION_ID !== this.sessionToken) {
      return;
    }
    this.debugSessions.add(session);
    this.scheduleStartupDiagnostics(session);
  }

  private scheduleStartupDiagnostics(session: vscode.DebugSession): void {
    const configuration = vscode.workspace.getConfiguration("autocurl");
    const delaySeconds = Math.max(1, configuration.get<number>("startupDiagnosticSeconds", 15));
    const requestCountAtStart = this.requests.length;
    setTimeout(() => {
      if (!this.debugSessions.has(session)) {
        return;
      }
      const expectedPorts = configuration.get<number[]>("expectedListenPorts", []);
      void Promise.all(expectedPorts.map(async (port) => ({
        port,
        listening: await isLocalPortListening(port),
      }))).then((checks) => {
        if (!this.debugSessions.has(session)) {
          return;
        }
        const missing = checks.filter((check) => !check.listening).map((check) => check.port);
        if (missing.length > 0) {
          this.emitDiagnostic({
            schema_version: "1",
            type: "diagnostic",
            timestamp: new Date().toISOString(),
            code: "expected_port_not_listening",
            severity: "error",
            host: "127.0.0.1",
            summary: "The application is running but an expected local port is not listening",
            detail: `Not listening after ${delaySeconds} seconds: ${missing.join(", ")}`,
            suggested_action:
              "Inspect startup dependencies and the Debug Console. Increase the delay if this service starts slowly.",
            retry_required: true,
          });
          return;
        }
        if (this.requests.length === requestCountAtStart) {
          this.emitDiagnostic({
            schema_version: "1",
            type: "diagnostic",
            timestamp: new Date().toISOString(),
            code: "no_proxy_traffic_observed",
            severity: "information",
            summary: "Autocurl has not observed outbound proxy traffic yet",
            detail:
              "Trigger an outbound request. If one was already sent, that HTTP client may ignore proxy environment variables.",
            suggested_action:
              "Use a standard client or configure its explicit proxy. Already-running processes must be restarted.",
          });
        }
      });
    }, delaySeconds * 1000);
  }

  private emitDiagnostic(event: DiagnosticEvent): void {
    const host = event.host ? ` (${event.host})` : "";
    this.output.appendLine(`[${event.severity}] ${event.code}${host}: ${event.summary}`);
    if (event.detail) {
      this.output.appendLine(event.detail);
    }
    if (event.suggested_action) {
      this.output.appendLine(`Next step: ${event.suggested_action}`);
    }
    this.diagnostics.fire(event);
  }

  private updateStatus(starting = false): void {
    if (starting) {
      this.status.text = "$(sync~spin) Autocurl starting";
      this.status.tooltip = "Starting process-scoped capture";
      this.status.command = undefined;
    } else if (!this.ready) {
      this.status.text = "$(circle-slash) Autocurl";
      this.status.tooltip = "Start Autocurl capture";
      this.status.command = "autocurl.start";
    } else if (this.recording) {
      this.status.text = "$(record) Autocurl";
      this.status.tooltip = `Recording through ${this.ready.proxy_url}. Click to pause recording.`;
      this.status.command = "autocurl.pause";
    } else {
      this.status.text = "$(debug-pause) Autocurl";
      this.status.tooltip = "Recording is paused; traffic still passes. Click to resume.";
      this.status.command = "autocurl.resume";
    }
    void vscode.commands.executeCommand("setContext", "autocurl.running", this.ready !== undefined);
    void vscode.commands.executeCommand("setContext", "autocurl.recording", this.recording);
  }

  dispose(): void {
    void this.stopSession();
    this.status.dispose();
    this.changed.dispose();
    this.diagnostics.dispose();
    this.debugDisposables.forEach((disposable) => disposable.dispose());
  }
}

function isLocalPortListening(port: number): Promise<boolean> {
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    return Promise.resolve(false);
  }
  return new Promise<boolean>((resolve) => {
    const socket = createConnection({ host: "127.0.0.1", port });
    const finish = (result: boolean) => {
      socket.destroy();
      resolve(result);
    };
    socket.setTimeout(300);
    socket.once("connect", () => finish(true));
    socket.once("timeout", () => finish(false));
    socket.once("error", () => finish(false));
  });
}
