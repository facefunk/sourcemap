package sourcemap

import (
	"bytes"
	"strings"
	"testing"
)

const (
	testFile  = `{"version":3,"file":"min.js","sourceRoot":"/the/root","sources":["one.js","two.js"],"names":["bar","baz","n"],"mappings":"CAAC,IAAI,IAAM,SAAUA,GAClB,OAAOC,IAAID;CCDb,IAAI,IAAM,SAAUE,GAClB,OAAOA"}` + "\n"
	testFileA = `{"version":3,"file":"a/min.js","sourceRoot":"/a/root","sources":["one.js","two.js"],"names":["bar","baz","n"],"mappings":"CAAC,IAAI,IAAM,SAAUA,GAClB,OAAOC,IAAID;CCDb,IAAI,IAAM,SAAUE,GAClB,OAAOA"}` + "\n"
	testFileB = `{"version":3,"file":"b/min.js","sourceRoot":"/b/root","sources":["three.js","four.js"],"names":["foo","foe","m"],"mappings":"CAAC,IAAI,IAAM,SAAUA,GAClB,OAAOC,IAAID;CCDb,IAAI,IAAM,SAAUE,GAClB,OAAOA"}` + "\n"
	testFileC = `{"version":3,"file":"c/min.js","sources":["/a/root/one.js","/a/root/two.js","/b/root/three.js","/b/root/four.js"],"names":["bar","baz","n","foo","foe","m"],"mappings":"CAAC,IAAI,IAAM,SAAUA,GAClB,OAAOC,IAAID;CCDb,IAAI,IAAM,SAAUE,GAClB,OAAOA;CCDT,IAAI,IAAM,SAAUC,GAClB,OAAOC,IAAID;CCDb,IAAI,IAAM,SAAUE,GAClB,OAAOA"}` + "\n"
)

func TestReadFrom(t *testing.T) {
	m, err := ReadFrom(strings.NewReader(testFile))
	if err != nil {
		t.Fatal(err)
	}
	if m.File != "min.js" || m.SourceRoot != "/the/root" || len(m.Sources) != 2 || m.Sources[0] != "one.js" || len(m.Names) != 3 || m.Names[0] != "bar" {
		t.Error(m)
	}
	mappings := m.DecodedMappings()
	if len(mappings) != 13 {
		t.Error(m)
	}
	assertMapping := func(got, expected *Mapping) {
		if got.GeneratedLine != expected.GeneratedLine || got.GeneratedColumn != expected.GeneratedColumn || got.OriginalSourceIndex != expected.OriginalSourceIndex || got.OriginalLine != expected.OriginalLine || got.OriginalColumn != expected.OriginalColumn || got.OriginalNameIndex != expected.OriginalNameIndex {
			t.Errorf("expected %v, got %v", expected, got)
		}
	}
	assertMapping(mappings[0], &Mapping{1, 1, 0, 1, 1, -1, m})
	assertMapping(mappings[1], &Mapping{1, 5, 0, 1, 5, -1, m})
	assertMapping(mappings[2], &Mapping{1, 9, 0, 1, 11, -1, m})
	assertMapping(mappings[3], &Mapping{1, 18, 0, 1, 21, 0, m})
	assertMapping(mappings[4], &Mapping{1, 21, 0, 2, 3, -1, m})
	assertMapping(mappings[5], &Mapping{1, 28, 0, 2, 10, 1, m})
	assertMapping(mappings[6], &Mapping{1, 32, 0, 2, 14, 0, m})
	assertMapping(mappings[7], &Mapping{2, 1, 1, 1, 1, -1, m})
	assertMapping(mappings[8], &Mapping{2, 5, 1, 1, 5, -1, m})
	assertMapping(mappings[9], &Mapping{2, 9, 1, 1, 11, -1, m})
	assertMapping(mappings[10], &Mapping{2, 18, 1, 1, 21, 2, m})
	assertMapping(mappings[11], &Mapping{2, 21, 1, 2, 3, -1, m})
	assertMapping(mappings[12], &Mapping{2, 28, 1, 2, 10, 2, m})
}

func TestWriteTo(t *testing.T) {
	m, err := ReadFrom(strings.NewReader(testFile))
	if err != nil {
		t.Fatal(err)
	}
	m.DecodedMappings()
	m.Swap(3, 4)
	m.Swap(5, 10)
	m.Mappings = ""
	m.Version = 0
	b := bytes.NewBuffer(nil)
	if err := m.WriteTo(b); err != nil {
		t.Fatal(err)
	}
	if b.String() != testFile {
		t.Error(b.String())
	}
}

func TestAppend(t *testing.T) {
	a, err := ReadFrom(strings.NewReader(testFileA))
	if err != nil {
		t.Fatal(err)
	}
	b, err := ReadFrom(strings.NewReader(testFileB))
	if err != nil {
		t.Fatal(err)
	}
	c := New()
	c.Append(a, 0)
	c.Append(b, 2)
	c.File = "c/min.js"

	buf := bytes.NewBuffer(nil)
	if err := c.WriteTo(buf); err != nil {
		t.Fatal(err)
	}
	if buf.String() != testFileC {
		t.Error(buf.String())
	}
}
