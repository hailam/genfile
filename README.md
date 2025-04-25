```
genfile picture.png 500KB
genfile song.wav 2MB
genfile movie.mp4 50MB
genfile archive.zip 1MB
```

Structural validity: Each file can be opened in its relevant application without errors (image viewers for PNG/JPEG, media players for MP4/WAV, CAD software for DWG/DXF, archive tools for ZIP, and Office for XLSX/DOCX).

Content is random or placeholder (noise in images/audio, gibberish text, etc.), so it carries no meaning.

Exact sizing achieved through careful padding (using format-specific tricks like ancillary chunks, comment segments, or archive comments).
