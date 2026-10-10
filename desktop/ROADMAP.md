# SyncthingApp 开发计划（P3 及以后）

> 记录时间：2026-10-09
> 当前基线：仓库 HEAD `5fb192176`（未提交的 App 代码见下）
> 详细使用说明见 `desktop/README.md`

## 一、当前状态

### 已完成

| 阶段 | 内容 | 状态 |
|---|---|---|
| P0 | 嵌入 syncthing 核心、事件桥、无头 CLI 验证 | ✅ 已验证 |
| P1 | Windows 桌面 MVP：Wails 窗口 + 托盘 + 拖放共享 + 设备配对 | ✅ 已验证 |
| P2 | 首次运行向导、便携模式、自启动代码、单实例、日志页 | ✅ 已验证（自启动/向导待人工确认） |
| — | 中英双语（跟随系统 + 手动切换） | ✅ 已验证 |

### 交付物

```
E:\Dev\syncthing-app\dist\
├── SyncthingApp-windows-amd64.zip     绿色便携包（分发这个）
└── syncthing-app-windows-amd64.exe
```

构建：`go run -tags tools ./buildapp -zip`

### 已写但**未验证**的部分（下次继续时优先补）

1. **首次运行向导是否真的拦得住**
   - 现象：6 次自动化测试中，程序都在约 9 秒后自行写入 `Data/.setup-complete`
   - 日志线索：`First-run setup completed syncDir=D:\CompanyKB\王琼 deviceName=POO`
   - 已排查：`SetupStatus`、`CompleteSetup`、嵌入的 JS 包均无自动调用点
   - 判断：疑似该测试环境的自动化交互产物，但**未排除真实 bug**
   - 待办：手动双击全新解压的包，确认停在欢迎页

2. **开机自启动端到端**
   - 代码已写（`HKCU\...\Run` + `--minimized`），未实测（避免污染用户注册表）
   - 待办：设置页勾选 → 注销重登 → 确认托盘出现

3. **语言切换实时刷新**
   - 逻辑通，但需真实点击下拉框验证
   - 已知限制：**托盘菜单文本运行时不变**（`fyne.io/systray` 不支持动态改文本），重启后生效

4. **托盘图标外观与菜单点击**
   - 只确认了 `SystrayClass` 窗口注册成功，图标颜色/交互未肉眼确认

5. **真实拖放**（`EnableFileDrop` + `OnFileDrop`）
   - 文件选择器路径已验证，拖拽路径未验证

### 其它已知问题 / 技术债

- **日志无滚动**：`appcore/core.go` 用 `O_APPEND` 写 `syncthing-app.log`，永不切分，长期运行会无限增长
- **移动文件夹是整体复制后删除，没有进度条**：大目录跨盘移动时界面看起来像卡住了（`appcore/folders.go` 的 `moveFolderData`）。确认框里报了大小，但没有进度反馈；超大数据集需要考虑边移动边同步的方案
- **消息模板少了占位符只能用眼睛看**：`t()` 把没给值的 `{name}` 原样留在文案里，不报错。`check.mjs` 现已静态校验「每个字面量 `t()` 调用都填满了对应文案里的占位符」，以及「带占位符的键都有人调用」（运行时拼接的 `folder.`/`code.` 前缀单独放行，并要求该前缀确实出现在某个 `t()` 拼接里）。Go 侧 `%v` 与调用实参个数的对应关系仍未校验，只能靠 `TestTranslateAllCodes` 那种「都渲染一次」的方式间接发现
- **内建中转站的地址是编译期常量**：`appcore/hub.go` 的 `hubAddress = tcp://101.69.254.66:22000`。换服务器 / 换端口 / 改成域名都要重新发版；且该地址是明文写死在二进制里的。若以后要支持多入口或热切换，需要改成配置项 + 一次「内置默认值」的首次写入
- **中转站的「同步给新文件夹」开关只作用于文件夹自己的设备列表**：syncthing 的每个文件夹各自保存设备列表，模板只决定「新建时带哪些设备」。因此：
  - 关掉开关只影响以后新建的文件夹；已在同步到中转站的文件夹保持原样（有意为之：把中转站从同步中的文件夹里移除会连带删掉服务器上的那份）
  - 若用户在别处改了文件夹的设备列表，模板与文件夹会分叉——这是 syncthing 本身的模型，不是本项目的 bug
- **文件夹移动的「同盘=瞬时」判断是近似的**：Windows 按盘符，macOS 按 `/Volumes` 挂载点，Linux 按设备号（`filesystem_*.go`）。mkdir 挂载的同一盘符会被误判为同盘，结果只是多复制一次，不会出错
- **数据目录迁移不搬共享文件夹**：这是有意的（共享文件夹可能在别的盘、体积很大），确认框里已写明；迁移只复制数据目录本身（config/cert/index-v2/log），旧目录保留作备份，需用户手动删
- **迁移后 `locations` 里的文件夹模板路径不重写**：`config.xml` 原样复制，其中 `Defaults.Folder.Path` 指向旧数据目录下的 `Sync/`。已共享的文件夹不受影响（路径是绝对的），但「接受别人的文件夹」时的默认落地位置仍指向旧目录。`AcceptPendingFolder` 走的是 `DataDir()/Sync/<id>`（`desktop/app.go:403`），所以实际行为正确，只有模板值本身是陈旧的
- ~~`dist/` 只被 `.gitignore` 的 `*.zip` 覆盖~~ 已核实：上游 `.gitignore` 同时含 `*.zip` 和 `*.exe`，`dist/` 下的产物全部被忽略，无需额外规则（早先看到的 `?? dist/` 来自一个构建残留文件 `syncthing-app-windows-amd64.exe~`，已删除）
- **macOS/Linux GUI 无法在本机交叉编译**：Wails 在 Linux 链接 GTK/WebKit、macOS 链接 Cocoa，`fyne.io/systray` 两端都是 cgo。`buildapp` 已能给出明确报错，但产物必须在对应平台构建
- **`lib/syncthing/internals.go` 是唯一改动的上游文件**（新增 `PendingDevices()` 转发）。升级 syncthing 时需重新核对 `Internals` 是否变化

## 二、P3：macOS / Linux 桌面

### 前置约束（重要）

- Apple 机器不在本环境，macOS 部分**只能写代码、无法编译验证**
- Linux 可交叉编译，但需要 gcc + GTK/WebKit/appindicator 开发库；托盘运行还需要 `libayatana-appindicator`
- 因此 P3 的产出应标注「已实现但未在目标平台验证」

### 工作项

1. **构建环境**
   - Linux：`build-essential libgtk-3-dev libwebkit2gtk-4.0-dev libayatana-appindicator3-dev`
   - macOS：Xcode command line tools（clang）
   - 验证 `go run -tags tools ./buildapp -goos <os> -zip` 在两个平台各跑通

2. **macOS 打包**
   - 生成 `.app` bundle：`Contents/{MacOS/, Info.plist, Resources/AppIcon.icns}`
   - `Info.plist` 关键项：`CFBundleIdentifier`、`CFBundleIconFile`、`LSMinimumSystemVersion`
   - **不要**设 `LSUIElement=true`（需要主窗口）
   - codesign：先 ad-hoc（`-s -`），脚本预留 Developer ID + notarize 参数
   - 自启动走 `~/Library/LaunchAgents/com.syncthingapp.launcher.plist`（代码已写）
   - 注意：未签名包会被 Gatekeeper 拦，需在文档说明「右键打开」

3. **Linux 打包**
   - AppImage（linuxdeploy）+ 随包 `libayatana-appindicator3.so`
   - `.desktop` 文件 + 图标安装到 `usr/share/`
   - 兜底：`tar.gz{ binary, syncthingapp.desktop }`
   - 自启动走 `~/.config/autostart/syncthingapp.desktop`（代码已写）

4. **平台适配验证点**
   - 托盘：GNOME 默认无托盘区，需 appindicator 扩展；Wayland 下也需实测
   - 拖放：Wayland 由 WebKitGTK 处理，需确认 `OnFileDrop` 有事件
   - 数据目录：Linux `~/.local/state/syncthingapp`、macOS `~/Library/Application Support/SyncthingApp`（代码已写，未验证）
   - 路径大小写：`samePath()` 已对 darwin 做 `EqualFold`，需在大小写敏感卷上确认行为

## 三、P4：Android

### 前置 spike（必须先做）

`gomobile bind` 强制开启 cgo，SQLite 会从 modernc.org/sqlite 切到 `mattn/go-sqlite3`（需 NDK）。**先验证**：

```
gomobile bind -target=android -androidapi 26 ./mobile
```
能否编译 + 打开数据库 + 读写一条记录。

### 绑定层设计（`mobile/core.go`，`package mobilecore`）

gomobile 只导出有限类型，所以全部用 **JSON 字符串 + 回调接口**：

```go
func Start(configDir string) error
func Stop() error
func MyID() string
func GetSnapshotJSON() string
func AddDevice(idJSON string) error
func AddFolderFromPath(path, label string, accepted bool) error
func SetIgnores(folderID, patternsJSON string) error
func GetLogs(tailLines int) string
type Listener interface{ OnState(snapJSON string) }
func SetStateListener(l Listener)
func SetLanguage(lang string) error
```

原则：`mobilecore` 零业务逻辑，只是薄翻译层。`appcore` 的 `Snapshot` JSON 结构与桌面端**完全一致**，前端 TS 类型可共享。

### 主要工程量（不在 Go 侧）

- Android 后台保活：前台服务 + 常驻通知 + 电池优化白名单引导
- Doze 模式、厂商杀后台策略

## 四、P5：iOS

- `gomobile bind -target=ios` → XC framework + SwiftUI 壳
- iOS 后台限制严格，只能「打开时同步」模式
- App Store 审核需说明后台行为

## 五、可选的后续增强（未排期）

- 日志滚动（按天/按大小切分）—— 目前是已确认的技术债
- 应用内检查更新（只提示，不自动，因为已三重禁用 auto-upgrade）
- 托盘菜单运行时跟随语言变化
- 嵌套共享的更强提示（目前只是 warning）
- 首次运行向导里检测 22000 端口占用并提示（原计划有，实现时判断为过度设计而删除，需要的话可重新加）
- ~~`dist/` 加入 `.gitignore`~~（已核实无需；见「其它已知问题」）

## 六、关键代码位置速查

| 目的 | 文件 |
|---|---|
| 嵌入组装序列（对照 `cmd/syncthing/main.go:406-568`） | `appcore/core.go` |
| 事件桥 / 状态聚合 | `appcore/state.go` |
| 翻译目录（Go 侧） | `appcore/i18n.go` |
| 翻译目录（前端） | `desktop/frontend/dist/i18n.js` |
| 托盘状态机 | `desktop/tray/tray.go` |
| 系统语言检测 | `appcore/locale.go` + `locale_{windows,other}.go` |
| 自启动三平台 | `appcore/autostart/` |
| 打包 | `buildapp/main.go` |
| 上游唯一改动 | `lib/syncthing/internals.go`（+4 行） |

## 七、构建 / 验证命令

```bash
# 构建便携包
go run -tags tools ./buildapp -zip

# 测试与检查
go test ./appcore/...
go vet -tags "noassets" ./appcore/... ./desktop/...
gofmt -l appcore desktop buildapp cmd/syncthing-app cmd/syncthing-cli

# 手工构建（两个 tag 缺一不可）
go build -tags "noassets,production" -ldflags "-H windowsgui" -o app.exe ./cmd/syncthing-app
```

**两个 build tag 的作用**：
- `production` — Wails 必需，否则编译出只弹错误框的桩
- `noassets` — 跳过嵌入上游 Web GUI（本 App 不提供）
