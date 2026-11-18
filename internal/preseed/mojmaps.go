package preseed

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"maps"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

// SanitizeMappings converts Mojang's ProGuard mappings into the TSRG file
// that DOWNLOAD_MOJMAPS --sanitize of Forge's installertools writes, and
// returns it when its SHA-1 is wantSHA1. It follows SrgUtils (IMappingFile.load,
// then write as TSRG) byte for byte; SrgUtils releases order members
// differently, so every known ordering is tried.
func SanitizeMappings(r io.Reader, wantSHA1 string) ([]byte, bool, error) {
	m, err := parseMappings(r)
	if err != nil {
		return nil, false, err
	}

	for _, order := range []memberOrder{srgUtilsLegacyOrder, srgUtilsOrder} {
		out := m.tsrg(order)
		if sum := sha1.Sum(out); strings.EqualFold(hex.EncodeToString(sum[:]), wantSHA1) {
			return out, true, nil
		}
	}

	return nil, false, nil
}

func parseMappings(r io.Reader) (*pgMappings, error) {
	m := &pgMappings{classes: make(map[string]*pgClass)}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64<<10), 16<<20)
	scanner.Split(scanJavaLines)

	var cls *pgClass
	for scanner.Scan() {
		line := stripMappingComment(scanner.Text())
		if line == "" {
			continue
		}

		var err error
		if cls, err = m.parse(cls, strings.ReplaceAll(line, ".", "/")); err != nil {
			return nil, err
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read the mappings: %w", err)
	}

	return m, nil
}

type pgClass struct {
	original, mapped string
	fields           map[string]pgMember
	methods          map[string]pgMember
}

// pgMember is a field or method; seq is the order of its first appearance,
// which a later duplicate keeps, as in a Java HashMap
type pgMember struct {
	original, desc, mapped string
	seq                    int
}

type pgMappings struct {
	classes map[string]*pgClass
}

func (m *pgMappings) parse(cls *pgClass, line string) (*pgClass, error) {
	switch {
	case !strings.HasPrefix(line, "    ") && strings.HasSuffix(line, ":"):
		parts := strings.Split(line, " -> ")
		if len(parts) < 2 {
			return nil, fmt.Errorf("malformed mappings line %q", line)
		}
		cls = &pgClass{
			original: parts[0],
			mapped:   strings.TrimSuffix(parts[1], ":"),
			fields:   make(map[string]pgMember),
			methods:  make(map[string]pgMember),
		}
		m.classes[cls.original] = cls
		return cls, nil

	case cls == nil:
		return nil, fmt.Errorf("mappings line %q precedes any class", line)

	case strings.Contains(line, "(") && strings.Contains(line, ")"):
		method, err := parseMethod(strings.TrimSpace(line))
		if err != nil {
			return nil, err
		}
		addMember(cls.methods, method.original+method.desc, method)
		return cls, nil

	default:
		parts := strings.Split(strings.TrimSpace(line), " ")
		if len(parts) < 4 {
			return nil, fmt.Errorf("malformed mappings line %q", line)
		}
		addMember(cls.fields, parts[1], pgMember{original: parts[1], desc: typeDescriptor(parts[0]), mapped: parts[3]})
		return cls, nil
	}
}

func addMember(members map[string]pgMember, key string, member pgMember) {
	member.seq = len(members)
	if old, ok := members[key]; ok {
		member.seq = old.seq
	}
	members[key] = member
}

// parseMethod reads "[start:end:]ret name(args)[:a:b] -> obf"
func parseMethod(line string) (pgMember, error) {
	if i := strings.IndexByte(line, ':'); i != -1 {
		j := strings.IndexByte(line[i+1:], ':')
		if j == -1 {
			return pgMember{}, fmt.Errorf("malformed mappings line %q", line)
		}
		for _, n := range []string{line[:i], line[i+1 : i+1+j]} {
			if _, err := strconv.Atoi(n); err != nil {
				return pgMember{}, fmt.Errorf("malformed mappings line %q", line)
			}
		}
		line = line[i+1+j+1:]
	}

	_, obf, ok := strings.Cut(line, " -> ")
	space := strings.IndexByte(line, ' ')
	open, closing := strings.IndexByte(line, '('), strings.IndexByte(line, ')')
	if !ok || space == -1 || open < space || closing < open {
		return pgMember{}, fmt.Errorf("malformed mappings line %q", line)
	}
	obf, _, _ = strings.Cut(obf, " -> ")

	var desc strings.Builder
	desc.WriteByte('(')
	for arg := range strings.SplitSeq(line[open+1:closing], ",") {
		if arg == "" {
			break
		}
		desc.WriteString(typeDescriptor(arg))
	}
	desc.WriteByte(')')
	desc.WriteString(typeDescriptor(line[:space]))

	return pgMember{original: line[space+1 : open], desc: desc.String(), mapped: obf}, nil
}

func typeDescriptor(t string) string {
	if elem, ok := strings.CutSuffix(t, "[]"); ok {
		return "[" + typeDescriptor(elem)
	}

	switch t {
	case "int":
		return "I"
	case "void":
		return "V"
	case "boolean":
		return "Z"
	case "byte":
		return "B"
	case "char":
		return "C"
	case "short":
		return "S"
	case "double":
		return "D"
	case "float":
		return "F"
	case "long":
		return "J"
	}

	return "L" + t + ";"
}

var classInDescriptor = regexp.MustCompile(`L([^;]+);`)

func (m *pgMappings) remapClass(name string) string {
	if cls, ok := m.classes[name]; ok {
		return cls.mapped
	}
	if i := strings.LastIndexByte(name, '$'); i != -1 {
		return m.remapClass(name[:i]) + "$" + name[i+1:]
	}
	return name
}

func (m *pgMappings) remapDescriptor(desc string) string {
	return classInDescriptor.ReplaceAllStringFunc(desc, func(match string) string {
		return "L" + m.remapClass(match[1:len(match)-1]) + ";"
	})
}

// memberOrder sorts the fields and the methods of a class in place
type memberOrder func(m *pgMappings, fields, methods []pgMember)

// srgUtilsLegacyOrder is SrgUtils before 2026: members sorted by name alone,
// overloads left in the iteration order of the HashMap that holds them
func srgUtilsLegacyOrder(_ *pgMappings, fields, methods []pgMember) {
	sort.SliceStable(fields, func(i, j int) bool { return fields[i].original < fields[j].original })

	sort.Slice(methods, func(i, j int) bool { return methods[i].seq < methods[j].seq })
	buckets := javaHashMapCapacity(len(methods)) - 1
	bucket := func(method pgMember) int32 { return javaHashSpread(method.original+method.desc) & buckets }
	sort.SliceStable(methods, func(i, j int) bool { return bucket(methods[i]) < bucket(methods[j]) })

	sort.SliceStable(methods, func(i, j int) bool { return methods[i].original < methods[j].original })
}

// srgUtilsOrder is SrgUtils since 2026: fields by obfuscated name and type,
// methods by name and descriptor
func srgUtilsOrder(m *pgMappings, fields, methods []pgMember) {
	sort.Slice(fields, func(i, j int) bool {
		if fields[i].mapped != fields[j].mapped {
			return fields[i].mapped < fields[j].mapped
		}
		return m.remapDescriptor(fields[i].desc) < m.remapDescriptor(fields[j].desc)
	})

	sort.Slice(methods, func(i, j int) bool {
		if methods[i].original != methods[j].original {
			return methods[i].original < methods[j].original
		}
		return methods[i].desc < methods[j].desc
	})
}

func (m *pgMappings) tsrg(order memberOrder) []byte {
	classes := make([]*pgClass, 0, len(m.classes))
	for _, cls := range m.classes {
		classes = append(classes, cls)
	}
	sort.Slice(classes, func(i, j int) bool { return classes[i].original < classes[j].original })

	var out bytes.Buffer
	for _, cls := range classes {
		fields := slices.Collect(maps.Values(cls.fields))
		methods := slices.Collect(maps.Values(cls.methods))
		order(m, fields, methods)

		out.WriteString(cls.original + " " + cls.mapped + "\n")
		for _, field := range fields {
			out.WriteString("\t" + field.original + " " + field.mapped + "\n")
		}
		for _, method := range methods {
			out.WriteString("\t" + method.original + " " + method.desc + " " + method.mapped + "\n")
		}
	}

	return out.Bytes()
}

// javaHashMapCapacity is the table size of a java.util.HashMap that got n
// entries one by one
func javaHashMapCapacity(n int) int32 {
	capacity := int32(16)
	for n > int(capacity)*3/4 {
		capacity *= 2
	}
	return capacity
}

// javaHashSpread is String.hashCode as HashMap spreads it over the buckets
func javaHashSpread(s string) int32 {
	var h int32
	for _, unit := range utf16.Encode([]rune(s)) {
		h = 31*h + int32(unit)
	}
	return h ^ int32(uint32(h)>>16)
}

// stripMappingComment mirrors SrgUtils' InternalUtils.stripComment, quirks
// included: the character before '#' is dropped and one leading space stays
func stripMappingComment(line string) string {
	i := strings.IndexByte(line, '#')
	if i == 0 {
		return ""
	}
	if i != -1 {
		line = line[:i-1]
	}

	end := len(line)
	for end > 1 && line[end-1] == ' ' {
		end--
	}
	return line[:end]
}

// scanJavaLines splits lines the way BufferedReader.readLine does: on \n,
// \r or \r\n
func scanJavaLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	if i := bytes.IndexAny(data, "\r\n"); i != -1 {
		if data[i] == '\n' {
			return i + 1, data[:i], nil
		}
		if i+1 < len(data) {
			if data[i+1] == '\n' {
				return i + 2, data[:i], nil
			}
			return i + 1, data[:i], nil
		}
		if atEOF {
			return i + 1, data[:i], nil
		}
		return 0, nil, nil
	}

	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}
