# IDE 市场上架与自动升级

目标不是让用户反复下载 ZIP/VSIX，而是让他们从 IDE 市场安装一次，之后由
IDE 检查并安装新版本：

| IDE | 发布位置 | 用户升级方式 |
| --- | --- | --- |
| IntelliJ IDEA / GoLand / PyCharm / WebStorm | JetBrains Marketplace | 新版本审核通过后，IDE 提示更新 |
| VS Code | Visual Studio Marketplace | 默认自动更新已启用的扩展 |
| Cursor | Open VSX，再由 Cursor 同步和安全检查 | 从 Cursor 扩展市场安装后跟随市场版本 |

仓库中的
[`publish-marketplaces.yml`](../.github/workflows/publish-marketplaces.yml)
已经把三个市场接到 Release 流程。账号、协议和密钥只需配置一次；以后更新
三个源码版本并推送 `v*` tag，CI 会先校验版本、构建插件，再发布所有已经配置
凭据的市场。

> 以前通过 **Install Plugin from Disk** 或 **Install from VSIX** 安装的用户，
> 不一定会自动切换到市场更新通道。市场上线后，建议卸载手动安装版本，再从
> 对应市场安装一次。VS Code 官方明确说明，从 VSIX 安装的扩展默认关闭自动
> 更新。

## 一、JetBrains Marketplace

JetBrains 要求第一个版本必须人工上传，不能用 API 或 Gradle 完成。完成一次
人工上传后，后续版本才能由 `publishPlugin` 自动发布。

### 1. 创建发布者并准备页面资料

1. 登录 [JetBrains Marketplace](https://plugins.jetbrains.com/author/me)。
2. 创建 Vendor Profile，填写可用的邮箱和网站。
3. 接受 Marketplace Developer Agreement。
4. 准备以下资料：
   - 名称：`Autocurl`
   - Plugin ID：`com.github.lingbohuang.autocurl`
   - License：MIT
   - Source URL：`https://github.com/Lingbo-Huang/autocurl`
   - Vendor：Huang Lingbo
   - Tags：Developer Tools、Debugging、HTTP Client
5. 仓库已经提供 Marketplace 要求的 40×40 SVG 图标、英文优先的描述、中文
   快速上手和开源许可证。

### 2. 给首次上传包签名

密钥只放本机安全目录和 GitHub Secrets，绝对不要提交到仓库。

```bash
mkdir -p "$HOME/.autocurl-signing"
cd "$HOME/.autocurl-signing"

openssl genpkey -aes-256-cbc -algorithm RSA \
  -out private_encrypted.pem -pkeyopt rsa_keygen_bits:4096
openssl rsa -in private_encrypted.pem -out private.pem
openssl req -key private.pem -new -x509 -days 365 -out chain.crt
```

回到仓库，向 Gradle 提供证书、私钥和刚才设置的密码：

```bash
cd ide/jetbrains
export CERTIFICATE_CHAIN_FILE="$HOME/.autocurl-signing/chain.crt"
export PRIVATE_KEY_FILE="$HOME/.autocurl-signing/private.pem"
read -s PRIVATE_KEY_PASSWORD
export PRIVATE_KEY_PASSWORD
./gradlew signPlugin verifyPluginSignature
```

将 `build/distributions/` 中生成的签名 ZIP，通过 Marketplace 个人页面的
**Upload plugin** 人工上传。选择 Vendor Profile，填写资料并提交审核。
JetBrains 会人工审核首个版本和后续更新。

### 3. 配置后续自动发布

在 Marketplace Profile 的
[My Tokens](https://plugins.jetbrains.com/author/me/tokens) 生成永久 token。
等首次插件上传完成后，在仓库根目录配置 GitHub Actions Secrets：

```bash
gh secret set JETBRAINS_PUBLISH_TOKEN
gh secret set JETBRAINS_CERTIFICATE_CHAIN \
  < "$HOME/.autocurl-signing/chain.crt"
gh secret set JETBRAINS_PRIVATE_KEY \
  < "$HOME/.autocurl-signing/private.pem"
gh secret set JETBRAINS_PRIVATE_KEY_PASSWORD
```

`JETBRAINS_PUBLISH_TOKEN` 和密码命令会安全地等待你粘贴/输入，不会写进
shell history。四个 Secret 缺少任意一个时，自动流程会跳过 JetBrains，避免
发布未签名的包。

## 二、Visual Studio Marketplace

扩展清单中的 publisher 已设置为 `lingbo-huang`。创建 Publisher 时必须使用
完全相同的 ID；如果该 ID 已被占用，需要先修改
`ide/vscode/package.json` 中的 `publisher`。

1. 在 [Manage Publishers & Extensions](https://marketplace.visualstudio.com/manage/publishers/)
   创建 Publisher `lingbo-huang`。
2. 在 Azure DevOps 创建 Personal Access Token：
   - Organization 选择 **All accessible organizations**；
   - Scope 选择 **Marketplace → Manage**。
3. 把 token 存入仓库：

   ```bash
   gh secret set VSCE_PAT
   ```

配置完成后，打 tag 时 CI 会执行：

```bash
npx vsce publish --packagePath autocurl-<version>.vsix
```

Microsoft 已宣布 Azure DevOps 全局 PAT 将在 **2026-12-01** 退役，并推荐
迁移到 Microsoft Entra ID 的安全自动发布。当前工作流保留 PAT 路径用于首次
快速上架；需要在退役日前将该步骤切换成 `vsce publish --azure-credential`。

## 三、Open VSX / Cursor

Cursor 的编辑器扩展库以 Open VSX 为底层来源，并有自己的同步和安全检查。
这和 Cursor 新推出的 Agent Plugin Marketplace 不是同一种插件；Autocurl 是
VS Code/Cursor 编辑器扩展，应发布到 Open VSX。

1. 创建 [Eclipse Account](https://accounts.eclipse.org/user/register)，并在
   账户资料中绑定 GitHub 用户名。
2. 使用 GitHub 登录 [Open VSX](https://open-vsx.org/)，连接 Eclipse 账户并
   签署 Open VSX Publisher Agreement。
3. 在 Open VSX User Settings 创建 Access Token。
4. 首次创建与 `package.json` 完全一致的 namespace：

   ```bash
   cd ide/vscode
   OVSX_PAT='<token>' npx ovsx create-namespace lingbo-huang
   ```

5. 把 token 存入 GitHub：

   ```bash
   gh secret set OVSX_PAT
   ```

之后 CI 会发布同一份 VSIX：

```bash
npx ovsx publish --skip-duplicate autocurl-<version>.vsix
```

Open VSX 发布成功后，Cursor 可能需要一段时间完成同步和扫描。如果始终搜不到，
再到 Cursor 社区的 Extension Verification 分类提交 Open VSX 页面申请验证。

## 四、以后怎样发一个新版本

三个发布版本和两个 IDE 引擎最低版本必须一致，Release 工作流会拒绝任意一处
不一致的 tag：

- `internal/cli/cli.go` 中的 `Version`
- `ide/vscode/package.json` 中的 `version`
- `ide/jetbrains/gradle.properties` 中的 `pluginVersion`
- `ide/vscode/src/binary.ts` 中的 `minimumVersion`
- `ide/jetbrains/.../EngineManager.java` 中的 `MINIMUM_VERSION`

以 `0.3.0` 为例：

```bash
bash scripts/check-release-version.sh v0.3.0
git tag v0.3.0
git push origin v0.3.0
```

Release 流程会：

1. 校验三个版本号；
2. 运行构建和插件结构检查；
3. 创建 GitHub Release；
4. 对已经配置凭据的市场执行自动发布；
5. 对未配置凭据的市场给出 warning 并安全跳过。

某个市场临时失败时不需要重打 tag。进入 GitHub
**Actions → Publish IDE Marketplaces → Run workflow**，填写已存在的 tag，
并只选择 `jetbrains`、`vscode` 或 `openvsx` 重试。

同一市场不接受相同版本重复上传。必须修复问题、递增版本号并重新发布；Open
VSX 自动重试使用了 `--skip-duplicate`，所以已经存在的版本不会让整次重试失败。

## 五、官方资料与容易误解的点

- JetBrains 新插件上传：
  https://plugins.jetbrains.com/docs/marketplace/uploading-a-new-plugin.html
- JetBrains 更新与审核：
  https://plugins.jetbrains.com/docs/marketplace/plugin-updates.html
- VS Code 打包、Publisher、发布、Pricing 与 Sponsor：
  https://code.visualstudio.com/api/working-with-extensions/publishing-extension
- Open VSX：
  https://github.com/eclipse-openvsx/openvsx

Cursor 的编辑器扩展市场与它的 Agent Plugin Marketplace 是两套机制。Autocurl
当前发布的是 VS Code 编辑器扩展，目标是 Open VSX。Cursor 还会做自己的同步、
兼容性和安全检查，可能短时展示旧版；Release 中的 VSIX 始终保留为确定性的
手动安装降级。
