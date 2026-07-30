const repository = "Lingbo-Huang/autocurl";

const examples = {
  get: `curl \\
  --request 'GET' \\
  --url 'https://api.example.com/v1/users?active=true' \\
  --http2 \\
  --header 'Accept: application/json' \\
  --header 'Authorization: [REDACTED]'`,
  post: `curl \\
  --request 'POST' \\
  --url 'https://api.example.com/v1/orders' \\
  --http2 \\
  --header 'Content-Type: application/json' \\
  --header 'Authorization: [REDACTED]' \\
  --data-binary '{"order_id":"demo-42","items":[{"sku":"A-42","quantity":2}]}'`,
  wss: `curl \\
  --request 'GET' \\
  --url 'https://stream.example.com/v1/events' \\
  --http1.1 \\
  --header 'Connection: Upgrade' \\
  --header 'Upgrade: websocket' \\
  --header 'Authorization: [REDACTED]'`,
};

const requestRows = document.querySelectorAll("[data-request]");
const curlOutput = document.querySelector("[data-curl-output]");

requestRows.forEach((row) => {
  row.addEventListener("click", () => {
    requestRows.forEach((candidate) => {
      candidate.classList.toggle("is-selected", candidate === row);
    });
    curlOutput.textContent = examples[row.dataset.request];
  });
});

const copyButton = document.querySelector("[data-copy-curl]");

copyButton?.addEventListener("click", async () => {
  const label = copyButton.querySelector("span");
  const originalLabel = label.textContent;
  let copied = false;

  try {
    await navigator.clipboard.writeText(curlOutput.textContent);
    copied = true;
  } catch {
    const fallback = document.createElement("textarea");
    fallback.value = curlOutput.textContent;
    fallback.setAttribute("readonly", "");
    fallback.style.position = "fixed";
    fallback.style.opacity = "0";
    document.body.append(fallback);
    fallback.select();
    copied = document.execCommand("copy");
    fallback.remove();

    if (!copied) {
      const selection = window.getSelection();
      const range = document.createRange();
      range.selectNodeContents(curlOutput);
      selection.removeAllRanges();
      selection.addRange(range);
    }
  }

  label.textContent = document.documentElement.lang.startsWith("zh")
    ? copied
      ? "已复制"
      : "请手动复制"
    : copied
      ? "Copied"
      : "Select to copy";

  window.setTimeout(() => {
    label.textContent = originalLabel;
  }, 1600);
});

async function resolveLatestRelease() {
  try {
    const response = await fetch(
      `https://api.github.com/repos/${repository}/releases/latest`,
      {
        headers: {
          Accept: "application/vnd.github+json",
        },
      },
    );

    if (!response.ok) {
      return;
    }

    const release = await response.json();
    const version = release.tag_name || "v0.3.2";

    document.querySelectorAll(".js-release-version").forEach((node) => {
      node.textContent = version;
    });

    document.querySelectorAll(".js-latest-release").forEach((link) => {
      link.href = release.html_url;
    });

    const matchers = {
      jetbrains: /^autocurl-jetbrains-.*\.zip$/,
      vscode: /^autocurl-.*\.vsix$/,
    };

    document.querySelectorAll("[data-release-asset]").forEach((link) => {
      const matcher = matchers[link.dataset.releaseAsset];
      const asset = release.assets?.find((candidate) =>
        matcher?.test(candidate.name),
      );

      if (asset) {
        link.href = asset.browser_download_url;
      }
    });
  } catch {
    // Static release links remain usable when GitHub's API is unavailable.
  }
}

resolveLatestRelease();
