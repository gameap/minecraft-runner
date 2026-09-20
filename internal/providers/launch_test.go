package providers

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func touch(t *testing.T, dir string, name ...string) {
	t.Helper()

	path := filepath.Join(append([]string{dir}, name...)...)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestForgeResolveLaunch(t *testing.T) {
	provider := NewForgeProvider()

	t.Run("nothing installed", func(t *testing.T) {
		jar := &ServerJar{Version: "1.20.1", ModVersion: "47.4.10"}
		if target := provider.ResolveLaunch(t.TempDir(), jar); target != nil {
			t.Errorf("target = %+v, want nil", target)
		}
	})

	t.Run("modular Forge starts from its argument file", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "libraries", "net", "minecraftforge", "forge", "1.20.1-47.4.10", argFileName())

		target := provider.ResolveLaunch(dir, &ServerJar{Version: "1.20.1", ModVersion: "47.4.10"})

		want := &LaunchTarget{ArgFiles: []string{"libraries/net/minecraftforge/forge/1.20.1-47.4.10/" + argFileName()}}
		if !reflect.DeepEqual(target, want) {
			t.Errorf("target = %+v, want %+v", target, want)
		}
	})

	t.Run("the argument file wins over the shim JAR next to it", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "libraries", "net", "minecraftforge", "forge", "1.21.1-52.1.0", argFileName())
		touch(t, dir, "forge-1.21.1-52.1.0-shim.jar")

		target := provider.ResolveLaunch(dir, &ServerJar{Version: "1.21.1", ModVersion: "52.1.0"})
		if target == nil || len(target.ArgFiles) != 1 || target.Jar != "" {
			t.Errorf("target = %+v, want an argument file launch", target)
		}
	})

	t.Run("another Forge version does not count as installed", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "libraries", "net", "minecraftforge", "forge", "1.20.1-47.4.10", argFileName())
		touch(t, dir, "forge-1.16.5-36.2.34.jar")

		if target := provider.ResolveLaunch(dir, &ServerJar{Version: "1.20.1", ModVersion: "47.4.23"}); target != nil {
			t.Errorf("target = %+v, want nil", target)
		}
	})

	t.Run("legacy Forge runs the JAR the installer left behind", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "forge-1.12.2-14.23.5.2859.jar")
		touch(t, dir, "forge-1.12.2-14.23.5.2859-installer.jar")
		touch(t, dir, "minecraft_server.1.12.2.jar")

		target := provider.ResolveLaunch(dir, &ServerJar{Version: "1.12.2", ModVersion: "14.23.5.2859"})

		want := &LaunchTarget{Jar: "forge-1.12.2-14.23.5.2859.jar"}
		if !reflect.DeepEqual(target, want) {
			t.Errorf("target = %+v, want %+v", target, want)
		}
	})

	t.Run("old releases name their JAR with extra suffixes", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "forge-1.7.10-10.13.4.1614-1.7.10-universal.jar")

		target := provider.ResolveLaunch(dir, &ServerJar{Version: "1.7.10", ModVersion: "10.13.4.1614"})

		want := &LaunchTarget{Jar: "forge-1.7.10-10.13.4.1614-1.7.10-universal.jar"}
		if !reflect.DeepEqual(target, want) {
			t.Errorf("target = %+v, want %+v", target, want)
		}
	})

	t.Run("an installer alone is not a launch target", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "forge-1.12.2-14.23.5.2859-installer.jar")

		if target := provider.ResolveLaunch(dir, &ServerJar{Version: "1.12.2", ModVersion: "14.23.5.2859"}); target != nil {
			t.Errorf("target = %+v, want nil", target)
		}
	})
}

func TestNeoForgeResolveLaunch(t *testing.T) {
	provider := NewNeoForgeProvider()

	t.Run("modern", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "libraries", "net", "neoforged", "neoforge", "21.1.251", argFileName())

		target := provider.ResolveLaunch(dir, &ServerJar{Version: "1.21.1", ModVersion: "21.1.251"})

		want := &LaunchTarget{ArgFiles: []string{"libraries/net/neoforged/neoforge/21.1.251/" + argFileName()}}
		if !reflect.DeepEqual(target, want) {
			t.Errorf("target = %+v, want %+v", target, want)
		}
	})

	t.Run("1.20.1 keeps the Forge layout", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "libraries", "net", "neoforged", "forge", "1.20.1-47.1.106", argFileName())

		target := provider.ResolveLaunch(dir, &ServerJar{Version: "1.20.1", ModVersion: "47.1.106"})

		want := &LaunchTarget{ArgFiles: []string{"libraries/net/neoforged/forge/1.20.1-47.1.106/" + argFileName()}}
		if !reflect.DeepEqual(target, want) {
			t.Errorf("target = %+v, want %+v", target, want)
		}
	})

	t.Run("not installed", func(t *testing.T) {
		if target := provider.ResolveLaunch(t.TempDir(), &ServerJar{Version: "1.21.1", ModVersion: "21.1.251"}); target != nil {
			t.Errorf("target = %+v, want nil", target)
		}
	})
}

func TestQuiltResolveLaunch(t *testing.T) {
	provider := NewQuiltProvider()
	jar := &ServerJar{Version: "1.21.1", ModVersion: "0.30.1"}

	t.Run("installed", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "quilt-server-launch-1.21.1-0.30.1.jar")
		touch(t, dir, "server.jar")

		want := &LaunchTarget{Jar: "quilt-server-launch-1.21.1-0.30.1.jar"}
		if target := provider.ResolveLaunch(dir, jar); !reflect.DeepEqual(target, want) {
			t.Errorf("target = %+v, want %+v", target, want)
		}
	})

	t.Run("a launcher of another version does not count", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "quilt-server-launch-1.20.4-0.30.1.jar")
		touch(t, dir, "server.jar")

		if target := provider.ResolveLaunch(dir, jar); target != nil {
			t.Errorf("target = %+v, want nil", target)
		}
	})

	t.Run("a launcher without the vanilla server does not count", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "quilt-server-launch-1.21.1-0.30.1.jar")

		if target := provider.ResolveLaunch(dir, jar); target != nil {
			t.Errorf("target = %+v, want nil", target)
		}
	})
}

func TestProxyProviders(t *testing.T) {
	proxies := map[string]bool{
		"velocity": true, "waterfall": true, "bungeecord": true,
		"paper": false, "folia": false, "forge": false, "vanilla": false,
	}

	all := []Provider{
		NewVelocityProvider(), NewWaterfallProvider(), NewBungeecordProvider(),
		NewPaperProvider(), NewFoliaProvider(), NewForgeProvider(), NewVanillaProvider(),
	}

	for _, provider := range all {
		if got := IsProxy(provider); got != proxies[provider.Name()] {
			t.Errorf("IsProxy(%s) = %v, want %v", provider.Name(), got, proxies[provider.Name()])
		}
	}

	if got := NewVelocityProvider().ListenArgs(25577); !reflect.DeepEqual(got, []string{"--port", "25577"}) {
		t.Errorf("velocity ListenArgs = %v", got)
	}
	if got := NewVelocityProvider().ListenArgs(0); got != nil {
		t.Errorf("velocity ListenArgs(0) = %v, want nil", got)
	}
	if got := NewWaterfallProvider().ListenArgs(25577); got != nil {
		t.Errorf("waterfall ListenArgs = %v, want nil", got)
	}
}
