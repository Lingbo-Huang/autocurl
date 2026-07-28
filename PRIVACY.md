# Privacy

Autocurl processes captured requests locally on the developer's machine.

- It has no Autocurl account, cloud backend, analytics, advertising, or
  telemetry in version 0.3.0.
- Request metadata, bodies, generated cURLs, and temporary certificate keys are
  not uploaded to the project author.
- IDE plugins contact GitHub only to check for and download a compatible
  Autocurl engine when automatic download is enabled.
- The proxy connects to the destinations requested by the application and
  validates upstream TLS certificates.
- Common credentials and sensitive fields are redacted before display by
  default. `showSecrets` is an explicit local setting and can expose secrets in
  the IDE, clipboard, terminal, or output file.
- Each capture session uses an ephemeral local CA. Its private key remains in
  memory, temporary trust files are removed when the session ends, and no CA is
  added to the operating-system keychain.

Captured data can still be sensitive even after automatic redaction. Review a
generated cURL before pasting it into chat, tickets, CI logs, shell history, or
another machine.

Questions and security reports should follow [SECURITY.md](SECURITY.md).

## 中文

Autocurl 在开发者本机处理请求。0.3.0 没有账号、云端服务、广告、分析或遥测，
不会把请求、Body、cURL 或临时证书私钥上传给作者。IDE 插件仅在启用自动下载时
访问 GitHub 获取匹配引擎。

常见敏感字段默认脱敏，但生成结果仍可能包含业务数据。分享、粘贴到工单或提交
到仓库前请人工检查。`showSecrets` 会显式关闭脱敏，只应在私密本机调试时使用。
