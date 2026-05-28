import java.util.Properties

plugins {
    id("com.android.application")
    id("kotlin-android")
    // The Flutter Gradle Plugin must be applied after the Android and Kotlin Gradle plugins.
    id("dev.flutter.flutter-gradle-plugin")
}

val keystoreProperties = Properties()
val keystorePropertiesFile = rootProject.file("key.properties")
val hasReleaseSigning = keystorePropertiesFile.exists()

if (hasReleaseSigning) {
    keystorePropertiesFile.inputStream().use(keystoreProperties::load)
}

fun requireKeystoreProperty(name: String): String =
    keystoreProperties.getProperty(name)?.takeIf { it.isNotBlank() }
        ?: error("android/key.properties 缺少 $name 配置")

val releaseSigningMissingMessage =
    "Release signing is not configured. Create android/key.properties from android/key.properties.example and generate your release keystore before building release artifacts."

fun pushProperty(name: String): String =
    providers.gradleProperty(name)
        .orElse(providers.environmentVariable(name))
        .orElse(providers.provider { keystoreProperties.getProperty(name) ?: "" })
        .orElse("")
        .get()

val pushPlaceholders = mapOf(
    "XIAOMI_APP_ID" to pushProperty("XIAOMI_APP_ID"),
    "XIAOMI_APP_KEY" to pushProperty("XIAOMI_APP_KEY"),
    "OPPO_APP_KEY" to pushProperty("OPPO_APP_KEY"),
    "OPPO_APP_SECRET" to pushProperty("OPPO_APP_SECRET"),
    "VIVO_APP_ID" to pushProperty("VIVO_APP_ID"),
    "VIVO_APP_KEY" to pushProperty("VIVO_APP_KEY"),
    "HONOR_APP_ID" to pushProperty("HONOR_APP_ID"),
)

val enforceProductionPushConfig =
    providers.gradleProperty("ENFORCE_PRODUCTION_PUSH_CONFIG")
        .orElse(providers.environmentVariable("ENFORCE_PRODUCTION_PUSH_CONFIG"))
        .map { it.equals("true", ignoreCase = true) || it == "1" }
        .orElse(true)
        .get()

val allowIncompletePushConfig =
    providers.gradleProperty("ALLOW_INCOMPLETE_PUSH_CONFIG")
        .orElse(providers.environmentVariable("ALLOW_INCOMPLETE_PUSH_CONFIG"))
        .map { it.equals("true", ignoreCase = true) || it == "1" }
        .orElse(false)
        .get()

android {
    namespace = "com.merrydance.locallife.merchant"
    compileSdk = flutter.compileSdkVersion
    ndkVersion = flutter.ndkVersion

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
        isCoreLibraryDesugaringEnabled = true
    }

    kotlinOptions {
        jvmTarget = JavaVersion.VERSION_17.toString()
    }

    signingConfigs {
        if (hasReleaseSigning) {
            create("release") {
                storeFile = rootProject.file(requireKeystoreProperty("storeFile"))
                storePassword = requireKeystoreProperty("storePassword")
                keyAlias = requireKeystoreProperty("keyAlias")
                keyPassword = requireKeystoreProperty("keyPassword")
            }
        }
    }

    defaultConfig {
        // TODO: Specify your own unique Application ID (https://developer.android.com/studio/build/application-id.html).
        applicationId = "com.merrydance.locallife.merchant"
        // You can update the following values to match your application needs.
        // For more information, see: https://flutter.dev/to/review-gradle-config.
        minSdk = flutter.minSdkVersion
        targetSdk = flutter.targetSdkVersion
        versionCode = flutter.versionCode
        versionName = flutter.versionName
        multiDexEnabled = true

        manifestPlaceholders += pushPlaceholders
    }

    buildTypes {
        release {
            if (hasReleaseSigning) {
                signingConfig = signingConfigs.getByName("release")
            }
        }
    }
}

tasks.configureEach {
    if (name.contains("Release", ignoreCase = true)) {
        doFirst {
            check(hasReleaseSigning) { releaseSigningMissingMessage }
            if (enforceProductionPushConfig && !allowIncompletePushConfig) {
                val missing = pushPlaceholders.filterValues { it.isBlank() }.keys
                check(missing.isEmpty()) {
                    "Production push config is incomplete: ${missing.joinToString()}. Provide Gradle properties/environment variables before building a production release, or set ALLOW_INCOMPLETE_PUSH_CONFIG=true only for internal non-production APK builds."
                }
            }
        }
    }
}

flutter {
    source = "../.."
}

dependencies {
    coreLibraryDesugaring("com.android.tools:desugar_jdk_libs:2.1.4")

    // 小米 / vivo 客户端 SDK 由官方 AAR 放入 libs/。
    implementation(fileTree(mapOf("dir" to "libs", "include" to listOf("*.jar", "*.aar"))))

    // 荣耀推送 (Maven)
    implementation("com.hihonor.mcs:push:7.0.41.301")

    // OPPO / Heytap Push (Volcengine Maven)
    implementation("com.heytap.msp:push:3.0.0")
}
