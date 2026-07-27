const esbuild = require("esbuild");

const watch = process.argv.includes("--watch");
const options = {
  entryPoints: ["src/extension.ts"],
  bundle: true,
  outfile: "dist/extension.js",
  external: ["vscode"],
  format: "cjs",
  platform: "node",
  target: "node18",
  sourcemap: true,
  minify: !watch,
};

async function main() {
  if (watch) {
    const context = await esbuild.context(options);
    await context.watch();
    console.log("watching Autocurl extension sources");
    return;
  }
  await esbuild.build(options);
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
