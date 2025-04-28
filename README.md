Okay, here is the complete Markdown content for the `README.md` file, incorporating the changes for the conditional DWG build and the accuracy table:

# genfile

`genfile` is a command-line utility written in Go to generate placeholder files of various formats with a specific target size. It's useful for testing scenarios that require files of specific sizes or types without needing actual meaningful content.

## Features

- **Target Sizing:** Generates files that aim to match the requested size. For most formats, this is byte-for-byte exact. See the table below.
- **Structural Validity:** Produced files are structurally valid and can typically be opened by relevant applications (though the content is placeholder/random).
- **Multiple Formats:** Supports a range of common file types.
- **CLI Interface:** Simple command-line interaction using flags.

## Supported Formats & Size Accuracy

The tool aims for exact byte-level accuracy where feasible. However, due to format complexities or library limitations, some formats might only achieve approximate sizing.

| Format Extension(s)   | Generated Content                      | Size Accuracy | Build Type | Dependencies |
| :-------------------- | :------------------------------------- | :------------ | :--------- | :----------- |
| `.txt`, `.log`, `.md` | Random printable ASCII text            | Exact         | Standard   | None         |
| `.png`                | Random noise image + padding chunk     | Exact         | Standard   | None         |
| `.jpg`, `.jpeg`       | Random noise image + padding comments  | Exact         | Standard   | None         |
| `.gif`                | Minimal single-color + padding         | Exact         | Standard   | None         |
| `.mp4`, `.m4v`        | Minimal H.264 structure + frame repeat | Exact         | Standard   | None         |
| `.wav`                | Standard header + random audio data    | Exact         | Standard   | None         |
| `.docx`               | Minimal structure + padded content     | Exact         | Standard   | None         |
| `.xlsx`               | Minimal structure + padded content     | Exact         | Standard   | None         |
| `.pdf`                | Minimal structure + generated content  | Approximate   | Standard   | None         |
| `.csv`                | Random rows/columns                    | Exact         | Standard   | None         |
| `.zip`                | Empty entry + padding entry            | Exact         | Standard   | None         |
| `.html`               | Basic template + padding               | Exact         | Standard   | None         |
| `.json`               | Key-value pairs + padding              | Exact         | Standard   | None         |
| `.xml`                | Basic template + comment padding       | Exact         | Standard   | None         |
| `.dxf`                | Minimal structure + comment padding    | Exact         | Standard   | None         |

## Installation / Building

### Standard Build (No DWG Support)

1.  Ensure you have Go installed (version 1.24.2 or later recommended).
2.  Clone the repository (if you haven't already).
3.  Navigate to the project's root directory.
4.  Run the build command using the Makefile (or standard Go build):

    ```bash
    # Using Makefile
    make build

    # Or using standard Go command
    go build -o genfile ./cmd/cli
    ```

5.  This creates the `genfile` binary.

## Usage

Run the compiled binary, providing the desired output file path and target size using flags:

```bash
# Standard build example
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
# Generate a 500 Kilobyte PNG image (standard build)
./genfile --output picture.png --size 500KB

# Generate a 2 Megabyte WAV audio file (standard build)
./genfile -o song.wav -s 2MB

# Generate a 5 Megabyte DWG file (requires DWG-enabled build)
# Assuming you built it as genfile_dwg
./genfile_dwg --output drawing.dwg --size 5MB

# Generate a 100KB Word document (standard build)
./genfile --output report.docx --size 100KB
```

## Architecture

This project follows the principles of Hexagonal Architecture (Ports and Adapters):

- **Core Application (`internal/application`):** Contains the central use case (creating a file) orchestrated by the `FileService`. It depends only on ports.
- **Ports (`internal/ports`):** Defines interfaces (`FileGenerator`, `GeneratorFactory`, `SizeParser`) that represent the contracts between the core application and the outside world.
- **Adapters (`internal/adapters`):** Implement the ports.
  - _Driving Adapters:_ The CLI (`cmd/cli/main.go`) drives the application based on user input.
  - _Driven Adapters:_ Concrete file generators (`internal/adapters/png`, `internal/adapters/zip`, etc.), the `GeneratorFactory` implementation (`internal/adapters/factory`), and the `SizeParser` implementation (`internal/adapters/utils`) provide the necessary functionalities required by the core application.
