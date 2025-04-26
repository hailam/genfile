package ports

// FileGenerator is the port for anything that can produce a file.
type FileGenerator interface {
	// Generate writes a file at outPath exactly sizeBytes long.
	Generate(outPath string, sizeBytes int64) error
}
