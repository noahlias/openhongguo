plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "dev.openhongguo.app"
    compileSdk = 36
    // Strips the native libraries: nsh.aar's libnsh.so keeps its symbols, ~23 MB an ABI.
    // The same NDK builds the Go core (bindings/mobile/build.sh takes the newest installed).
    ndkVersion = "29.0.14206865"

    defaultConfig {
        applicationId = "dev.openhongguo.app"
        // Covers are HEIC, which Android decodes from 28 on.
        minSdk = 28
        targetSdk = 36
        versionCode = 1
        versionName = "0.1.0"
        buildConfigField("String", "NSH_DEV_SERVER", "\"\"")
    }

    buildFeatures { buildConfig = true }

    // One APK an ABI: libnsh.so alone is ~47 MB per ABI once stripped. nsh.aar ships these three;
    // the Go core and libmpv also build x86, which would go unused.
    splits {
        abi {
            isEnable = true
            reset()
            include("arm64-v8a", "armeabi-v7a", "x86_64")
            isUniversalApk = false
        }
    }

    buildTypes {
        debug {
            // `./gradlew installDebug -Pnsh.dev=<url nsh dev prints>` loads from `nsh dev` instead of the bundle.
            buildConfigField("String", "NSH_DEV_SERVER", "\"${providers.gradleProperty("nsh.dev").getOrElse("")}\"")
        }
        release {
            isMinifyEnabled = false
            // Sideloaded builds: signed with the debug key until the app has a release key.
            signingConfig = signingConfigs.getByName("debug")
        }
    }

    packaging {
        // libmpv brings its own libc++_shared.so; nothing else in the app does today.
        jniLibs.pickFirsts += "**/libc++_shared.so"
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}

kotlin {
    compilerOptions {
        jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17)
    }
}

dependencies {
    // gomobile's classes (dev.openhongguo.core, go.*); the .so files are in src/main/jniLibs.
    implementation(files("libs/core.jar"))
    // nsh-dev.aar adds connectDevServer, and allows cleartext for the dev server.
    debugImplementation(files("libs/nsh-dev.aar"))
    releaseImplementation(files("libs/nsh.aar"))
    implementation("dev.jdtech.mpv:libmpv:1.0.0")
}
