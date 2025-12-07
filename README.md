# mcrun - Minecraft Server Runner

A cross-platform CLI tool to download and run Minecraft servers with integrated Java management.

## Features

- **Cross-platform**: Works on Linux, Windows, and macOS
- **Multiple server types**: Vanilla, Paper, Fabric, Forge, proxy servers (Waterfall, Velocity, Bungeecord), and more
- **Automatic Java management**: Detects, installs, and manages Java versions
- **Official API integration**: Downloads from official sources (Mojang, PaperMC, Fabric, Forge)
- **Smart version mapping**: Automatically selects correct Java version for each Minecraft version
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
mcrun run --mod=forge --version=1.20.4 --memory=8G --min-memory=4G --accept-eula

# Run Waterfall proxy
mcrun run --mod=waterfall --version=1.21

# Run Velocity proxy
mcrun run --mod=velocity

# Run Bungeecord proxy
mcrun run --mod=bungeecord
```

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
      --version int      Java version to install (8, 11, 17, 21)
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
-m, --mod string           Server type: vanilla, paper, forge, fabric, waterfall, velocity, bungeecord, spigot, craftbukkit, cauldron
    --mod-version string   Mod-specific version (e.g., Paper build number)
    --version string       Minecraft version (e.g., 1.20.4)
    --java int             Java version override (8, 11, 17, 21)
    --java-path string     Custom Java binary path
-V, --verbose              Enable verbose output
```

## Supported Server Types

### Game Servers

| Type | Description | Download Source |
|------|-------------|-----------------|
| `vanilla` | Official Minecraft server | Mojang API |
| `paper` | Paper (high-performance Spigot fork) | PaperMC API |
| `fabric` | Fabric mod loader | Fabric Meta API |
| `forge` | Forge mod loader | Forge Maven |
| `spigot` | Spigot (suggests Paper) | - |
| `craftbukkit` | CraftBukkit (suggests Paper) | - |
| `cauldron` | Cauldron (legacy, MC 1.7.10 max) | Archived |

### Proxy Servers

| Type | Description | Download Source |
|------|-------------|-----------------|
| `waterfall` | Waterfall proxy (BungeeCord fork by PaperMC) | PaperMC API |
| `velocity` | Velocity proxy (modern, high-performance) | PaperMC API |
| `bungeecord` | BungeeCord proxy (original) | Jenkins CI |

**Note:** Proxy servers don't require EULA acceptance or server.properties - they use their own `config.yml` configuration.

## Java Version Requirements

mcrun automatically selects the correct Java version based on Minecraft version:

| Minecraft Version | Required Java |
|-------------------|---------------|
| 1.21+ | Java 21 |
| 1.20.5 - 1.20.6 | Java 21 |
| 1.18 - 1.20.4 | Java 17 |
| 1.17 - 1.17.1 | Java 16+ |
| 1.16.5 and older | Java 8 |

**Proxy servers** (Waterfall, Velocity, Bungeecord) require **Java 17**.

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

Location: `.mcrun.yaml` in server directory

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

mcrun uses optimized JVM flags by default (Aikar's flags):

```
-XX:+UseG1GC
-XX:+ParallelRefProcEnabled
-XX:MaxGCPauseMillis=200
-XX:+UnlockExperimentalVMOptions
-XX:+DisableExplicitGC
-XX:+AlwaysPreTouch
-XX:G1NewSizePercent=30
-XX:G1MaxNewSizePercent=40
-XX:G1HeapRegionSize=8M
-XX:G1ReservePercent=20
-XX:G1HeapWastePercent=5
-XX:G1MixedGCCountTarget=4
-XX:InitiatingHeapOccupancyPercent=15
-XX:G1MixedGCLiveThresholdPercent=90
-XX:G1RSetUpdatingPauseTimePercent=5
-XX:SurvivorRatio=32
-XX:+PerfDisableSharedMem
-XX:MaxTenuringThreshold=1
```

## License

MIT License
