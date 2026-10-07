plugins {
  alias(libs.plugins.android.application)
  alias(libs.plugins.compose.compiler)
  alias(libs.plugins.kotlin.serialization)
}

android {
    namespace = "dev.tabsync.tabfinder"
    compileSdk = 36
    defaultConfig {
        applicationId = "dev.tabsync.tabfinder"
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
        minSdk = 29
        targetSdk = 36
        versionCode = 1
        versionName = "1.0"
        // tabscan is built for arm64 (mise run bin). CI's x86_64 emulators get an x86_64 build instead
        // (-PtabscanAbis=x86_64, see .github/workflows/test.yml).
        ndk { abiFilters += providers.gradleProperty("tabscanAbis").getOrElse("arm64-v8a").split(",") }
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
            // Sideloaded personal app: the debug key is enough. Debug builds scroll badly (no R8).
            signingConfig = signingConfigs.getByName("debug")
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

  // Local tests: jUnit, coroutines, Android runner
  testImplementation(libs.junit)
  testImplementation(libs.kotlinx.coroutines.test)

  // Instrumented tests: jUnit rules and runners
  androidTestImplementation(libs.androidx.test.core)
  androidTestImplementation(libs.androidx.test.ext.junit)
  androidTestImplementation(libs.androidx.test.runner)
  androidTestImplementation(libs.androidx.test.espresso.core)
  androidTestImplementation(libs.androidx.test.espresso.intents)
  androidTestImplementation(libs.androidx.test.rules)

  implementation(libs.kotlinx.serialization.json)
}
