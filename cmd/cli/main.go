package main

import (
	"fmt"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"

	"github.com/hailam/genfile/internal/adapters/factory"
	adapterutils "github.com/hailam/genfile/internal/adapters/utils"
	"github.com/hailam/genfile/internal/application"

	// --- Add blank imports for ALL generator packages ---
	// This ensures their init() functions run and register the generators.
	_ "github.com/hailam/genfile/internal/adapters/csv"
	_ "github.com/hailam/genfile/internal/adapters/docx"
	_ "github.com/hailam/genfile/internal/adapters/dxf"
	_ "github.com/hailam/genfile/internal/adapters/html"
	_ "github.com/hailam/genfile/internal/adapters/jpeg"
	_ "github.com/hailam/genfile/internal/adapters/json"
	_ "github.com/hailam/genfile/internal/adapters/mp4"
	_ "github.com/hailam/genfile/internal/adapters/pdf"
	_ "github.com/hailam/genfile/internal/adapters/png"
	_ "github.com/hailam/genfile/internal/adapters/txt"
	_ "github.com/hailam/genfile/internal/adapters/wav"
	_ "github.com/hailam/genfile/internal/adapters/xlsx"
	_ "github.com/hailam/genfile/internal/adapters/zip"
)

// Variables to hold flag values
var outputPath string
var sizeStr string

func main() {
	// --- Composition Root: Initialize Adapters and Core Logic ---
	// This remains the same as before
	generatorFactory := factory.NewGeneratorFactory()
	sizeParser := adapterutils.NewUtilSizeParser()
	fileService := application.NewFileService(generatorFactory, sizeParser)
	// --- End Composition Root ---

	// --- Cobra Command Definition ---
	var rootCmd = &cobra.Command{
		Use:   "genfile",
		Short: "Generates a file of a specific type and size.",
		Long: `genfile is a CLI tool to generate placeholder files of various formats
(e.g., txt, png, jpg, mp4, wav, dwg, zip, xlsx, docx) with a specified size.
The content generated is typically random or minimal structure.`,
		Args: cobra.NoArgs, // We use flags instead of positional arguments now
		Run: func(cmd *cobra.Command, args []string) {
			// Validate flags
			if outputPath == "" {
				fmt.Fprintln(os.Stderr, "Error: output path flag --output is required")
				cmd.Usage()
				os.Exit(1)
			}
			if sizeStr == "" {
				fmt.Fprintln(os.Stderr, "Error: size flag --size is required")
				cmd.Usage()
				os.Exit(1)
			}

			// start spinner
			spinner := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
			spinner.Prefix = fmt.Sprintf("Generating %s (%s)... ", outputPath, sizeStr)
			spinner.Start()

			// --- Execute Core Logic ---
			err := fileService.CreateFile(outputPath, sizeStr)
			spinner.Stop()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error generating file: %v\n", err)
				os.Exit(1)
			}
			// --- End Execute Core Logic ---

			fmt.Printf("Successfully generated %s with size spec '%s'\n", outputPath, sizeStr)
		},
	}

	// Define flags
	rootCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Path to the output file (required)")
	rootCmd.Flags().StringVarP(&sizeStr, "size", "s", "", "Target size (e.g., 500KB, 2MB, 1G) (required)")

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		// Cobra prints errors automatically, but we exit non-zero
		os.Exit(1)
	}
}
