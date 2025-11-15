# FloraGo

A simple CLI application built with Go, demonstrating proper project structure and cross-platform compilation.

## Features

- Built with [Cobra](https://github.com/spf13/cobra) CLI framework
- Cross-compilation support for macOS, Linux, and Windows (AMD64/ARM64)
- SLURM cluster monitoring utilities
- Embedded remote debugging with Delve
- Version information embedded at build time
- Makefile for easy building and management

## Installation

### Prerequisites

- Go 1.21 or higher
- Make (optional, for using Makefile)

### Building from source

```bash
# Clone the repository (if applicable)
cd florago

# Build for your current platform
make build

# Or use Go directly
go build -o bin/florago .
```

### Cross-compilation

```bash
# Build for macOS Intel (AMD64)
make build-amd64

# Build for macOS Apple Silicon (ARM64)
make build-arm64

# Build for all platforms
make build-all
```

The binaries will be created in the `bin/` directory.

## Usage

```bash
# Run the application
./bin/florago

# Greet someone
./bin/florago [name]

# Check version
./bin/florago version

# Get help
./bin/florago --help
```

## Development

### Available Make targets

```bash
make help          # Show all available targets
make build         # Build for current platform
make build-all     # Build for all platforms
make clean         # Remove build artifacts
make test          # Run tests
make run           # Build and run
make deps          # Download dependencies
make install       # Install to $GOPATH/bin
```

### Project Structure

```
florago/
├── cmd/              # CLI commands
│   └── root.go      # Root command and version
├── bin/             # Build output (gitignored)
├── main.go          # Application entry point
├── go.mod           # Go module file
├── go.sum           # Dependency checksums
├── Makefile         # Build automation
├── README.md        # This file
└── .gitignore       # Git ignore rules
```

## License

MIT
