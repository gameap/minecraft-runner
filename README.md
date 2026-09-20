# mcrun - Minecraft Server Runner

A cross-platform CLI tool to download and run Minecraft servers with integrated Java management.

## Features

- **Cross-platform**: Works on Linux, Windows, and macOS
- **Multiple server types**: Vanilla, Paper and its forks (Folia, Purpur, Leaf, Pufferfish), mod loaders (Forge, NeoForge, Fabric, Quilt), hybrids (Mohist, Banner), SpongeVanilla and proxy servers (Velocity, Waterfall, Bungeecord)
- **Automatic Java management**: Detects, installs, and manages Java versions
- **Official API integration**: Downloads from official sources and verifies the published checksums
- **Smart version mapping**: Automatically selects correct Java version for each Minecraft version
- **Panel friendly**: console input is passed straight to the server, stop signals shut it down gracefully, and an installed server still starts when a download API is unreachable
- **Configuration files**: Global and per-server YAML configuration

## Installation

### From Source

```bash
go install github.com/gameap/minecraft-runner@latest
```

### Build from Source

```bash
git clone https://github.com/gameap/minecraft-runner.git
cd minecraft-runner
go build -o mcrun .
```

## Quick Start

```bash
# Run latest vanilla server
mcrun run --accept-eula

# Run Paper 1.20.4
mcrun run --mod=paper --version=1.20.4 --accept-eula

# Run with custom memory
mcrun run --mod=paper --version=1.20.4 --memory=4G --accept-eula
```

## Commands

### run

Download (if needed) and run a Minecraft server.

```bash
mcrun run [flags]

Flags:
      --accept-eula            Automatically accept Minecraft EULA
      --ip string              Server bind IP
      --port int               Server port (default: 25565)
      --query-port int         Query port (enables query)
      --rcon-port int          RCON port
      --rcon-password string   RCON password (enables RCON)
      --memory string          Max heap size (e.g., '4G')
      --min-memory string      Initial heap size (e.g., '1G')
      --jvm-args strings       Additional JVM arguments
```

Examples:
```bash
# Run vanilla server
mcrun run --version=1.20.4 --accept-eula

# Run Paper with RCON enabled
mcrun run --mod=paper --version=1.20.4 --rcon-port=25575 --rcon-password=secret --accept-eula

# Run Forge with custom memory
mcrun run --mod=forge --version=1.20.1 --memory=8G --min-memory=4G --accept-eula

# Run a specific NeoForge version
mcrun run --mod=neoforge --version=1.21.1 --mod-version=21.1.251 --accept-eula

# Run Waterfall proxy
mcrun run --mod=waterfall --version=1.21

# Run Velocity proxy
mcrun run --mod=velocity

# Run Bungeecord proxy
mcrun run --mod=bungeecord
```

#### Installation and updates

`mcrun run` asks the provider which file serves the requested version, installs it if
that exact version is not in the server directory yet, and starts it:

- Without `--mod-version` the newest build is used, so a restart picks up a new Paper
  build or a new recommended Forge. The JAR a newer build replaced is removed.
- Installer-based servers (Forge, NeoForge, Quilt) are installed once per version. Forge
  1.17+ and NeoForge are started from the JVM argument file the installer writes
  (`@libraries/.../unix_args.txt`), older Forge from its JAR. A `user_jvm_args.txt` in
  the server directory is honoured; `--memory` and `--min-memory` win over it.
- What was installed is recorded in `.mcrun-state.json`. When the provider's API cannot
  be reached, the installed server is started with a warning instead of failing - as
  long as it matches the requested mod and version.

#### Stopping and console input

The server reads the console input of `mcrun` directly, so a control panel can write
commands to the stdin of `mcrun`. When stdin is not a terminal, `-Dterminal.jline=false`
is passed to make Paper, Forge and the like read plain lines; pass
`--jvm-args=-Dterminal.jline=true` to override.

On `SIGINT`, `SIGTERM` and `SIGHUP` the server is asked to stop and gets 90 seconds to
save the world before it is killed; `mcrun` then exits with code 0. If the server stops
on its own with an error, `mcrun` exits with the same exit code.

The selected Java is put first on `PATH` and exported as `JAVA_HOME` for the server
process: hybrid servers restart themselves through `java` after installing their libraries.

### list

List available server versions.

```bash
mcrun list [flags]
```

Examples:
```bash
# List vanilla versions
mcrun list

# List Paper versions
mcrun list --mod=paper

# List Forge versions
mcrun list --mod=forge

# List Paper builds for specific MC version
mcrun list --mod=paper --version=1.20.4

# List Waterfall proxy versions
mcrun list --mod=waterfall

# List Velocity proxy versions
mcrun list --mod=velocity
```

### download

Download a server JAR without running it.

```bash
mcrun download [flags]

Flags:
      --force   Force re-download even if file exists
```

Examples:
```bash
# Download vanilla 1.20.4
mcrun download --version=1.20.4

# Download Paper
mcrun download --mod=paper --version=1.20.4

# Force re-download
mcrun download --mod=fabric --version=1.20.4 --force

# Download Waterfall proxy
mcrun download --mod=waterfall --version=1.21

# Download Velocity proxy
mcrun download --mod=velocity

# Download Bungeecord proxy
mcrun download --mod=bungeecord
```

### install java

Install or manage Java versions.

```bash
mcrun install java [flags]

Flags:
      --version int      Java version to install (8, 11, 17, 21, 25)
      --system           Install system-wide (requires root/admin)
      --list             List installed Java versions
      --set-default      Set as system default after install
```

Examples:
```bash
# List installed Java versions
mcrun install java --list

# Install Java 21
mcrun install java --version=21

# Install Java 17 system-wide (Linux, requires sudo)
sudo mcrun install java --version=17 --system

# Install and set as default
mcrun install java --version=21 --set-default
```

## Global Flags

```
-c, --config string        Config file path (default: ~/.mcrun/config.yaml)
-d, --dir string           Server working directory (default: current directory)
-m, --mod string           Server type, see "Supported Server Types" (default: vanilla)
    --mod-version string   Mod-specific version (Paper build, Forge or NeoForge version, Fabric/Quilt loader, ...)
    --version string       Minecraft version (e.g., 1.20.4)
    --java int             Java version override (8, 11, 17, 21, 25)
    --java-path string     Custom Java binary path
-V, --verbose              Enable verbose output
```

## Supported Server Types

### Game Servers

| Type            | Description                                   | Download Source          | `--mod-version`       |
|-----------------|-----------------------------------------------|--------------------------|-----------------------|
| `vanilla`       | Official Minecraft server                     | Mojang API               | -                     |
| `paper`         | Paper (high-performance Spigot fork)          | PaperMC Fill API v3      | build number          |
| `folia`         | Folia (regionised multithreading, by PaperMC) | PaperMC Fill API v3      | build number          |
| `purpur`        | Purpur (Paper fork)                           | Purpur API               | build number          |
| `leaf`          | Leaf (Paper fork)                             | Leaf API                 | build number          |
| `pufferfish`    | Pufferfish (Paper fork)                       | Pufferfish Jenkins       | build number          |
| `fabric`        | Fabric mod loader                             | Fabric Meta API          | loader version        |
| `quilt`         | Quilt mod loader                              | Quilt Meta API + Maven   | loader version        |
| `forge`         | Forge mod loader                              | Forge Maven              | Forge version         |
| `neoforge`      | NeoForge mod loader (1.20.1+)                 | NeoForged Maven          | NeoForge version      |
| `mohist`        | Mohist (Forge + Bukkit hybrid)                | MohistMC API             | build number          |
| `banner`        | Banner (Fabric + Bukkit hybrid)               | MohistMC API             | build number          |
| `spongevanilla` | SpongeVanilla (Sponge API)                    | Sponge Downloads API     | SpongeVanilla version |
| `spigot`        | Alias: installs Paper                         | PaperMC Fill API v3      | build number          |
| `craftbukkit`   | Alias: installs Paper                         | PaperMC Fill API v3      | build number          |
| `cauldron`      | Alias: installs Mohist 1.7.10                 | MohistMC API             | build number          |

Without `--version` the newest release with a stable build is used; a version that only
has pre-release builds yet (for example ALPHA builds of Paper right after a Minecraft
release) can still be requested explicitly.

### Proxy Servers

| Type         | Description                                  | Download Source     |
|--------------|----------------------------------------------|---------------------|
| `waterfall`  | Waterfall proxy (BungeeCord fork by PaperMC) | PaperMC Fill API v3 |
| `velocity`   | Velocity proxy (modern, high-performance)    | PaperMC Fill API v3 |
| `bungeecord` | BungeeCord proxy (original)                  | Jenkins CI          |

**Note:** Proxy servers don't require EULA acceptance or server.properties - they use their own configuration
(`velocity.toml`, `config.yml`). `--port` is passed to Velocity on the command line; Waterfall and Bungeecord
take their listen address from `config.yml` only.

## Java Version Requirements

mcrun automatically selects a recommended Java version: it starts from the requirement
in Mojang's version metadata when available and rounds it up to the nearest available
LTS release (e.g. Minecraft 1.17 reports Java 16, mcrun uses Java 17). When Mojang
metadata is unavailable, this fallback table is used:

| Minecraft Version | Recommended Java |
|-------------------|------------------|
| 26.2+ (year-based versions) | Java 25 |
| 1.21+ | Java 21 |
| 1.20.5 - 1.20.6 | Java 21 |
| 1.18 - 1.20.4 | Java 17 |
| 1.17 - 1.17.1 | Java 17 |
| 1.16.5 and older | Java 8 |

**Proxy servers**: Waterfall and Bungeecord run on **Java 17**, Velocity on **Java 25**.

On Alpine and other musl based systems the Alpine builds of Temurin are installed.

## Configuration

### Global Configuration

Location: `~/.mcrun/config.yaml`

```yaml
defaults:
  mod: paper
  accept_eula: true
  memory: 4G
  min_memory: 1G

java:
  prefer_bundled: false
  auto_install: true
  paths:
    17: /usr/lib/jvm/java-17/bin/java
    21: /usr/lib/jvm/java-21/bin/java

server:
  jvm_args:
    - "-XX:+UseG1GC"
    - "-XX:+ParallelRefProcEnabled"
    - "-XX:MaxGCPauseMillis=200"
  properties:
    enable-query: "true"

cache:
  directory: ~/.mcrun/cache
  max_age_days: 30
```

### Per-Server Configuration

Location: `.mcrun.yaml` in server directory. Its values are the defaults of the
corresponding flags: a flag given on the command line always wins.

```yaml
version: "1.20.4"
mod: paper
mod_version: "499"

java:
  version: 17
  memory: 8G
  min_memory: 2G
  args:
    - "-Dpaper.playerconnection.keepalive=60"

server:
  ip: "0.0.0.0"
  port: 25565
  query_port: 25565
  rcon_port: 25575
  rcon_password: "secret"
```

## Default JVM Arguments

Unless `server.jvm_args` is set in the config, mcrun starts the server with:

```
-XX:+UseG1GC
-XX:+ParallelRefProcEnabled
-XX:MaxGCPauseMillis=200
-XX:+UnlockExperimentalVMOptions
-XX:+DisableExplicitGC
-XX:+AlwaysPreTouch
```

The heap is `-Xmx2G -Xms1G` by default (`--memory`, `--min-memory`); an initial heap
above the maximum is lowered to it. For large heaps consider the full set of
[Aikar's flags](https://docs.papermc.io/paper/aikars-flags) through `server.jvm_args`.

## Development

```bash
go vet ./... && go test ./...

# Resolve the default server of every provider against the real download APIs
go test -tags live ./internal/providers/ -run TestLiveResolve -v
```

## License

MIT License
