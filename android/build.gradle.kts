// The Android app: an nsh view (ui/, TSX) over the Go core (gomobile, as bindings/mobile) and libmpv.
// `make android` puts the prebuilt parts into app/libs, app/src/main/jniLibs and app/src/main/assets.
plugins {
    id("com.android.application") version "8.13.2" apply false
    id("org.jetbrains.kotlin.android") version "2.1.20" apply false
}
