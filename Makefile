# openhongguo 的统一入口：构建、生成类型和测试。make help 列出所有目标。

# gomobile 和 gobind 装在 GOPATH/bin。
export PATH := $(shell go env GOPATH)/bin:$(PATH)
# macOS 上的默认位置；已经设置时沿用。
ANDROID_HOME ?= $(HOME)/Library/Android/sdk
export ANDROID_HOME
ifeq ($(JAVA_HOME),)
ifneq ($(wildcard /Applications/Android\ Studio.app/Contents/jbr/Contents/Home),)
export JAVA_HOME := /Applications/Android Studio.app/Contents/jbr/Contents/Home
endif
endif

.PHONY: help build cli node mobile gen test test-go test-node test-react-native test-flutter \
	test-mobile test-android test-ios desktop install-desktop test-desktop

help: ## 列出所有目标
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "} {printf "  %-18s %s\n", $$1, $$2}'

build: cli node mobile ## 构建 CLI、Node.js 包和全部移动端包

cli: ## 构建命令行工具 bin/ohg
	go build -o bin/ohg ./cmd/ohg

node: ## 构建 Node.js 包（wasm）
	cd bindings/node && npm run build

mobile: ## 编译 Go core，构建 Android、iOS、React Native 和 Flutter 包
	bindings/mobile/build.sh all

gen: ## 修改 sdk 的类型或注释后，重新生成各语言的类型和文档
	go run ./bindings/node/tsgen > bindings/node/types.d.ts
	go run ./bindings/node/tsgen > bindings/react-native/src/types.ts
	go run ./bindings/mobile/gen
	go run ./bindings/docgen

desktop: ## 构建 Linux 桌面客户端 desktop/target/release/ohg-desktop（需要 Go、Rust 和 libmpv）
	cargo build --release --manifest-path desktop/Cargo.toml

# 装到 ~/.local：程序、启动器条目和图标。PREFIX=/usr/local 可改位置。
PREFIX ?= $(HOME)/.local
install-desktop: desktop ## 安装桌面客户端和启动器条目
	install -Dm755 desktop/target/release/ohg-desktop $(PREFIX)/bin/ohg-desktop
	install -Dm644 desktop/assets/openhongguo.svg $(PREFIX)/share/icons/hicolor/scalable/apps/openhongguo.svg
	install -d $(PREFIX)/share/applications
	sed 's|^Exec=ohg-desktop|Exec=$(PREFIX)/bin/ohg-desktop|' desktop/assets/openhongguo.desktop \
		> $(PREFIX)/share/applications/openhongguo.desktop

test-desktop: ## 桌面客户端的 rustfmt、clippy 和测试
	cd desktop && cargo fmt --check && cargo clippy --all-targets -- -D warnings && cargo test

test: test-go test-node test-react-native test-flutter ## 运行不需要移动端工具链的全部测试

test-go: ## gofmt、go vet 和 Go 测试，包括移动端的 Go core
	test -z "$$(gofmt -l $$(git ls-files '*.go'))"
	go vet ./...
	go test ./...
	cd bindings/mobile/core && go test ./...

test-node: ## 构建 Node.js 包并运行测试
	cd bindings/node && npm run build && npm test

test-react-native: ## 编译 React Native 包并运行测试
	cd bindings/react-native && (test -d node_modules || npm ci) && npm test

test-flutter: ## 运行 Flutter 包的测试
	cd bindings/flutter && flutter test

test-mobile: test-android test-ios ## 运行需要 gomobile、Android SDK 和 Xcode 的测试

test-android: ## 编译 Android 版 core，运行 Kotlin 单元测试并打包 aar
	bindings/mobile/build.sh android

test-ios: ## 编译 iOS 版 core，在 macOS 上运行 Swift 测试
	bindings/mobile/build.sh ios
	cd bindings/mobile/ios && swift test
