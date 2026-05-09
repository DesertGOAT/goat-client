// Gradle settings for the goat-client Android shell.
//
// The :app module wraps the gomobile-bound goat-client.aar
// (built by `gomobile bind` against ../GoatClientSDK — see
// mobile/android/README.md for the build pipeline).

pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}

dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        google()
        mavenCentral()
        // The gomobile-built AAR lives next to the app module's
        // build.gradle.kts at app/libs/goat-client.aar. flatDir is
        // the path Android Studio uses for sideloaded AARs.
        flatDir { dirs("app/libs") }
    }
}

rootProject.name = "goat-client"
include(":app")
