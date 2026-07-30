import { access, readFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";

const repositoryRoot = process.cwd();
const siteRoot = path.join(repositoryRoot, "site");
const htmlFiles = [
  path.join(siteRoot, "index.html"),
  path.join(siteRoot, "zh", "index.html"),
  path.join(siteRoot, "404.html"),
];

const failures = [];

async function exists(file) {
  try {
    await access(file);
    return true;
  } catch {
    return false;
  }
}

for (const file of htmlFiles) {
  const source = await readFile(file, "utf8");
  const relativeName = path.relative(repositoryRoot, file);
  const h1Count = (source.match(/<h1(?:\s|>)/g) || []).length;

  if (h1Count !== 1) {
    failures.push(`${relativeName}: expected exactly one h1, found ${h1Count}`);
  }

  if (!/<html lang="[^"]+">/.test(source)) {
    failures.push(`${relativeName}: missing html lang attribute`);
  }

  if (/xiaohongshu|edithai|10\.35\.|10\.61\./i.test(source)) {
    failures.push(`${relativeName}: contains an internal test value`);
  }

  for (const match of source.matchAll(/(?:href|src)="([^"]+)"/g)) {
    const reference = match[1];

    if (
      reference.startsWith("http://") ||
      reference.startsWith("https://") ||
      reference.startsWith("#") ||
      reference.startsWith("data:") ||
      reference.startsWith("mailto:")
    ) {
      continue;
    }

    const cleanReference = reference.split(/[?#]/, 1)[0];
    const target = path.resolve(path.dirname(file), cleanReference);

    if (!(await exists(target))) {
      failures.push(`${relativeName}: broken local reference ${reference}`);
    }
  }
}

const appSource = await readFile(path.join(siteRoot, "app.js"), "utf8");

if (!appSource.includes("[REDACTED]")) {
  failures.push("site/app.js: examples must demonstrate redaction");
}

const vercelConfig = JSON.parse(
  await readFile(path.join(repositoryRoot, "vercel.json"), "utf8"),
);

if (vercelConfig.outputDirectory !== "site") {
  failures.push("vercel.json: outputDirectory must remain site");
}

if (failures.length > 0) {
  console.error(failures.join("\n"));
  process.exit(1);
}

console.log("Website structure and local links are valid.");
