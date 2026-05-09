// goat-client Android shell — :app module.
//
// Wraps the gomobile-bound goat-client.aar (built from
// ../../GoatClientSDK with `gomobile bind`; see
// mobile/android/README.md for the pipeline).

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "io.dlf_dds.goat_client"
    compileSdk = 34

    defaultConfig {
        applicationId = "io.dlf_dds.goat_client"
        minSdk = 24      // Android 7.0 — covers >97% devices, predates the
                         // pidfd-seccomp-policy issues that the engine
                         // works around at runtime.
        targetSdk = 34
        versionCode = 1
        versionName = "0.0.1-pre"
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro",
            )
            // Engineering builds ship unsigned. Play Store / Internal
            // Track signing comes in the v1.5 mobile-release track.
        }
        debug {
            isDebuggable = true
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions {
        jvmTarget = "17"
    }
    buildFeatures {
        viewBinding = true
    }
    packaging {
        resources {
            excludes += "/META-INF/{AL2.0,LGPL2.1}"
        }
    }
}

dependencies {
    // The gomobile-built AAR. Resolved via the flatDir repo declared
    // in settings.gradle.kts. The file itself is .gitignore'd; build
    // it with `gomobile bind` per mobile/android/README.md.
    implementation(files("libs/goat-client.aar"))

    implementation("androidx.core:core-ktx:1.13.1")
    implementation("androidx.appcompat:appcompat:1.7.0")
    implementation("androidx.activity:activity-ktx:1.9.2")
    implementation("androidx.lifecycle:lifecycle-runtime-ktx:2.8.6")
    implementation("androidx.lifecycle:lifecycle-viewmodel-ktx:2.8.6")
    implementation("com.google.android.material:material:1.12.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.8.1")

    testImplementation("junit:junit:4.13.2")
    androidTestImplementation("androidx.test.ext:junit:1.2.1")
}
