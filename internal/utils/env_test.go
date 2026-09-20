package utils

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestJavaEnvironment(t *testing.T) {
	sep := string(os.PathListSeparator)
	javaHome := filepath.Join(string(filepath.Separator)+"opt", "mcrun", "java", "jdk-17")
	javaBinary := filepath.Join(javaHome, "bin", "java")

	t.Run("the chosen Java leads PATH and becomes JAVA_HOME", func(t *testing.T) {
		got := javaEnvironment([]string{"HOME=/root", "PATH=/usr/bin", "JAVA_HOME=/usr/lib/jvm/old"}, javaBinary)

		want := []string{
			"HOME=/root",
			"PATH=" + filepath.Join(javaHome, "bin") + sep + "/usr/bin",
			"JAVA_HOME=" + javaHome,
		}

		sort.Strings(got)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("env = %v, want %v", got, want)
		}
	})

	t.Run("an environment without PATH gets one", func(t *testing.T) {
		got := javaEnvironment([]string{"HOME=/root"}, javaBinary)

		want := []string{"HOME=/root", "PATH=" + filepath.Join(javaHome, "bin"), "JAVA_HOME=" + javaHome}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("env = %v, want %v", got, want)
		}
	})

	t.Run("a bare command name changes nothing", func(t *testing.T) {
		environ := []string{"PATH=/usr/bin", "JAVA_HOME=/usr/lib/jvm/old"}
		if got := javaEnvironment(environ, "java"); !reflect.DeepEqual(got, environ) {
			t.Errorf("env = %v, want it untouched", got)
		}
	})

	t.Run("a binary outside a bin directory sets no JAVA_HOME", func(t *testing.T) {
		got := javaEnvironment([]string{"PATH=/usr/bin"}, filepath.Join(string(filepath.Separator)+"custom", "java"))

		want := []string{"PATH=" + string(filepath.Separator) + "custom" + sep + "/usr/bin"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("env = %v, want %v", got, want)
		}
	})
}
