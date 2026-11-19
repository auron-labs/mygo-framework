package generator

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

const phoneBuildPlatformEnvironment = "MYGO_PHONE_BUILD_PLATFORM"

// phoneBuildOutputs is the artifact each phone platform writes beneath build/.
var phoneBuildOutputs = map[string]string{
	"android":       "android/*.apk",
	"ios/simulator": "ios-simulator/*.app",
}

// TestGeneratedPhoneBuilds builds both starters for the phone platform named by
// MYGO_PHONE_BUILD_PLATFORM, which needs that platform's toolchain: the Android
// SDK, NDK and a JDK, or Xcode on a Mac. Builds are unsigned or debug-signed.
func TestGeneratedPhoneBuilds(t *testing.T) {
	platform := os.Getenv(phoneBuildPlatformEnvironment)
	if platform == "" {
		t.Skip("set " + phoneBuildPlatformEnvironment + " to android or ios/simulator to build the starters for phones")
	}
	output, ok := phoneBuildOutputs[platform]
	if !ok {
		t.Fatalf("%s = %q, want android or ios/simulator", phoneBuildPlatformEnvironment, platform)
	}

	repository := repositoryRoot(t)
	directory := t.TempDir()
	framework := filepath.Join(directory, "framework")
	if runtime.GOOS == "windows" {
		framework += ".exe"
	}
	runAcceptanceCommand(t, repository, "go", "build", "-o", framework, "./cmd/framework")

	for _, starter := range []struct {
		name      string
		directory string
		arguments []string
		web       bool
	}{
		{name: "WebPhoneApp", directory: "web-phone-app", web: true},
		{name: "NativePhoneApp", directory: "native-phone-app", arguments: []string{"--native"}},
	} {
		t.Run(starter.name, func(t *testing.T) {
			runAcceptanceCommand(t, directory, framework, append([]string{"new", starter.name, "--framework", repository}, starter.arguments...)...)
			project := filepath.Join(directory, starter.directory)
			if starter.web {
				runAcceptanceCommand(t, project, "bun", "install", "--frozen-lockfile")
			}
			runAcceptanceCommand(t, project, framework, "build", "-platform", platform)

			built, err := filepath.Glob(filepath.Join(project, "build", filepath.FromSlash(output)))
			if err != nil || len(built) == 0 {
				t.Fatalf("no %s under %s/build after building for %s: %v", output, starter.name, platform, err)
			}
		})
	}
}
