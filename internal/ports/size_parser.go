package ports

// SizeParser parses human-readable size specs (like "10MB") into bytes.
type SizeParser interface {
	Parse(spec string) (int64, error)
}
