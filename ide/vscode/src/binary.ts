import * as crypto from "node:crypto";
import * as fs from "node:fs";
import * as https from "node:https";
import * as os from "node:os";
import * as path from "node:path";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import * as vscode from "vscode";

const execFileAsync = promisify(execFile);
const repository = "Lingbo-Huang/autocurl";
// Keep this aligned with the extension version. Older engines may be protocol
// compatible while still missing runtime fixes such as platform Go CA trust.
const minimumVersion = "0.2.4";

interface ReleaseAsset {
  name: string;
  browser_download_url: string;
}

interface Release {
  tag_name: string;
  assets: ReleaseAsset[];
}

export class BinaryManager {
  constructor(
    private readonly context: vscode.ExtensionContext,
    private readonly output: vscode.OutputChannel,
  ) {}

  async resolve(forceDownload = false): Promise<string> {
    const configuration = vscode.workspace.getConfiguration("autocurl");
    const configured = configuration.get<string>("binaryPath", "").trim();
    if (!forceDownload && configured) {
      await this.requireCompatible(configured);
      return configured;
    }

    const managed = this.managedBinaryPath();
    if (!forceDownload && await this.isCompatible(managed)) {
      return managed;
    }
    if (!forceDownload && await this.isCompatible("autocurl")) {
      return "autocurl";
    }
    if (!configuration.get<boolean>("autoDownload", true) && !forceDownload) {
      throw new Error(
        "No compatible autocurl engine was found. Set autocurl.binaryPath or enable autocurl.autoDownload.",
      );
    }
    return vscode.window.withProgress(
      {
        location: vscode.ProgressLocation.Notification,
        title: "Downloading the Autocurl engine",
        cancellable: false,
      },
      async () => this.downloadLatest(),
    );
  }

  private managedBinaryPath(): string {
    return path.join(
      this.context.globalStorageUri.fsPath,
      "bin",
      process.platform === "win32" ? "autocurl.exe" : "autocurl",
    );
  }

  private async isCompatible(binary: string): Promise<boolean> {
    try {
      await this.requireCompatible(binary);
      return true;
    } catch {
      return false;
    }
  }

  private async requireCompatible(binary: string): Promise<void> {
    const { stdout } = await execFileAsync(binary, ["version"], {
      timeout: 5000,
      windowsHide: true,
    });
    const version = stdout.trim().replace(/^v/, "");
    if (compareVersions(version, minimumVersion) < 0) {
      throw new Error(
        `Autocurl ${minimumVersion} or newer is required; ${binary} reports ${version}.`,
      );
    }
  }

  private async downloadLatest(): Promise<string> {
    const release = JSON.parse(
      (await downloadBuffer(
        `https://api.github.com/repos/${repository}/releases/latest`,
        "application/vnd.github+json",
      )).toString("utf8"),
    ) as Release;
    const version = release.tag_name.replace(/^v/, "");
    if (compareVersions(version, minimumVersion) < 0) {
      throw new Error(
        `Latest GitHub release is ${release.tag_name}, but this extension requires v${minimumVersion} or newer.`,
      );
    }

    const platform = platformName();
    const architecture = architectureName();
    const suffix = process.platform === "win32" ? ".zip" : ".tar.gz";
    const assetName = `autocurl-${release.tag_name}-${platform}-${architecture}${suffix}`;
    const archiveAsset = release.assets.find((asset) => asset.name === assetName);
    const checksumAsset = release.assets.find((asset) => asset.name === "SHA256SUMS");
    if (!archiveAsset || !checksumAsset) {
      throw new Error(`Release ${release.tag_name} has no asset for ${platform}/${architecture}.`);
    }

    const archive = await downloadBuffer(archiveAsset.browser_download_url);
    const sums = (await downloadBuffer(checksumAsset.browser_download_url)).toString("utf8");
    const expected = checksumFor(sums, assetName);
    const actual = crypto.createHash("sha256").update(archive).digest("hex");
    if (!expected || actual !== expected) {
      throw new Error(`SHA-256 verification failed for ${assetName}.`);
    }

    const storage = this.context.globalStorageUri.fsPath;
    const staging = path.join(storage, `staging-${Date.now()}`);
    const archivePath = path.join(staging, assetName);
    await fs.promises.mkdir(staging, { recursive: true });
    await fs.promises.writeFile(archivePath, archive, { mode: 0o600 });
    try {
      await execFileAsync("tar", ["-xf", archivePath, "-C", staging], {
        timeout: 30000,
        windowsHide: true,
      });
      const extracted = await findBinary(staging);
      if (!extracted) {
        throw new Error(`No autocurl executable was found in ${assetName}.`);
      }
      const managed = this.managedBinaryPath();
      await fs.promises.mkdir(path.dirname(managed), { recursive: true });
      await fs.promises.copyFile(extracted, managed);
      if (process.platform !== "win32") {
        await fs.promises.chmod(managed, 0o755);
      }
      await this.requireCompatible(managed);
      this.output.appendLine(`Installed ${release.tag_name} engine at ${managed}`);
      return managed;
    } finally {
      await fs.promises.rm(staging, { recursive: true, force: true });
    }
  }
}

function platformName(): string {
  if (process.platform === "darwin") {
    return "darwin";
  }
  if (process.platform === "linux") {
    return "linux";
  }
  if (process.platform === "win32") {
    return "windows";
  }
  throw new Error(`Unsupported operating system: ${process.platform}`);
}

function architectureName(): string {
  if (process.arch === "x64") {
    return "amd64";
  }
  if (process.arch === "arm64") {
    return "arm64";
  }
  throw new Error(`Unsupported CPU architecture: ${process.arch}`);
}

function compareVersions(left: string, right: string): number {
  const a = left.split(/[.-]/).slice(0, 3).map((value) => Number(value) || 0);
  const b = right.split(/[.-]/).slice(0, 3).map((value) => Number(value) || 0);
  for (let index = 0; index < 3; index += 1) {
    if (a[index] !== b[index]) {
      return a[index] - b[index];
    }
  }
  return 0;
}

function checksumFor(contents: string, assetName: string): string | undefined {
  for (const line of contents.split(/\r?\n/)) {
    const match = line.trim().match(/^([a-fA-F0-9]{64})\s+\*?(.+)$/);
    if (match && path.basename(match[2]) === assetName) {
      return match[1].toLowerCase();
    }
  }
  return undefined;
}

async function findBinary(directory: string): Promise<string | undefined> {
  const wanted = process.platform === "win32" ? "autocurl.exe" : "autocurl";
  for (const entry of await fs.promises.readdir(directory, { withFileTypes: true })) {
    const candidate = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      const nested = await findBinary(candidate);
      if (nested) {
        return nested;
      }
    } else if (entry.name === wanted) {
      return candidate;
    }
  }
  return undefined;
}

function downloadBuffer(url: string, accept = "application/octet-stream"): Promise<Buffer> {
  return new Promise((resolve, reject) => {
    const request = https.get(
      url,
      {
        headers: {
          Accept: accept,
          "User-Agent": "autocurl-vscode-extension",
          "X-GitHub-Api-Version": "2022-11-28",
        },
        timeout: 30000,
      },
      (response) => {
        if (response.statusCode && response.statusCode >= 300 && response.statusCode < 400) {
          const location = response.headers.location;
          response.resume();
          if (!location) {
            reject(new Error(`Download redirected without a Location header: ${url}`));
            return;
          }
          downloadBuffer(location, accept).then(resolve, reject);
          return;
        }
        if (response.statusCode !== 200) {
          response.resume();
          reject(new Error(`Download failed with HTTP ${response.statusCode}: ${url}`));
          return;
        }
        const chunks: Buffer[] = [];
        response.on("data", (chunk: Buffer) => chunks.push(chunk));
        response.on("end", () => resolve(Buffer.concat(chunks)));
      },
    );
    request.on("timeout", () => request.destroy(new Error(`Download timed out: ${url}`)));
    request.on("error", reject);
  });
}
