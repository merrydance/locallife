# merchant_app

A new Flutter project.

## Android release signing

This project expects a local Android release keystore for store builds.

1. Generate a keystore locally:

```bash
keytool -genkeypair -v \
	-keystore android/release-keystore.jks \
	-keyalg RSA \
	-keysize 2048 \
	-validity 10000 \
	-alias release
```

2. Copy `android/key.properties.example` to `android/key.properties`.
3. Fill in your real passwords, alias, and keystore path.
4. Build a signed release artifact with `flutter build apk --release` or `flutter build appbundle --release`.

Notes:

- Keep the keystore and passwords outside version control.
- Use the same release keystore for Google Play and domestic Android stores so future updates stay installable.
- If you later enable Google Play App Signing, keep this keystore as your upload key or migration source.

## Android vendor push config

Release builds require native push credentials by default:

- `XIAOMI_APP_ID`, `XIAOMI_APP_KEY`
- `OPPO_APP_KEY`, `OPPO_APP_SECRET`
- `VIVO_APP_ID`, `VIVO_APP_KEY`
- `HONOR_APP_ID`

Provide them as Gradle properties or environment variables before a production release. Internal non-production APK builds may explicitly set `ALLOW_INCOMPLETE_PUSH_CONFIG=true`.

## Getting Started

This project is a starting point for a Flutter application.

A few resources to get you started if this is your first Flutter project:

- [Learn Flutter](https://docs.flutter.dev/get-started/learn-flutter)
- [Write your first Flutter app](https://docs.flutter.dev/get-started/codelab)
- [Flutter learning resources](https://docs.flutter.dev/reference/learning-resources)

For help getting started with Flutter development, view the
[online documentation](https://docs.flutter.dev/), which offers tutorials,
samples, guidance on mobile development, and a full API reference.
