package docx

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/hailam/genfile/internal/ports"
	"github.com/hailam/genfile/internal/utils"
)

type DocxGenerator struct{}

func New() ports.FileGenerator {
	return &DocxGenerator{}
}

// Generate creates a DOCX file at the given path with the specified size.
func (g *DocxGenerator) Generate(path string, targetSize int64) error {
	padOH := utils.ZipEntryOverhead()

	// minimal DOCX (1 para)
	buf := &bytes.Buffer{}
	zipWriterMinimal(buf, 1)
	minimal := int64(buf.Len())
	if minimal+padOH > targetSize {
		return fmt.Errorf("target %d too small (min %d + padOH %d)", targetSize, minimal, padOH)
	}

	// avg per para (5 paras)
	buf2 := &bytes.Buffer{}
	zipWriterMinimal(buf2, 5)
	avgPara := (int64(buf2.Len()) - minimal) / 5
	if avgPara < 1 {
		avgPara = 1
	}

	// initial guess
	maxUsable := targetSize - padOH
	estCount := (maxUsable - minimal) / avgPara
	if estCount < 1 {
		estCount = 1
	}

	var finalCount int
	for cnt := estCount; cnt >= 1; cnt-- {
		// write cnt paras
		outF, _ := os.Create(path)
		zw := zip.NewWriter(outF)
		writeContentTypes(zw)
		writeRels(zw)
		writeDocRels(zw)
		writeDocumentXML(zw, int(cnt))
		zw.Close()
		outF.Close()

		info, _ := os.Stat(path)
		if info.Size()+padOH <= targetSize {
			finalCount = int(cnt)
			break
		}
	}
	if finalCount == 0 {
		return errors.New("could not fit even one paragraph")
	}

	// rewrite finalCount
	outF, _ := os.Create(path)
	zw := zip.NewWriter(outF)
	writeContentTypes(zw)
	writeRels(zw)
	writeDocRels(zw)
	writeDocumentXML(zw, finalCount)
	zw.Close()
	outF.Close()

	return utils.PadZipExtend(path, targetSize)
}

// zipWriterMinimal builds a minimal DOCX zip with 'n' paragraphs into w.
func zipWriterMinimal(w io.Writer, n int) {
	zw := zip.NewWriter(w)
	writeContentTypes(zw)
	writeRels(zw)
	writeDocRels(zw)
	writeDocumentXML(zw, n)
	zw.Close()
}

// Helpers to write the four minimal parts:

func writeContentTypes(zw *zip.Writer) {
	mustCreate(zw, "[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`)
}

func writeRels(zw *zip.Writer) {
	mustCreate(zw, "_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1"
    Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"
    Target="word/document.xml"/>
</Relationships>`)
}

func writeDocRels(zw *zip.Writer) {
	mustCreate(zw, "word/_rels/document.xml.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`)
}

// writeDocumentXML writes a word/document.xml with n paragraphs of random text.
func writeDocumentXML(zw *zip.Writer, n int) {
	buf := &bytes.Buffer{}
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
`)
	for i := 0; i < n; i++ {
		buf.WriteString("    <w:p><w:r><w:t>")
		buf.WriteString(utils.RandString(50))
		buf.WriteString("</w:t></w:r></w:p>\n")
	}
	buf.WriteString("    <w:sectPr/>\n  </w:body>\n</w:document>")
	mustCreate(zw, "word/document.xml", buf.String())
}

// mustCreate is as before
func mustCreate(zw *zip.Writer, name, content string) {
	w, _ := zw.Create(name)
	w.Write([]byte(content))
}
