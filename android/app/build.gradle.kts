plugins {
  alias(libs.plugins.android.application)
  alias(libs.plugins.compose.compiler)
  alias(libs.plugins.kotlin.serialization)
}

// The release version, from gradle.properties; the release workflow checks the tag against it.
val appVersion = providers.gradleProperty("tabfinderVersion").get()
val (major, minor, patch) = appVersion.split(".").map { it.toInt() }

android {
    namespace = "dev.tabsync.tabfinder"
    compileSdk = 36
    defaultConfig {
        applicationId = "dev.tabsync.tabfinder"
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
        minSdk = 29
        targetSdk = 36
        versionCode = major * 10000 + minor * 100 + patch
        versionName = appVersion
        // tabscan is built for arm64 (mise run bin). CI's x86_64 emulators get an x86_64 build instead
        // (-PtabscanAbis=x86_64, see .github/workflows/test.yml).
        ndk { abiFilters += providers.gradleProperty("tabscanAbis").getOrElse("arm64-v8a").split(",") }
    }

    // The release key, from ~/.gradle/gradle.properties locally and from secrets in the release
    // workflow. Without it, release builds are signed with the debug key.
    val keystore = providers.gradleProperty("tabfinderKeystore").orNull
    signingConfigs {
        if (keystore != null) {
            create("release") {
                storeFile = file(keystore)
                storePassword = providers.gradleProperty("tabfinderKeystorePassword").get()
                keyAlias = "tabfinder"
                keyPassword = storePassword
            }
        }
    }

    buildTypes {
        // Debug builds install next to the real app, so device tests never touch its saved folder and scan.
        debug {
            applicationIdSuffix = ".debug"
        }
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
            // Debug builds scroll badly (no R8).
            signingConfig = signingConfigs.findByName("release") ?: signingConfigs.getByName("debug")
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_21
        targetCompatibility = JavaVersion.VERSION_21
    }
    buildFeatures {
      compose = true
      aidl = false
      buildConfig = false
      shaders = false
    }

    // Robolectric runs the screen in the JVM tests; it needs the app's resources.
    testOptions { unitTests { isIncludeAndroidResources = true } }
    // The made-up tab library, shared by the JVM and the device tests.
    sourceSets {
        named("test") { kotlin.directories.add("src/sharedTest/java") }
        named("androidTest") { kotlin.directories.add("src/sharedTest/java") }
    }

    packaging {
      resources {
        excludes += "/META-INF/{AL2.0,LGPL2.1}"
      }
      // Extract libtabscan.so to nativeLibraryDir: the only place Android lets apps execute files.
      jniLibs {
        useLegacyPackaging = true
      }
    }
}

kotlin {
    jvmToolchain(21)
}

// The JVM tests run the host build of tabscan as the real backend (see data/FinderTest.kt).
val hostTabscan = layout.buildDirectory.file("tabscan-host/tabscan")
val buildHostTabscan by tasks.registering(Exec::class) {
    description = "Builds tabscan for this machine, the backend of the JVM tests"
    val repo = rootDir.parentFile
    workingDir = repo
    commandLine("go", "build", "-o", hostTabscan.get().asFile.path, "./cmd/tabscan")
    inputs.files(fileTree(repo.resolve("cmd/tabscan")), fileTree(repo.resolve("internal")), repo.resolve("go.mod"))
    outputs.file(hostTabscan)
}
tasks.withType<Test>().configureEach {
    dependsOn(buildHostTabscan)
    systemProperty("tabscan.binary", hostTabscan.get().asFile.path)
    // Robolectric reads FileDescriptor's internals.
    jvmArgs("--add-opens=java.base/java.io=ALL-UNNAMED", "--add-exports=java.base/jdk.internal.access=ALL-UNNAMED")
}

dependencies {
  val composeBom = platform(libs.androidx.compose.bom)
  implementation(composeBom)
  androidTestImplementation(composeBom)

  // Core Android dependencies
  implementation(libs.androidx.core.ktx)
  implementation(libs.androidx.lifecycle.runtime.ktx)
  implementation(libs.androidx.activity.compose)

  // Arch Components
  implementation(libs.androidx.lifecycle.runtime.compose)
  implementation(libs.androidx.lifecycle.viewmodel.compose)

  // Compose
  implementation(libs.androidx.compose.ui)
  implementation(libs.androidx.compose.ui.tooling.preview)
  implementation(libs.androidx.compose.material3)
  // Tooling
  debugImplementation(libs.androidx.compose.ui.tooling)
  // Instrumented tests
  androidTestImplementation(libs.androidx.compose.ui.test.junit4)
  debugImplementation(libs.androidx.compose.ui.test.manifest)

  // Local tests: jUnit, coroutines, and Robolectric for the screen
  testImplementation(libs.junit)
  testImplementation(libs.kotlinx.coroutines.test)
  testImplementation(libs.robolectric)
  testImplementation(composeBom)
  testImplementation(libs.androidx.compose.ui.test.junit4)
  testImplementation(libs.androidx.test.core)
  testImplementation(libs.androidx.test.ext.junit)

  // Instrumented tests: jUnit rules and runners
  androidTestImplementation(libs.androidx.test.core)
  androidTestImplementation(libs.androidx.test.ext.junit)
  androidTestImplementation(libs.androidx.test.runner)
  androidTestImplementation(libs.androidx.test.espresso.core)
  androidTestImplementation(libs.androidx.test.espresso.intents)
  androidTestImplementation(libs.androidx.test.rules)

  implementation(libs.kotlinx.serialization.json)
}
