package application

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hailam/genfile/internal/ports" //
)

// --- Mock Implementations ---

// MockSizeParser is a mock for ports.SizeParser
type MockSizeParser struct {
	ParseFunc func(spec string) (int64, error)
}

func (m *MockSizeParser) Parse(spec string) (int64, error) {
	if m.ParseFunc != nil {
		return m.ParseFunc(spec)
	}
	// Default behavior if no function is provided
	switch spec {
	case "10KB":
		return 10 * 1024, nil
	case "1MB":
		return 1024 * 1024, nil
	case "badsize":
		return 0, errors.New("mock parse error")
	default:
		return 0, fmt.Errorf("unexpected size spec in mock: %s", spec)
	}
}

// MockFileGenerator is a mock for ports.FileGenerator
type MockFileGenerator struct {
	GenerateFunc   func(outPath string, sizeBytes int64) error
	GenerateCalled bool
	CalledWithPath string
	CalledWithSize int64
}

func (m *MockFileGenerator) Generate(outPath string, sizeBytes int64) error {
	m.GenerateCalled = true
	m.CalledWithPath = outPath
	m.CalledWithSize = sizeBytes
	if m.GenerateFunc != nil {
		return m.GenerateFunc(outPath, sizeBytes)
	}
	// Default behavior: success
	return nil
}

// MockGeneratorFactory is a mock for ports.GeneratorFactory
type MockGeneratorFactory struct {
	ForFunc       func(t ports.FileType) (ports.FileGenerator, error)
	MockGenerator *MockFileGenerator // Shared mock generator instance
}

func (m *MockGeneratorFactory) For(t ports.FileType) (ports.FileGenerator, error) {
	if m.ForFunc != nil {
		return m.ForFunc(t)
	}
	// Default behavior: return the shared mock generator for known types, error otherwise
	switch t {
	case ports.FileTypeTXT, ports.FileTypePNG:
		if m.MockGenerator == nil {
			panic("MockGeneratorFactory.MockGenerator is nil in default For behavior")
		}
		return m.MockGenerator, nil
	default:
		return nil, fmt.Errorf("mock factory error: unsupported type %s", t)
	}
}

// --- Test Cases ---

func TestFileService_CreateFile(t *testing.T) {
	// Create a temporary directory for output files
	tempDir := t.TempDir()

	// --- Test Scenarios ---
	tests := []struct {
		name           string
		outputPath     string
		sizeSpec       string
		setupParser    func(*MockSizeParser)
		setupFactory   func(*MockGeneratorFactory, *MockFileGenerator)
		expectedErrMsg string                               // Substring of expected error message, empty for success
		validateMock   func(*testing.T, *MockFileGenerator) // Optional validation
	}{
		{
			name:       "Success TXT",
			outputPath: filepath.Join(tempDir, "test.txt"),
			sizeSpec:   "10KB",
			setupParser: func(p *MockSizeParser) {
				// Use default behavior
			},
			setupFactory: func(f *MockGeneratorFactory, mg *MockFileGenerator) {
				// Use default behavior
			},
			expectedErrMsg: "",
			validateMock: func(t *testing.T, mg *MockFileGenerator) {
				if !mg.GenerateCalled {
					t.Errorf("Expected Generate to be called, but it wasn't")
				}
				if mg.CalledWithSize != 10*1024 {
					t.Errorf("Generate called with size %d, want %d", mg.CalledWithSize, 10*1024)
				}
				if mg.CalledWithPath != filepath.Join(tempDir, "test.txt") {
					t.Errorf("Generate called with path %q, want %q", mg.CalledWithPath, filepath.Join(tempDir, "test.txt"))
				}
			},
		},
		{
			name:       "Success PNG uppercase",
			outputPath: filepath.Join(tempDir, "image.PNG"), // Uppercase extension
			sizeSpec:   "1MB",
			setupParser: func(p *MockSizeParser) {
				// Use default behavior
			},
			setupFactory: func(f *MockGeneratorFactory, mg *MockFileGenerator) {
				// Use default behavior
			},
			expectedErrMsg: "",
			validateMock: func(t *testing.T, mg *MockFileGenerator) {
				if !mg.GenerateCalled {
					t.Errorf("Expected Generate to be called, but it wasn't")
				}
				if mg.CalledWithSize != 1024*1024 {
					t.Errorf("Generate called with size %d, want %d", mg.CalledWithSize, 1024*1024)
				}
			},
		},
		{
			name:       "Error Invalid Size Spec",
			outputPath: filepath.Join(tempDir, "test.txt"),
			sizeSpec:   "badsize",
			setupParser: func(p *MockSizeParser) {
				// Use default behavior
			},
			setupFactory: func(f *MockGeneratorFactory, mg *MockFileGenerator) {
				// Factory won't be called
			},
			expectedErrMsg: "invalid size 'badsize': mock parse error",
			validateMock: func(t *testing.T, mg *MockFileGenerator) {
				if mg.GenerateCalled {
					t.Errorf("Expected Generate NOT to be called on size parse error")
				}
			},
		},
		{
			name:       "Error Unsupported Extension",
			outputPath: filepath.Join(tempDir, "test.unknown"),
			sizeSpec:   "10KB",
			setupParser: func(p *MockSizeParser) {
				// Use default behavior
			},
			setupFactory: func(f *MockGeneratorFactory, mg *MockFileGenerator) {
				// Factory won't be called
			},
			expectedErrMsg: "unsupported file extension: unknown",
			validateMock: func(t *testing.T, mg *MockFileGenerator) {
				if mg.GenerateCalled {
					t.Errorf("Expected Generate NOT to be called on unsupported extension")
				}
			},
		},
		{
			name:       "Error No Generator Found",
			outputPath: filepath.Join(tempDir, "test.csv"), // Use a type not handled by default mock factory
			sizeSpec:   "10KB",
			setupParser: func(p *MockSizeParser) {
				// Use default behavior
			},
			setupFactory: func(f *MockGeneratorFactory, mg *MockFileGenerator) {
				// Use default behavior which returns error for CSV
			},
			expectedErrMsg: "no generator for type 'csv': mock factory error: unsupported type csv",
			validateMock: func(t *testing.T, mg *MockFileGenerator) {
				if mg.GenerateCalled {
					t.Errorf("Expected Generate NOT to be called when factory returns error")
				}
			},
		},
		{
			name:       "Error During Generation",
			outputPath: filepath.Join(tempDir, "test.txt"),
			sizeSpec:   "10KB",
			setupParser: func(p *MockSizeParser) {
				// Use default behavior
			},
			setupFactory: func(f *MockGeneratorFactory, mg *MockFileGenerator) {
				// Setup the mock generator to return an error
				mg.GenerateFunc = func(outPath string, sizeBytes int64) error {
					return errors.New("mock generation error")
				}
				// Factory returns this mock generator
				f.ForFunc = func(t ports.FileType) (ports.FileGenerator, error) {
					if t == ports.FileTypeTXT {
						return mg, nil
					}
					return nil, fmt.Errorf("unexpected type in factory setup: %s", t)
				}
			},
			expectedErrMsg: "failed to generate", // Check for substring "failed to generate"
			validateMock: func(t *testing.T, mg *MockFileGenerator) {
				if !mg.GenerateCalled {
					t.Errorf("Expected Generate to be called even if it returns an error")
				}
			},
		},
		{
			name:       "Success No Extension",
			outputPath: filepath.Join(tempDir, "filewithoutextension"),
			sizeSpec:   "10KB",
			// ... setup ...
			expectedErrMsg: "unsupported file extension:", // Expect error because no extension maps to a type
		},
		{
			name:       "Success Path with Dots",
			outputPath: filepath.Join(tempDir, "archive.tar.gz"), // Common pattern, .gz is the relevant ext
			sizeSpec:   "10KB",
			// ... setup ...
			expectedErrMsg: "unsupported file extension: gz", // Expect error as .gz is not directly supported
		},
		{
			name:        "Success JPEG extension mapping",
			outputPath:  filepath.Join(tempDir, "photo.jpeg"),
			sizeSpec:    "1MB",
			setupParser: func(p *MockSizeParser) {}, // Default is fine
			setupFactory: func(f *MockGeneratorFactory, mg *MockFileGenerator) {
				// Need factory to handle JPEG specifically
				f.ForFunc = func(t ports.FileType) (ports.FileGenerator, error) {
					if t == ports.FileTypeJPEG {
						return mg, nil
					}
					return nil, fmt.Errorf("mock factory only supports JPEG for this test")
				}
			},
			expectedErrMsg: "",
			validateMock: func(t *testing.T, mg *MockFileGenerator) {
				if !mg.GenerateCalled {
					t.Errorf("Expected Generate to be called")
				}
				if mg.CalledWithSize != 1024*1024 {
					t.Errorf("Expected size 1MB")
				}
			},
		},
	}

	// --- Run Tests ---
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create fresh mocks for each test
			mockParser := &MockSizeParser{}
			mockGenerator := &MockFileGenerator{}
			mockFactory := &MockGeneratorFactory{MockGenerator: mockGenerator}

			// Apply test-specific setup
			if tc.setupParser != nil {
				tc.setupParser(mockParser)
			}
			if tc.setupFactory != nil {
				tc.setupFactory(mockFactory, mockGenerator)
			}

			// Create FileService with mocks
			service := NewFileService(mockFactory, mockParser) //

			// Execute the method under test
			err := service.CreateFile(tc.outputPath, tc.sizeSpec) //

			// Assertions
			if tc.expectedErrMsg == "" {
				if err != nil {
					t.Errorf("CreateFile() unexpected error = %v", err)
				}
				// Check if file exists (optional, as Generate is mocked)
				// _, statErr := os.Stat(tc.outputPath)
				// if os.IsNotExist(statErr) {
				//  t.Errorf("Expected file %q to exist, but it doesn't", tc.outputPath)
				// }
			} else {
				if err == nil {
					t.Errorf("CreateFile() expected an error containing %q, but got nil", tc.expectedErrMsg)
				} else if !contains(err.Error(), tc.expectedErrMsg) {
					t.Errorf("CreateFile() error = %q, expected error containing %q", err.Error(), tc.expectedErrMsg)
				}
			}

			// Validate mock interactions
			if tc.validateMock != nil {
				tc.validateMock(t, mockGenerator)
			}

			// Clean up created file if it exists (optional, t.TempDir() handles dir)
			if tc.expectedErrMsg == "" && err == nil {
				_ = os.Remove(tc.outputPath)
			}
		})
	}
}

// Helper to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr || // Check suffix first for common errors
		strings.Contains(s, substr) // Fallback to general contains
}
