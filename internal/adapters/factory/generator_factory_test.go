package factory

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/hailam/genfile/internal/ports" //
)

// --- Mock FileGenerator for testing registration ---
type MockGenerator struct {
	id string // To differentiate instances
}

func (m *MockGenerator) Generate(outPath string, sizeBytes int64) error {
	return fmt.Errorf("mock generate called for %s", m.id)
}

// --- Test Helper to Reset Registry ---
// WARNING: This modifies global state and should be used carefully, ideally
// by running tests sequentially or ensuring no parallel tests modify the registry.
var testRegistryMutex sync.Mutex // Use a separate mutex for test manipulation

func resetRegistry() {
	testRegistryMutex.Lock()
	defer testRegistryMutex.Unlock()
	// Create a new map to effectively clear the old one
	generatorRegistry = make(map[ports.FileType]ports.FileGenerator)
}

// --- Test Cases ---

func TestNewGeneratorFactory(t *testing.T) {
	factory := NewGeneratorFactory() //
	if factory == nil {
		t.Fatal("NewGeneratorFactory() returned nil")
	}
	// Check if it implements the interface
	var _ ports.GeneratorFactory = factory
	// Check if it's the expected type (optional, but good practice)
	if _, ok := factory.(*DynamicGeneratorFactory); !ok { //
		t.Errorf("NewGeneratorFactory() returned type %T, want *DynamicGeneratorFactory", factory)
	}
}

func TestDynamicGeneratorFactory_For(t *testing.T) {
	resetRegistry() // Start with a clean registry for this test

	// Setup: Register some mock generators
	mockGenTxt := &MockGenerator{id: "txt-gen"}
	mockGenPng := &MockGenerator{id: "png-gen"}
	RegisterGenerator(ports.FileTypeTXT, mockGenTxt) //
	RegisterGenerator(ports.FileTypePNG, mockGenPng) //

	factory := NewGeneratorFactory()

	tests := []struct {
		name        string
		fileType    ports.FileType
		wantID      string // Expected ID of the returned mock generator
		wantErr     bool
		wantErrText string // Expected error text substring
	}{
		{
			name:     "Get TXT generator",
			fileType: ports.FileTypeTXT,
			wantID:   "txt-gen",
			wantErr:  false,
		},
		{
			name:     "Get PNG generator",
			fileType: ports.FileTypePNG,
			wantID:   "png-gen",
			wantErr:  false,
		},
		{
			name:        "Get unsupported type",
			fileType:    ports.FileTypeCSV, // Not registered in this test setup
			wantErr:     true,
			wantErrText: "unsupported file type: 'csv'",
		},
		{
			name:        "Get empty type",
			fileType:    "", // Empty FileType
			wantErr:     true,
			wantErrText: "unsupported file type: ''",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotGenerator, err := factory.For(tc.fileType) //

			if (err != nil) != tc.wantErr {
				t.Errorf("For(%q) error = %v, wantErr %v", tc.fileType, err, tc.wantErr)
				return
			}

			if tc.wantErr {
				if err == nil {
					t.Errorf("For(%q) expected an error containing %q, but got nil", tc.fileType, tc.wantErrText)
				} else if !strings.Contains(err.Error(), tc.wantErrText) { // Use strings.Contains for substring check
					t.Errorf("For(%q) error = %q, expected error containing %q", tc.fileType, err.Error(), tc.wantErrText)
				}
			} else {
				// Check if the returned generator is the correct mock instance
				if gotMock, ok := gotGenerator.(*MockGenerator); ok {
					if gotMock.id != tc.wantID {
						t.Errorf("For(%q) returned generator with ID %q, want %q", tc.fileType, gotMock.id, tc.wantID)
					}
				} else {
					t.Errorf("For(%q) returned type %T, want *MockGenerator", tc.fileType, gotGenerator)
				}
			}
		})
	}
}

func TestRegisterGenerator_Overwrite(t *testing.T) {
	resetRegistry() // Clean slate

	mockGen1 := &MockGenerator{id: "gen1"}
	mockGen2 := &MockGenerator{id: "gen2"}

	// Register initial generator
	RegisterGenerator(ports.FileTypeTXT, mockGen1) //

	// Verify initial registration
	factory := NewGeneratorFactory()
	gen, err := factory.For(ports.FileTypeTXT)
	if err != nil {
		t.Fatalf("For(TXT) failed after initial registration: %v", err)
	}
	if mockGen, ok := gen.(*MockGenerator); !ok || mockGen.id != "gen1" {
		t.Fatalf("For(TXT) returned wrong generator after initial registration. Got ID: %v", gen)
	}

	// Register second generator for the same type (overwrite)
	// Note: Check test output/logs for the "Warning: Duplicate generator registration" message.
	RegisterGenerator(ports.FileTypeTXT, mockGen2) //

	// Verify overwrite
	gen, err = factory.For(ports.FileTypeTXT)
	if err != nil {
		t.Fatalf("For(TXT) failed after overwriting registration: %v", err)
	}
	if mockGen, ok := gen.(*MockGenerator); !ok || mockGen.id != "gen2" {
		t.Errorf("For(TXT) did not return the overwritten generator. Got ID: %v, want 'gen2'", gen)
	}
}

func TestRegisteredTypes(t *testing.T) {
	resetRegistry() // Clean slate

	// Register some types
	RegisterGenerator(ports.FileTypeZIP, &MockGenerator{id: "zip"}) //
	RegisterGenerator(ports.FileTypeTXT, &MockGenerator{id: "txt"}) //
	RegisterGenerator(ports.FileTypePNG, &MockGenerator{id: "png"}) //

	expectedTypes := []ports.FileType{ports.FileTypeZIP, ports.FileTypeTXT, ports.FileTypePNG}
	gotTypes := RegisteredTypes() //

	// Sort both slices for reliable comparison
	sort.Slice(expectedTypes, func(i, j int) bool { return expectedTypes[i] < expectedTypes[j] })
	sort.Slice(gotTypes, func(i, j int) bool { return gotTypes[i] < gotTypes[j] })

	if !reflect.DeepEqual(gotTypes, expectedTypes) {
		t.Errorf("RegisteredTypes() = %v, want %v", gotTypes, expectedTypes)
	}

	// Test after adding another
	RegisterGenerator(ports.FileTypeCSV, &MockGenerator{id: "csv"}) //
	expectedTypes = append(expectedTypes, ports.FileTypeCSV)
	sort.Slice(expectedTypes, func(i, j int) bool { return expectedTypes[i] < expectedTypes[j] })

	gotTypes = RegisteredTypes()
	sort.Slice(gotTypes, func(i, j int) bool { return gotTypes[i] < gotTypes[j] })

	if !reflect.DeepEqual(gotTypes, expectedTypes) {
		t.Errorf("RegisteredTypes() after adding CSV = %v, want %v", gotTypes, expectedTypes)
	}

	// Test with empty registry
	resetRegistry()
	gotTypes = RegisteredTypes()
	if len(gotTypes) != 0 {
		t.Errorf("RegisteredTypes() on empty registry = %v, want empty slice", gotTypes)
	}
}
