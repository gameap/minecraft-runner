package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gameap/minecraft-runner/internal/config"
	"github.com/gameap/minecraft-runner/internal/providers"
)

// fakeProvider is a providers.Provider double whose installer writes the
// files listed in installs
type fakeProvider struct {
	name       string
	proxy      bool
	jar        *providers.ServerJar
	resolveErr error
	launch     *providers.LaunchTarget
	installs   []string

	installCalls int
}

func (p *fakeProvider) Name() string  { return p.name }
func (p *fakeProvider) IsProxy() bool { return p.proxy }

func (p *fakeProvider) ListVersions(context.Context) ([]providers.VersionInfo, error) {
	return nil, nil
}

func (p *fakeProvider) ListModVersions(context.Context, string) ([]providers.VersionInfo, error) {
	return nil, nil
}

func (p *fakeProvider) GetLatestVersion(context.Context) (*providers.VersionInfo, error) {
	if p.resolveErr != nil {
		return nil, p.resolveErr
	}
	return &providers.VersionInfo{MinecraftVersion: p.jar.Version}, nil
}

func (p *fakeProvider) GetServerJar(context.Context, string, string) (*providers.ServerJar, error) {
	if p.resolveErr != nil {
		return nil, p.resolveErr
	}
	return p.jar, nil
}

func (p *fakeProvider) PostDownload(_ context.Context, dir string, _ *providers.ServerJar, _ string) error {
	p.installCalls++
	for _, file := range p.installs {
		path := filepath.Join(dir, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(path, nil, 0644); err != nil {
			return err
		}
	}
	return nil
}

func (p *fakeProvider) GetRecommendedJavaVersion(context.Context, string) int { return 21 }

// fakeInstallerProvider also resolves its own launch target, as Forge does
type fakeInstallerProvider struct {
	fakeProvider
}

func (p *fakeInstallerProvider) ResolveLaunch(dir string, _ *providers.ServerJar) *providers.LaunchTarget {
	if !launchFilesExist(dir, *p.launch) {
		return nil
	}
	return p.launch
}

func newTestRunner(provider providers.Provider) *Runner {
	registry := providers.NewRegistry()
	registry.Register(provider)

	return NewRunner(registry, nil, &config.Config{})
}

func writeFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("jar"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestBuildArgs(t *testing.T) {
	cfg := &config.Config{}
	cfg.Defaults.Memory = "2G"
	cfg.Defaults.MinMemory = "1G"
	cfg.Server.JVMArgs = []string{"-XX:+UseG1GC"}

	runner := &Runner{config: cfg}

	t.Run("plain JAR under a control panel", func(t *testing.T) {
		inst := &installation{launch: providers.LaunchTarget{Jar: "paper-1.21.8-60.jar"}}

		got := runner.buildArgs(&fakeProvider{name: "paper"}, RunOptions{JVMArgs: []string{"-Dfoo=bar"}}, inst, false)

		want := []string{
			"-Xmx2G", "-Xms1G", "-Dterminal.jline=false", "-XX:+UseG1GC", "-Dfoo=bar",
			"-jar", "paper-1.21.8-60.jar", "--nogui",
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("args = %v\nwant   %v", got, want)
		}
	})

	t.Run("a terminal keeps the JLine console", func(t *testing.T) {
		inst := &installation{launch: providers.LaunchTarget{Jar: "server.jar"}}

		got := runner.buildArgs(&fakeProvider{name: "paper"}, RunOptions{}, inst, true)

		for _, arg := range got {
			if arg == "-Dterminal.jline=false" {
				t.Errorf("args = %v, JLine must stay enabled on a terminal", got)
			}
		}
	})

	t.Run("argument file launch with the user JVM arguments of a server pack", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, userJVMArgsFile)

		inst := &installation{
			launch:     providers.LaunchTarget{ArgFiles: []string{"libraries/net/minecraftforge/forge/1.20.1-47.4.10/unix_args.txt"}},
			serverArgs: []string{"nogui"},
		}

		got := runner.buildArgs(&fakeProvider{name: "forge"}, RunOptions{Directory: dir, Memory: "6G"}, inst, false)

		want := []string{
			"@user_jvm_args.txt", "-Xmx6G", "-Xms1G", "-Dterminal.jline=false", "-XX:+UseG1GC",
			"@libraries/net/minecraftforge/forge/1.20.1-47.4.10/unix_args.txt", "nogui",
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("args = %v\nwant   %v", got, want)
		}
	})

	t.Run("argument file launch without a user JVM arguments file", func(t *testing.T) {
		inst := &installation{launch: providers.LaunchTarget{ArgFiles: []string{"libraries/x/unix_args.txt"}}}

		got := runner.buildArgs(&fakeProvider{name: "forge"}, RunOptions{Directory: t.TempDir()}, inst, true)

		if got[0] != "-Xmx2G" {
			t.Errorf("args = %v, must not reference a missing %s", got, userJVMArgsFile)
		}
	})

	t.Run("proxies get no GUI switch", func(t *testing.T) {
		inst := &installation{launch: providers.LaunchTarget{Jar: "BungeeCord-2096.jar"}}

		got := runner.buildArgs(&fakeProvider{name: "bungeecord", proxy: true}, RunOptions{Port: 25577}, inst, true)

		want := []string{"-Xmx2G", "-Xms1G", "-XX:+UseG1GC", "-jar", "BungeeCord-2096.jar"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("args = %v\nwant   %v", got, want)
		}
	})

	t.Run("Velocity takes its port on the command line", func(t *testing.T) {
		inst := &installation{launch: providers.LaunchTarget{Jar: "velocity.jar"}}

		got := runner.buildArgs(providers.NewVelocityProvider(), RunOptions{Port: 25577}, inst, true)

		want := []string{"-Xmx2G", "-Xms1G", "-XX:+UseG1GC", "-jar", "velocity.jar", "--port", "25577"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("args = %v\nwant   %v", got, want)
		}
	})
}

func TestMemorySettings(t *testing.T) {
	cfg := &config.Config{}
	cfg.Defaults.Memory = "2G"
	cfg.Defaults.MinMemory = "1G"

	tests := []struct {
		name             string
		runner           *Runner
		opts             RunOptions
		wantMax, wantMin string
	}{
		{"config defaults", &Runner{config: cfg}, RunOptions{}, "2G", "1G"},
		{"flags win", &Runner{config: cfg}, RunOptions{Memory: "8G", MinMemory: "4G"}, "8G", "4G"},
		{"built-in defaults", &Runner{}, RunOptions{}, "1G", "1G"},
		{"an initial heap above the maximum is lowered", &Runner{config: cfg}, RunOptions{Memory: "512M"}, "512M", "512M"},
		{"units are compared, not strings", &Runner{}, RunOptions{Memory: "2G", MinMemory: "1024M"}, "2G", "1024M"},
		{"unparseable values pass through", &Runner{}, RunOptions{Memory: "lots", MinMemory: "4G"}, "lots", "4G"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMax, gotMin := tt.runner.memorySettings(tt.opts)
			if gotMax != tt.wantMax || gotMin != tt.wantMin {
				t.Errorf("memorySettings() = %s/%s, want %s/%s", gotMax, gotMin, tt.wantMax, tt.wantMin)
			}
		})
	}
}

func TestParseMemory(t *testing.T) {
	tests := []struct {
		value string
		want  int64
		ok    bool
	}{
		{"512M", 512 << 20, true},
		{"4g", 4 << 30, true},
		{"1024k", 1024 << 10, true},
		{"1048576", 1048576, true},
		{" 2G ", 2 << 30, true},
		{"", 0, false},
		{"G", 0, false},
		{"-1G", 0, false},
		{"1.5G", 0, false},
	}

	for _, tt := range tests {
		got, ok := parseMemory(tt.value)
		if got != tt.want || ok != tt.ok {
			t.Errorf("parseMemory(%q) = %d, %v; want %d, %v", tt.value, got, ok, tt.want, tt.ok)
		}
	}
}

func TestInstall(t *testing.T) {
	t.Run("a plain JAR that is already there is not fetched again", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "paper-1.21.8-60.jar")

		provider := &fakeProvider{name: "paper", jar: &providers.ServerJar{
			Version: "1.21.8", Filename: "paper-1.21.8-60.jar", URL: "http://127.0.0.1:1/unreachable",
		}}

		target, err := newTestRunner(provider).install(context.Background(), provider, provider.jar, "java", dir)
		if err != nil {
			t.Fatalf("install: %v", err)
		}
		if target.Jar != "paper-1.21.8-60.jar" {
			t.Errorf("target = %+v", target)
		}
	})

	t.Run("an installed version does not run its installer again", func(t *testing.T) {
		dir := t.TempDir()
		argFile := "libraries/net/minecraftforge/forge/1.20.1-47.4.10/unix_args.txt"

		provider := &fakeInstallerProvider{fakeProvider{
			name: "forge",
			jar: &providers.ServerJar{
				Version: "1.20.1", ModVersion: "47.4.10", Filename: "forge-installer.jar",
				URL: "http://127.0.0.1:1/unreachable", RequiresInstall: true,
			},
			launch:   &providers.LaunchTarget{ArgFiles: []string{argFile}},
			installs: []string{argFile},
		}}
		runner := newTestRunner(provider)

		// The installer is "downloaded" already, so the first run only has to install
		writeFile(t, dir, "forge-installer.jar")

		for run := 1; run <= 2; run++ {
			target, err := runner.install(context.Background(), provider, provider.jar, "java", dir)
			if err != nil {
				t.Fatalf("run %d: install: %v", run, err)
			}
			if !reflect.DeepEqual(target.ArgFiles, []string{argFile}) {
				t.Errorf("run %d: target = %+v", run, target)
			}

			// The real installers delete themselves; the second run must not need them
			os.Remove(filepath.Join(dir, "forge-installer.jar"))
		}

		if provider.installCalls != 1 {
			t.Errorf("installer ran %d times, want 1", provider.installCalls)
		}
	})

	t.Run("an installer that leaves nothing to launch is an error", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "forge-installer.jar")

		provider := &fakeInstallerProvider{fakeProvider{
			name:   "forge",
			jar:    &providers.ServerJar{Version: "1.20.1", Filename: "forge-installer.jar", RequiresInstall: true},
			launch: &providers.LaunchTarget{ArgFiles: []string{"libraries/missing/unix_args.txt"}},
		}}

		if _, err := newTestRunner(provider).install(context.Background(), provider, provider.jar, "java", dir); err == nil {
			t.Error("expected an error")
		}
	})
}

func TestInstalledFallback(t *testing.T) {
	cause := errors.New("unexpected status code 410")

	setup := func(t *testing.T) (string, *Runner) {
		t.Helper()

		dir := t.TempDir()
		writeFile(t, dir, "paper-1.21.8-60.jar")

		state := &State{
			Mod: "paper", Version: "1.21.8", ModVersion: "60", File: "paper-1.21.8-60.jar",
			Launch: providers.LaunchTarget{Jar: "paper-1.21.8-60.jar"},
		}
		if err := state.Save(dir); err != nil {
			t.Fatal(err)
		}

		return dir, newTestRunner(&fakeProvider{name: "paper", resolveErr: cause})
	}

	t.Run("the installed server starts when the API is down", func(t *testing.T) {
		dir, runner := setup(t)

		for _, opts := range []RunOptions{
			{Mod: "paper", Directory: dir},
			{Mod: "paper", Version: "1.21.8", Directory: dir},
			{Mod: "paper", Version: "1.21.8", ModVersion: "60", Directory: dir},
		} {
			inst, err := runner.installedFallback(opts, cause)
			if err != nil {
				t.Fatalf("installedFallback(%+v): %v", opts, err)
			}
			if inst.version != "1.21.8" || inst.launch.Jar != "paper-1.21.8-60.jar" {
				t.Errorf("inst = %+v", inst)
			}
		}
	})

	t.Run("a different request is not answered with the installed server", func(t *testing.T) {
		dir, runner := setup(t)

		for _, opts := range []RunOptions{
			{Mod: "purpur", Directory: dir},
			{Mod: "paper", Version: "1.21.10", Directory: dir},
			{Mod: "paper", Version: "1.21.8", ModVersion: "61", Directory: dir},
		} {
			if _, err := runner.installedFallback(opts, cause); !errors.Is(err, cause) {
				t.Errorf("installedFallback(%+v) err = %v, want the resolve error", opts, err)
			}
		}
	})

	t.Run("a state whose files are gone is useless", func(t *testing.T) {
		dir, runner := setup(t)
		os.Remove(filepath.Join(dir, "paper-1.21.8-60.jar"))

		if _, err := runner.installedFallback(RunOptions{Mod: "paper", Directory: dir}, cause); !errors.Is(err, cause) {
			t.Errorf("err = %v, want the resolve error", err)
		}
	})

	t.Run("no state at all", func(t *testing.T) {
		runner := newTestRunner(&fakeProvider{name: "paper", resolveErr: cause})

		if _, err := runner.installedFallback(RunOptions{Mod: "paper", Directory: t.TempDir()}, cause); !errors.Is(err, cause) {
			t.Errorf("err = %v, want the resolve error", err)
		}
	})
}

func TestRecordInstall(t *testing.T) {
	runner := newTestRunner(&fakeProvider{name: "paper"})

	record := func(dir, file, modVersion string) {
		writeFile(t, dir, file)
		runner.recordInstall(
			RunOptions{Mod: "paper", Directory: dir},
			&providers.ServerJar{Version: "1.21.8", ModVersion: modVersion, Filename: file},
			&installation{version: "1.21.8", launch: providers.LaunchTarget{Jar: file}},
		)
	}

	dir := t.TempDir()
	record(dir, "paper-1.21.8-59.jar", "59")
	record(dir, "paper-1.21.8-60.jar", "60")

	if _, err := os.Stat(filepath.Join(dir, "paper-1.21.8-59.jar")); !os.IsNotExist(err) {
		t.Error("the superseded JAR should have been removed")
	}
	if _, err := os.Stat(filepath.Join(dir, "paper-1.21.8-60.jar")); err != nil {
		t.Errorf("the current JAR must stay: %v", err)
	}

	state, err := LoadState(dir)
	if err != nil || state == nil {
		t.Fatalf("LoadState: %v, %v", state, err)
	}
	if state.ModVersion != "60" || state.File != "paper-1.21.8-60.jar" {
		t.Errorf("state = %+v", state)
	}

	installedAt := state.InstalledAt
	record(dir, "paper-1.21.8-60.jar", "60")

	state, _ = LoadState(dir)
	if !state.InstalledAt.Equal(installedAt) {
		t.Error("an unchanged install must not rewrite the state")
	}
}

func TestRemoveSupersededLeavesInstallerBasedServersAlone(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "forge-1.12.2-14.23.5.2859.jar")

	previous := &State{
		Mod: "forge", File: "forge-1.12.2-14.23.5.2859-installer.jar",
		Launch: providers.LaunchTarget{Jar: "forge-1.12.2-14.23.5.2859.jar"},
	}
	current := &State{
		Mod: "forge", File: "forge-1.12.2-14.23.5.2864-installer.jar",
		Launch: providers.LaunchTarget{Jar: "forge-1.12.2-14.23.5.2864.jar"},
	}

	removeSuperseded(dir, previous, current)

	if _, err := os.Stat(filepath.Join(dir, "forge-1.12.2-14.23.5.2859.jar")); err != nil {
		t.Errorf("the Forge JAR must stay: %v", err)
	}
}

func TestStateMatches(t *testing.T) {
	state := &State{Mod: "paper", Version: "1.21.8", ModVersion: "60"}

	tests := []struct {
		mod, version, modVersion string
		want                     bool
	}{
		{"paper", "", "", true},
		{"paper", "1.21.8", "", true},
		{"paper", "1.21.8", "60", true},
		{"paper", "", "60", true},
		{"paper", "1.21.10", "", false},
		{"paper", "1.21.8", "61", false},
		{"folia", "1.21.8", "60", false},
	}

	for _, tt := range tests {
		if got := state.Matches(tt.mod, tt.version, tt.modVersion); got != tt.want {
			t.Errorf("Matches(%q, %q, %q) = %v, want %v", tt.mod, tt.version, tt.modVersion, got, tt.want)
		}
	}
}

func TestLoadStateMissing(t *testing.T) {
	state, err := LoadState(t.TempDir())
	if state != nil || err != nil {
		t.Errorf("LoadState() = %v, %v; want nil, nil", state, err)
	}
}
