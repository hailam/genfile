package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hailam/genfile/internal/generators"
	"github.com/hailam/genfile/internal/utils"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: genfile <output-path> <size>")
		os.Exit(1)
	}
	outputPath := os.Args[1]
	sizeStr := os.Args[2]
	sizeBytes, err := utils.ParseSize(sizeStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid size: %v\n", err)
		os.Exit(1)
	}
	if sizeBytes <= 0 {
		fmt.Fprintln(os.Stderr, "Size must be greater than 0")
		os.Exit(1)
	}
	ext := strings.ToLower(strings.TrimPrefix(strings.ToLower(
		filepath.Ext(outputPath)), "."))
	// Call the appropriate generator based on extension
	switch ext {
	case "txt":
		err = generators.GenerateTXT(outputPath, sizeBytes)
	case "png":
		err = generators.GeneratePNG(outputPath, sizeBytes)
	case "jpg", "jpeg":
		err = generators.GenerateJPEG(outputPath, sizeBytes)
	case "mp4", "m4v":
		err = generators.GenerateMP4(outputPath, sizeBytes)
	case "wav":
		err = generators.GenerateWAV(outputPath, sizeBytes)
	case "dwg":
		err = generators.GenerateDWG(outputPath, sizeBytes)
	case "zip":
		err = generators.GenerateZIP(outputPath, sizeBytes)
	case "xlsx":
		err = generators.GenerateXLSX(outputPath, sizeBytes)
	case "docx":
		err = generators.GenerateDOCX(outputPath, sizeBytes)
	default:
		fmt.Fprintf(os.Stderr, "Unsupported file extension: %s\n", ext)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Generated %s (%d bytes)\n", outputPath, sizeBytes)
}
