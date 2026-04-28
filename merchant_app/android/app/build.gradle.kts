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

        manifestPlaceholders += mapOf(
            // Xiaomi Push
            "XIAOMI_APP_ID" to "1000000", // TODO: 替换为实际 ID
            "XIAOMI_APP_KEY" to "5000000000000", // TODO: 替换为实际 KEY
            // OPPO Push
            "OPPO_APP_KEY" to "xxx",
            "OPPO_APP_SECRET" to "xxx",
            // VIVO Push
            "VIVO_APP_ID" to "xxx",
            "VIVO_APP_KEY" to "xxx"
        )
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
        }
    }
}

flutter {
    source = "../.."
}

dependencies {
    coreLibraryDesugaring("com.android.tools:desugar_jdk_libs:2.1.4")

    // 小米推送 (Maven 仓库访问失败，请检查网络或使用本地依赖)
    // implementation("com.xiaomi.mipush.sdk:mipush:5.0.8-C")

    // 荣耀推送 (Maven)
    implementation("com.hihonor.mcs:push:7.0.41.301")

    // vivo 推送 (vivo 不提供公共 Maven 仓库，请将 SDK AAR 文件放入 libs/ 目录并使用本地依赖)
    // implementation("com.vivo.push:sdk:3.0.0.4")

    // OPPO 推送 (由于 OPPO SDK 常规不发 Maven，通常需要放在 libs/ 下)
    // implementation(fileTree(mapOf("dir" to "libs", "include" to listOf("*.jar", "*.aar"))))
}
