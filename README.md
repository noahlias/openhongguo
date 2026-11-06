# openhongguo 桌面版

[![desktop](https://github.com/noahlias/openhongguo/actions/workflows/desktop.yml/badge.svg)](https://github.com/noahlias/openhongguo/actions/workflows/desktop.yml)

红果短剧的非官方桌面客户端，支持 Linux、macOS 和 Windows。浏览、搜索、在窗口里直接播放，记住每部剧看到哪一集。

本仓库是 [beilunyang/openhongguo](https://github.com/beilunyang/openhongguo) 的分支。客户端用 [GPUI](https://www.gpui.rs) 写界面，用 libmpv 播放，数据来自上游的 Go SDK。

> **免责声明**：本项目仅供学习和技术研究，禁止用于商业用途或任何非法用途。短剧、图片等内容的权利归原权利人所有。使用本项目即表示您同意文末的[免责声明](#免责声明)。

## 下载

在 [Releases](https://github.com/noahlias/openhongguo/releases) 下载对应平台的压缩包。还没有正式版本时，可以在 [Actions](https://github.com/noahlias/openhongguo/actions/workflows/desktop.yml) 里打开最新一次成功的构建，从页面底部的 Artifacts 下载（需要登录 GitHub）。

| 平台 | 文件 | 怎么运行 |
|---|---|---|
| Linux x86_64 | `openhongguo-desktop_*_linux-x86_64.tar.gz` | 先装 libmpv 和 ffmpeg（Arch：`mpv ffmpeg`，Debian/Ubuntu：`libmpv2 ffmpeg`），解压后运行 `./ohg-desktop` |
| macOS 15+（Apple 芯片） | `openhongguo-desktop_*_macos-arm64.dmg` | 打开 dmg，把 `openhongguo.app` 拖进“应用程序”，第一次打开前执行 `xattr -cr /Applications/openhongguo.app` |
| Windows x64 | `openhongguo-desktop_*_windows-x86_64.zip` | 解压后运行 `ohg-desktop.exe`，旁边的 DLL 和 `ffmpeg.exe` 不要删。SmartScreen 拦截时点“更多信息 → 仍要运行” |

安装包没有签名，所以 macOS 和 Windows 第一次打开时会拦截。macOS 和 Windows 的包自带 libmpv 和 ffmpeg（封面是 HEIC 格式，靠 ffmpeg 解码）。Linux 版在 Ubuntu 24.04 上构建，需要 glibc 2.39 或更新的系统。

## 功能

- 推荐、搜索、短剧详情、继续观看、观看记录和设置
- 窗口内播放：续播、自动下一集、清晰度切换、选集、全屏
- 布局随窗口宽度变化，窄窗口时改用底部标签栏
- 深色和浅色主题，中文和英文界面
- 观看记录和命令行工具 `ohg` 共用，桌面端看到哪里，`ohg continue` 就从哪里接着播

## 快捷键

| 按键 | 作用 |
|---|---|
| `Ctrl+K`（macOS 上是 `⌘K`） | 搜索 |
| 空格 | 播放或暂停 |
| `←` `→` | 后退或快进 5 秒 |
| `↑` `↓` | 音量 |
| `m` | 静音 |
| `f`，双击画面 | 全屏 |
| `n` `p` | 下一集或上一集 |
| `Esc` | 关闭菜单、退出全屏、返回 |
| `F12` | 调试面板 |

## 数据存放位置

设置（`desktop.json`）、匿名设备和观看记录都在同一个目录里。设置环境变量 `OHG_CONFIG_DIR` 可以换到别的目录。

| 平台 | 目录 |
|---|---|
| Linux | `~/.config/openhongguo` |
| macOS | `~/Library/Application Support/openhongguo` |
| Windows | `%APPDATA%\openhongguo` |

## 从源码构建

需要 Go 1.26+、稳定版 Rust 和 libmpv。各平台完整的构建步骤见 [desktop.yml](.github/workflows/desktop.yml)。

| 平台 | 准备 |
|---|---|
| Linux | 装 `mpv ffmpeg`（Arch）或 `libmpv-dev ffmpeg`（Debian/Ubuntu）。中文字体建议装 Noto CJK |
| macOS | `brew install mpv ffmpeg pkgconf`（没有 pkgconf 时链接器找不到 libmpv）；打包成 dmg 用 [bundle-macos.sh](desktop/scripts/bundle-macos.sh)（还需要 `dylibbundler`） |
| Windows | 用 MSVC 版 Rust；cgo 需要 MinGW 的 `gcc`；从 [mpv-winbuild-cmake](https://github.com/shinchiro/mpv-winbuild-cmake/releases) 下载 `mpv-dev-x86_64-*`，生成 `mpv.lib`，并设置 `MPV_LIBRARY_DIR` |

```sh
cd desktop
cargo build --release      # 产物在 desktop/target/release/
cargo test --release
```

Linux 上还可以用根目录的 `make install-desktop`，它会把客户端装到 `~/.local`，并添加启动器条目和图标。

## 其他文档

| 文档 | 内容 |
|---|---|
| [SDK 文档](docs/sdk.md) | 上游的 Go、Node.js、Android、iOS、React Native 和 Flutter SDK |
| [命令行文档](docs/cli.md) | 命令行工具 `ohg` 和它的全屏界面 |
| [开发说明](docs/development.md) | 接口契约检查和持续集成 |

## 免责声明

本项目是技术研究与学习工具，用于学习移动应用接口协议的分析与调用。

- 使用时请遵守中华人民共和国相关法律法规。因使用本项目引起的法律责任由使用者自行承担。
- 严禁用于非法目的、商业用途，或侵犯他人知识产权和其他合法权益的行为。
- 通过本项目访问的短剧、图片、文字等内容，著作权归原权利人所有。不得复制、传播或公开展示这些内容；研究完成后请及时删除。
- 开发者不对使用本项目造成的任何直接或间接损失负责。

完整条款见 [SDK 文档的免责声明](docs/sdk.md#免责声明)。

## 许可证

Apache License 2.0，全文见 [LICENSE](LICENSE)。SDK 和命令行工具版权归 beilunyang（2026）。
