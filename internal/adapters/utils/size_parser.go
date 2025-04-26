package utils

import (
	"github.com/hailam/genfile/internal/ports"
	"github.com/hailam/genfile/internal/utils"
)

// UtilSizeParser adapts the utils.ParseSize function to the ports.SizeParser interface.
type UtilSizeParser struct{}

// NewUtilSizeParser creates a new size parser adapter.
func NewUtilSizeParser() ports.SizeParser {
	return &UtilSizeParser{}
}

// Parse uses the existing utility function to parse the size string.
func (p *UtilSizeParser) Parse(spec string) (int64, error) {
	return utils.ParseSize(spec) //
}
