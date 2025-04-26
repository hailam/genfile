package ports

// FileType is the identifier for each format.
type FileType string

const (
	FileTypeTXT  FileType = "txt"
	FileTypePNG  FileType = "png"
	FileTypeJPEG FileType = "jpeg"
	FileTypeMP4  FileType = "mp4"
	FileTypeM4V  FileType = "m4v"
	FileTypeWAV  FileType = "wav"
	FileTypeDWG  FileType = "dwg"
	FileTypeDXF  FileType = "dxf"
	FileTypeZIP  FileType = "zip"
	FileTypeXLSX FileType = "xlsx"
	FileTypeDOCX FileType = "docx"
	FileTypePDF  FileType = "pdf"
	FileTypeCSV  FileType = "csv"
	FileTypeJSON FileType = "json"
	FileTypeHTML FileType = "html"
)
