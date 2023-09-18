package sourcemap

import (
	"bytes"
	"encoding/json"
	"io"
	"path"
	"sort"
	"strings"
)

type Map struct {
	Version         int             `json:"version"`
	File            string          `json:"file,omitempty"`
	SourceRoot      string          `json:"sourceRoot,omitempty"`
	Sources         []string        `json:"sources"`
	SourcesContent  []SourceContent `json:"sourcesContent,omitempty"`
	Names           []string        `json:"names"`
	Mappings        string          `json:"mappings"`
	decodedMappings []*Mapping
	fileIndexMap    map[string]int
	nameIndexMap    map[string]int
	fullSources     map[int]string
	resolvedSources map[int]string
}

type Mapping struct {
	GeneratedLine       int
	GeneratedColumn     int
	OriginalSourceIndex int
	OriginalLine        int
	OriginalColumn      int
	OriginalNameIndex   int
	m                   *Map
}

type SourceContent []byte

func (s *SourceContent) UnmarshalJSON(d []byte) error {
	var (
		str string
		err = json.Unmarshal(d, &str)
	)
	if err == nil {
		*s = []byte(str)
	}
	return err
}

func (s *SourceContent) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(*s))
}

func New() *Map {
	return &Map{Version: 3}
}

func (m *Map) IndexForSource(name string) int {
	if i, ok := m.fileIndexMap[name]; ok {
		return i
	}
	return -1
}

func (m *Map) IndexForName(name string) int {
	if i, ok := m.nameIndexMap[name]; ok {
		return i
	}
	return -1
}

func (m *Map) AddSource(name string, content []byte) int {
	if name == "" {
		return -1
	}
	if i, ok := m.fileIndexMap[name]; ok {
		return i
	}
	if m.fileIndexMap == nil {
		m.fileIndexMap = make(map[string]int)
	}
	i := len(m.Sources)
	m.Sources = append(m.Sources, name)
	m.fileIndexMap[name] = i
	if content != nil {
		if len(m.SourcesContent) != len(m.Sources) {
			l := make([]SourceContent, len(m.Sources))
			copy(l, m.SourcesContent)
			m.SourcesContent = l
		}
		m.SourcesContent[i] = content
	}
	return i
}

func (m *Map) AddName(name string) int {
	if name == "" {
		return -1
	}
	if i, ok := m.nameIndexMap[name]; ok {
		return i
	}
	if m.nameIndexMap == nil {
		m.nameIndexMap = make(map[string]int)
	}
	i := len(m.Names)
	m.Names = append(m.Names, name)
	m.nameIndexMap[name] = i
	return i
}

func (m *Mapping) OriginalSource() string {
	if m.OriginalSourceIndex < 0 {
		return ""
	}
	return m.m.Sources[m.OriginalSourceIndex]
}

func (m *Mapping) OriginalFullSource() string {
	if m.OriginalSourceIndex < 0 {
		return ""
	}
	if m.m.SourceRoot == "" {
		return m.OriginalSource()
	}
	if s, ok := m.m.fullSources[m.OriginalSourceIndex]; ok {
		return s
	}
	if m.m.fullSources == nil {
		m.m.fullSources = make(map[int]string)
	}
	s := path.Join(m.m.SourceRoot, m.OriginalSource())
	m.m.fullSources[m.OriginalSourceIndex] = s
	return s
}

func (m *Mapping) OriginalResolvedSource() string {
	if m.OriginalSourceIndex < 0 {
		return ""
	}
	if m.m.File == "" {
		return m.OriginalFullSource()
	}
	if s, ok := m.m.resolvedSources[m.OriginalSourceIndex]; ok {
		return s
	}
	if m.m.resolvedSources == nil {
		m.m.resolvedSources = make(map[int]string)
	}
	s := m.OriginalFullSource()
	if !path.IsAbs(s) {
		s = path.Clean(path.Join(path.Dir(m.m.File), s))
	}
	m.m.resolvedSources[m.OriginalSourceIndex] = s
	return s
}

func (m *Mapping) OriginalSourceContent() []byte {
	if m.OriginalSourceIndex < 0 {
		return nil
	}
	if len(m.m.SourcesContent) > m.OriginalSourceIndex {
		return m.m.SourcesContent[m.OriginalSourceIndex]
	}
	return nil
}

func (m *Mapping) OriginalName() string {
	if m.OriginalNameIndex < 0 {
		return ""
	}
	return m.m.Names[m.OriginalNameIndex]
}

func ReadFrom(r io.Reader) (*Map, error) {
	d := json.NewDecoder(r)
	var m Map
	if err := d.Decode(&m); err != nil {
		return nil, err
	}

	m.fileIndexMap = make(map[string]int)
	m.nameIndexMap = make(map[string]int)

	for i, s := range m.Sources {
		m.fileIndexMap[s] = i
	}
	for i, s := range m.Names {
		m.nameIndexMap[s] = i
	}

	return &m, nil
}

const base64encode = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

var base64decode [256]int

func init() {
	for i := 0; i < len(base64decode); i++ {
		base64decode[i] = 0xff
	}
	for i := 0; i < len(base64encode); i++ {
		base64decode[base64encode[i]] = i
	}
}

func (m *Map) decodeMappings() {
	if m.decodedMappings != nil {
		return
	}

	r := strings.NewReader(m.Mappings)
	var generatedLine = 1
	var generatedColumn = 0
	var originalFile = 0
	var originalLine = 1
	var originalColumn = 0
	var originalName = 0
	for r.Len() != 0 {
		b, _ := r.ReadByte()
		if b == ',' {
			continue
		}
		if b == ';' {
			generatedLine++
			generatedColumn = 0
			continue
		}
		r.UnreadByte()

		count := 0
		readVLQ := func() int {
			v := 0
			s := uint(0)
			for {
				b, _ := r.ReadByte()
				o := base64decode[b]
				if o == 0xff {
					r.UnreadByte()
					return 0
				}
				v += o &^ 32 << s
				if o&32 == 0 {
					break
				}
				s += 5
			}
			count++
			if v&1 != 0 {
				return -(v >> 1)
			}
			return v >> 1
		}
		generatedColumn += readVLQ()
		originalFile += readVLQ()
		originalLine += readVLQ()
		originalColumn += readVLQ()
		originalName += readVLQ()

		switch count {
		case 1:
			m.decodedMappings = append(m.decodedMappings, &Mapping{generatedLine, generatedColumn, -1, 0, 0, -1, m})
		case 4:
			m.decodedMappings = append(m.decodedMappings, &Mapping{generatedLine, generatedColumn, originalFile, originalLine, originalColumn, -1, m})
		case 5:
			m.decodedMappings = append(m.decodedMappings, &Mapping{generatedLine, generatedColumn, originalFile, originalLine, originalColumn, originalName, m})
		}
	}
}

func (m *Map) DecodedMappings() []*Mapping {
	m.decodeMappings()
	return m.decodedMappings
}

func (m *Map) ClearMappings() {
	m.Mappings = ""
	m.decodedMappings = nil
}

func (m *Map) AddMapping(mapping *Mapping) {
	m.decodedMappings = append(m.decodedMappings, mapping)
}

func (m *Map) Len() int {
	m.decodeMappings()
	return len(m.DecodedMappings())
}

func (m *Map) Less(i, j int) bool {
	a := m.decodedMappings[i]
	b := m.decodedMappings[j]
	return a.GeneratedLine < b.GeneratedLine || a.GeneratedLine == b.GeneratedLine && a.GeneratedColumn < b.GeneratedColumn
}

func (m *Map) Swap(i, j int) {
	m.decodedMappings[i], m.decodedMappings[j] = m.decodedMappings[j], m.decodedMappings[i]
}

func (m *Map) EncodeMappings() {
	sort.Sort(m)
	var generatedLine = 1
	var generatedColumn = 0
	var originalFile = 0
	var originalLine = 1
	var originalColumn = 0
	var originalName = 0
	buf := bytes.NewBuffer(nil)
	comma := false
	for _, mapping := range m.decodedMappings {
		for mapping.GeneratedLine > generatedLine {
			buf.WriteByte(';')
			generatedLine++
			generatedColumn = 0
			comma = false
		}
		if comma {
			buf.WriteByte(',')
		}

		writeVLQ := func(v int) {
			v <<= 1
			if v < 0 {
				v = -v
				v |= 1
			}
			for v >= 32 {
				buf.WriteByte(base64encode[32|v&31])
				v >>= 5
			}
			buf.WriteByte(base64encode[v])
		}

		writeVLQ(mapping.GeneratedColumn - generatedColumn)
		generatedColumn = mapping.GeneratedColumn

		if mapping.OriginalSourceIndex >= 0 {
			fileIndex := mapping.OriginalSourceIndex
			writeVLQ(fileIndex - originalFile)
			originalFile = fileIndex

			writeVLQ(mapping.OriginalLine - originalLine)
			originalLine = mapping.OriginalLine

			writeVLQ(mapping.OriginalColumn - originalColumn)
			originalColumn = mapping.OriginalColumn

			if mapping.OriginalNameIndex >= 0 {
				nameIndex := mapping.OriginalNameIndex
				writeVLQ(nameIndex - originalName)
				originalName = nameIndex
			}
		}

		comma = true
	}
	m.Mappings = buf.String()
}

func (m *Map) WriteTo(w io.Writer) error {
	if m.Version == 0 {
		m.Version = 3
	}
	if m.decodedMappings != nil {
		m.EncodeMappings()
	}
	if m.Names == nil {
		m.Names = make([]string, 0)
	}
	if m.Sources == nil {
		m.Sources = make([]string, 0)
	}
	enc := json.NewEncoder(w)
	return enc.Encode(m)
}

func (a *Map) Append(b *Map, line_offset int) {
	a.decodeMappings()
	b.decodeMappings()

	out := make([]*Mapping, len(a.decodedMappings)+len(b.decodedMappings))
	copy(out, a.decodedMappings)
	copy(out[len(a.decodedMappings):], b.decodedMappings)
	appended := out[len(a.decodedMappings):]

	a_last_source_index := -1
	b_last_source_index := -1

	for i, bm := range appended {
		// copy mapping from b
		am := &Mapping{}
		*am = *bm
		am.m = a
		appended[i] = am

		// update indexes
		if b_last_source_index == bm.OriginalSourceIndex {
			am.OriginalSourceIndex = a_last_source_index
		} else {
			b_last_source_index = bm.OriginalSourceIndex
			a_last_source_index = a.AddSource(bm.OriginalResolvedSource(), bm.OriginalSourceContent())
			am.OriginalSourceIndex = a_last_source_index
		}
		am.OriginalNameIndex = a.AddName(bm.OriginalName())
		am.GeneratedLine += line_offset
	}

	a.decodedMappings = out
}
