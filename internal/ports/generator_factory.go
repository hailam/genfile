package ports

// GeneratorFactory is the port for looking up generators by FileType.
type GeneratorFactory interface {
	// For returns a FileGenerator for the given FileType, or an error if unsupported.
	For(t FileType) (FileGenerator, error)
}
