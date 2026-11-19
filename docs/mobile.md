# Phones and tablets

MyGo builds the same app for iOS and Android as for the desktop: pages show in
the system web view, native `ui.View`s draw with Metal or OpenGL ES, and the Go
code, services and events don't change. `framework` delegates MyGo's phone
commands, so `framework dev -platform ios` and the rest below work in both
starters. Desktop builds are unchanged and need none of it.

## Prerequisites

- **iOS**: a Mac with Xcode. The Go code links with Xcode's clang, and MyGo
  compiles and signs the app without an Xcode project. Devices and the App
  Store need an Apple developer team and a provisioning profile for the app's
  identifier.
- **Android**: the Android SDK and NDK, and a JDK. MyGo finds the SDK in its
  usual place or `ANDROID_HOME`, the NDK in the SDK or `ANDROID_NDK_HOME`, and
  Android Studio's JDK or `JAVA_HOME`.

Phone builds link with cgo; desktop builds stay cgo-free. Run `framework doctor`
in a project for what is installed and what is missing, with how to get it. A
missing phone toolchain is a warning, not a failure, so desktop work goes on.

## Devices and development

`framework devices` lists what `-device` takes, by name and ID: iOS simulators
and devices, Android devices and emulators, and emulators that are not running.
It doesn't regenerate adapters.

```sh
framework devices
framework dev -platform ios        # the booted simulator
framework dev -platform android    # the device or emulator adb reaches
framework dev -platform ios -device "iPhone 16"   # a simulator by name or UDID
framework dev -platform android -device emulator-5554
```

Web projects can use the scripts generated in `package.json` instead:

```sh
bun run dev:ios
bun run dev:android
```

`framework dev` passes the flags to MyGo, which builds, installs and launches
the app and shows its output. The development app is `<name> Dev` with the
identifier `<identifier>.dev`, so it sits beside a release install. A frontend's
`devUrl` works as it does on the desktop; a physical Android device reaches it
through `adb reverse`. An iOS device needs Developer Mode, and a `devUrl` on
`localhost` must be reachable on the Mac's network address.

## Building and signing

```sh
framework build -platform ios              # build/ios-arm64/<slug>-<version>.ipa, signed
framework build -platform ios/simulator    # build/ios-simulator/<name>.app
framework build -platform android          # build/android/<slug>-<version>.apk and .aab
framework build -platform android/arm64    # one ABI: arm64, arm or amd64
```

The starters set `"out": "build"`, so outputs land there.

The `.ipa` is signed with the installed provisioning profile that best fits the
app's identifier and `ios.distribution` (`app-store` by default), whose
certificate is in the keychain; `ios.team` names the developer team. Release
Android builds are signed with `android.keystore` and `keyAlias`, whose
passwords come from `MYGO_ANDROID_KEYSTORE_PASSWORD` and
`MYGO_ANDROID_KEY_PASSWORD`; without them MyGo uses the debug key, which Google
Play refuses.

Put the settings in `mygo.json` for native projects and `mygo.config.ts` for web
projects:

```ts
export default defineConfig({
  // ...
  ios: { team: "ABCDE12345", infoPlist: { NSCameraUsageDescription: "Scan receipts." } },
  android: { keystore: "upload.jks", keyAlias: "upload", permissions: ["CAMERA"] },
});
```

The native starter's `mygo.json` sets `ios.minimumSystemVersion` 17.0 and
`android.minSdk` 26. The web starter's `mygo.config.ts` carries commented
`ios` and `android` examples, and MyGo applies the same defaults when the
fields are unset; `package.json` in web projects also has the `dev:ios` and
`dev:android` scripts. Adjust the other `ios` and `android` fields as MyGo's
[configuration reference](https://github.com/egoist/mygo/blob/v0.10.0/docs/configuration.md)
describes.

## Publishing

`framework publish` uploads what `framework build` wrote for the version, so
build first:

```sh
framework build -platform ios && framework publish -platform ios
framework build -platform android && framework publish -platform android -track beta
```

For web projects, `framework publish` regenerates the adapters before
delegating, as `framework dev` and `framework build` do.

iOS uploads the `.ipa` to App Store Connect with `xcrun altool`, where it shows
in TestFlight once processed. It reads an App Store Connect API key:

| Variable | |
| --- | --- |
| `MYGO_ASC_KEY_ID` | the key's ID |
| `MYGO_ASC_ISSUER_ID` | the issuer's ID |
| `MYGO_ASC_KEY` | the path of its `.p8` |

Android uploads the `.aab` through Google Play's Android Publisher API.
`MYGO_PLAY_SERVICE_ACCOUNT` holds the path of a service account's JSON key, or
the JSON itself, for CI secrets; the account needs access to the app in Play
Console. `-track` is `internal` by default, or `alpha`, `beta`, `production` or
a closed testing track's name.

## Reporting errors

Phones have no launch environment to read `SENTRY_DSN` from, so compile the
DSN into a packaged build with `MYGO_LDFLAGS`; MyGo appends it to phone builds.

```sh
MYGO_LDFLAGS="-X github.com/auron-labs/mygo-framework/errorreport.buildDSN=https://<key>@<host>/<project>" framework build -platform ios
```

The variable, the release workflow and how reporting is enabled are in
[Reporting errors](native.md#reporting-errors).
