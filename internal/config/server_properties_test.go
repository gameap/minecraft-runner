package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerPropertiesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.properties")

	original := "#Minecraft server properties\n" +
		"motd=A \\u00A7aGreen\\u00A7r Server\n" +
		"level-type=minecraft\\:normal\n" +
		"server-port=25565\n" +
		"enable-rcon=false\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	props, err := LoadServerProperties(dir)
	if err != nil {
		t.Fatalf("LoadServerProperties: %v", err)
	}

	props.SetPort(25570)
	props.SetRcon(25575, `pa\ss=wo:rd`)

	if err := props.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	saved := string(data)

	for _, want := range []string{
		"#Minecraft server properties\n",
		"motd=A \\u00A7aGreen\\u00A7r Server\n",
		"level-type=minecraft\\:normal\n",
		"server-port=25570\n",
		"enable-rcon=true\n",
		"rcon.port=25575\n",
		`rcon.password=pa\\ss=wo:rd` + "\n",
	} {
		if !strings.Contains(saved, want) {
			t.Errorf("saved file misses %q:\n%s", want, saved)
		}
	}

	if strings.Count(saved, "server-port=") != 1 {
		t.Errorf("server-port must be replaced in place:\n%s", saved)
	}
}

func TestCreateEULA(t *testing.T) {
	dir := t.TempDir()

	if err := CreateEULA(dir); err != nil {
		t.Fatalf("CreateEULA: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "eula.txt"))
	if err != nil || !strings.Contains(string(data), "eula=true") {
		t.Errorf("eula.txt = %q, %v", data, err)
	}
}
