# genfile

`genfile` is a command-line utility written in Go to generate placeholder files of various formats with a specific target size. It's useful for testing scenarios that require files of specific sizes or types without needing actual meaningful content.

## Features

- **Target Sizing:** Generates files that aim to match the requested size. For most formats, this is byte-for-byte exact. See the table below.
- **Structural Validity:** Produced files are structurally valid and can typically be opened by relevant applications (though the content is placeholder/random).
- **Multiple Formats:** Supports a range of common file types.
- **CLI Interface:** Simple command-line interaction using flags.

## Supported Formats & Size Accuracy

The tool aims for exact byte-level accuracy where feasible. However, due to format complexities or library limitations, some formats might only achieve approximate sizing.

_(Note: MP4/H.264 uses a minimal structure for sizing, not full encoding)._

| Format Extension(s)   | Generated Content                      | Size Accuracy | Validity | Notes                    |
| :-------------------- | :------------------------------------- | :------------ | :------- | :----------------------- |
| `.txt`, `.log`, `.md` | Random printable ASCII text            | Exact         | Full     |                          |
| `.png`                | Random noise image + padding chunk     | Exact         | Full     |                          |
| `.jpg`, `.jpeg`       | Random noise image + padding comments  | Exact         | Full     |                          |
| `.gif`                | Minimal single-color + padding         | Exact         | Full     |                          |
| `.mp4`, `.m4v`        | Minimal H.264 structure + frame repeat | Exact         | Partial  | Minimal structure        |
| `.wav`                | Standard header + random audio data    | Exact         | Full     |                          |
| `.docx`               | Minimal structure + padded content     | Approximate   | Full     | Based on OOXML structure |
| `.xlsx`               | Minimal structure + padded content     | Approximate   | Full     | Based on OOXML structure |
| `.pdf`                | Minimal structure + generated content  | Exact         | Full     |                          |
| `.csv`                | Random rows/columns                    | Exact         | Full     |                          |
| `.zip`                | Empty entry + padding entry            | Exact         | Full     |                          |
| `.html`               | Basic template + padding               | Exact         | Full     |                          |
| `.json`               | Key-value pairs + padding              | Exact         | Full     |                          |
| `.xml`                | Basic template + comment padding       | Exact         | Full     |                          |
| `.dxf`                | Minimal structure + comment padding    | Exact         | Full     |                          |

## Installation / Building

### Prerequisites

- Go version 1.24.2 or later installed.

### Building

1.  Clone the repository:
    ```bash
    git clone <your-repo-url>
    cd <repository-directory>
    ```
2.  Build the binary using the Makefile or Go command:

    ```bash
    # Using Makefile (if available)
    make build

    # Or using standard Go command
    go build -o genfile ./cmd/cli
    ```

3.  This creates the `genfile` binary in the current directory.

## Usage

Run the compiled `genfile` binary, providing the desired output file path and target size using flags.

```bash
./genfile --output <output-path> --size <size>
```

**Flags:**

- `-o`, `--output`: (Required) The path and filename for the generated file (e.g., `my_document.docx`). The file extension determines the type of file generated.
- `-s`, `--size`: (Required) The target size of the file. Supports common units (case-insensitive):
  - Bytes (no suffix or `B`, e.g., `500`, `500B`)
  - Kilobytes (`K` or `KB`, e.g., `10K`, `500KB`)
  - Megabytes (`M` or `MB`, e.g., `4M`, `100MB`)
  - Gigabytes (`G` or `GB`, e.g., `1G`, `2GB`)

**Examples:**

```bash
# Generate a 500 Kilobyte PNG image
./genfile --output picture.png --size 500KB

# Generate a 2 Megabyte WAV audio file
./genfile -o song.wav -s 2MB

# Generate a 100KB Word document
./genfile --output report.docx --size 100KB
```

## Architecture

This project follows the principles of Hexagonal Architecture (Ports and Adapters):

- **Core Application (`internal/application`):** Contains the central use case (creating a file) orchestrated by the `FileService`. It depends only on ports.
- **Ports (`internal/ports`):** Defines interfaces (`FileGenerator`, `GeneratorFactory`, `SizeParser`) that represent the contracts between the core application and the outside world.
- **Adapters (`internal/adapters`):** Implement the ports.
  - _Driving Adapters:_ The CLI (`cmd/cli/main.go`) drives the application based on user input.
  - _Driven Adapters:_ Concrete file generators (`internal/adapters/png`, `internal/adapters/zip`, etc.), the `GeneratorFactory` implementation (`internal/adapters/factory`), and the `SizeParser` implementation (`internal/adapters/utils`) provide the necessary functionalities required by the core application.
