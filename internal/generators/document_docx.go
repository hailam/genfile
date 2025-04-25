package generators

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"os"
)

// generateDOCX writes a minimal DOCX with repeated paragraphs, then pads.
func GenerateDOCX(path string, targetSize int64) error {
	// 1) Overhead with 1 paragraph
	buf1 := &bytes.Buffer{}
	zipWriterMinimal(buf1, 1)
	overhead := int64(buf1.Len())
	if overhead >= targetSize {
		return fmt.Errorf("target %d < minimal DOCX %d", targetSize, overhead)
	}

	// 2) Avg per paragraph with 5 test paras
	buf2 := &bytes.Buffer{}
	zipWriterMinimal(buf2, 5)
	avgPara := (int64(buf2.Len()) - overhead) / 5
	if avgPara < 1 {
		avgPara = 1
	}

	// 3) Pad entry overhead
	padOH := zipEntryOverhead()

	// 4) How many paras we can fill
	usable := targetSize - overhead - padOH
	paraCount := usable / avgPara
	if paraCount < 1 {
		paraCount = 1
	}

	// 5) Write real DOCX
	outF, err := os.Create(path)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(outF)
	writeContentTypes(zw)
	writeRels(zw)
	writeDocRels(zw)
	writeDocumentXML(zw, int(paraCount))
	zw.Close()
	outF.Close()

	// 6) Pad
	return padZipExtend(path, targetSize)
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
		buf.WriteString(randString(50))
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

// ----------------------- Padding -----------------------

// padZipExtend appends a 'pad.bin' entry to reach exactly targetSize bytes.
func padZipExtend(inPath string, targetSize int64) error {
	info, err := os.Stat(inPath)
	if err != nil {
		return err
	}
	orig := info.Size()
	if orig > targetSize {
		return fmt.Errorf("file is %d > target %d", orig, targetSize)
	}
	// compute overhead of empty pad.bin entry
	padOH := zipEntryOverhead()
	needed := targetSize - orig - padOH

	// open original
	zr, err := zip.OpenReader(inPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	tmp := inPath + ".tmp"
	outF, _ := os.Create(tmp)
	zw := zip.NewWriter(outF)

	// copy entries
	for _, f := range zr.File {
		hdr := f.FileHeader
		w, _ := zw.CreateHeader(&hdr)
		r, _ := f.Open()
		io.Copy(w, r)
		r.Close()
	}
	// create pad.bin uncompressed
	padHdr := &zip.FileHeader{Name: "pad.bin", Method: zip.Store}
	w, _ := zw.CreateHeader(padHdr)
	zero := make([]byte, 64*1024)
	for needed > 0 {
		chunk := int64(len(zero))
		if chunk > needed {
			chunk = needed
		}
		w.Write(zero[:chunk])
		needed -= chunk
	}
	zw.Close()
	outF.Close()
	os.Rename(tmp, inPath)
	return nil
}

// zipEntryOverhead returns the byte-length of an empty 'pad.bin' entry in a new ZIP.
func zipEntryOverhead() int64 {
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	hdr := &zip.FileHeader{Name: "pad.bin", Method: zip.Store}
	zw.CreateHeader(hdr)
	zw.Close()
	return int64(buf.Len())
}

// randString returns a random A–Z string of length n.
func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('A' + rand.Intn(26))
	}
	return string(b)
}
