# genfile

`genfile` is a command-line utility written in Go to generate placeholder files of various formats with a precise target size. It's useful for testing scenarios that require files of specific sizes or types without needing actual meaningful content.

## Features

- **Exact Sizing:** Generates files that match the requested size byte-for-byte using format-specific padding techniques.
- **Structural Validity:** Produced files are structurally valid and can typically be opened by relevant applications (though the content is placeholder/random).
- **Multiple Formats:** Supports a range of common file types.
- **CLI Interface:** Simple command-line interaction using flags.

## Supported Formats

- Text: `.txt`, `.log`, `.md`
- Images: `.png`, `.jpg`, `.jpeg`, `.gif`
- Video: `.mp4`, `.m4v`
- Audio: `.wav`
- Documents: `.docx`, `.xlsx`, `.pdf`, `.csv`
- CAD: `.dwg` (generates a basic DXF file)
- Archives: `.zip`
- Web/Data: `.html`, `.json`, `.json`

## Installation / Building

To build the `genfile` binary:

1.  Ensure you have Go installed (version 1.24.2 or later recommended).
2.  Clone the repository (if you haven't already).
3.  Navigate to the project's root directory.
4.  Run the build command using the Makefile:

    ```bash
    make build
    ```

5.  This will create the executable binary at `./genfile` in the current directory.

## Usage

Run the compiled binary from the current directory, providing the desired output file path and target size using flags:

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

# Generate a 50 Megabyte MP4 video file
./genfile --output movie.mp4 --size 50M

# Generate a 1 Gigabyte ZIP archive
./genfile -o archive.zip -s 1G

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
