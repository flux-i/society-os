package documents

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/xml"
	"io"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Statement originals are inspected and retained, never evaluated or imported.
// The initial workbook profile accepts plain worksheet packages only.
const MaxStatementBytes = 10 * 1024 * 1024
const statementExpandedBytes = 30 * 1024 * 1024
const statementPartBytes = 8 * 1024 * 1024
const statementCellLimit = 100000
const statementRows = 50000
const statementColumns = 512

var statementSheetPart = regexp.MustCompile(`^xl/worksheets/sheet[0-9]+\.xml$`)
var statementThemePart = regexp.MustCompile(`^xl/theme/theme[0-9]+\.xml$`)
var statementCellRef = regexp.MustCompile(`^([A-Z]{1,3})([1-9][0-9]{0,5})$`)
var statementNumber = regexp.MustCompile(`^[+-]?[0-9]+(?:\.[0-9]+)?$`)
var statementFunction = regexp.MustCompile(`([A-Za-z_][A-Za-z_0-9.]*)\s*\(`)
var statementQuotedText = regexp.MustCompile(`"(?:[^"]|"")*"`)

func ValidateStatementOriginal(ctx context.Context, filename string, data []byte) (string, string) {
	if ctx.Err() != nil {
		return "", "CHECK_CANCELLED"
	}
	if len(data) == 0 || len(data) > MaxStatementBytes {
		return "", "FILE_TOO_LARGE"
	}
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".pdf":
		return ValidateOriginal(ctx, filename, data)
	case ".csv":
		if len(data) > 4*1024*1024 {
			return "", "FILE_TOO_LARGE"
		}
		if code := validateStatementCSV(ctx, data); code != "" {
			return "", code
		}
		return "text/csv; charset=utf-8", ""
	case ".xlsx":
		if code := validateStatementWorkbook(ctx, data); code != "" {
			return "", code
		}
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ""
	default:
		return "", "UNSUPPORTED_TYPE"
	}
}

func activeCSVField(value string) bool {
	trimmed := strings.TrimLeftFunc(value, unicode.IsSpace)
	if trimmed == "" {
		return false
	}
	if statementNumber.MatchString(trimmed) {
		return false
	}
	first, _ := utf8.DecodeRuneInString(trimmed)
	return strings.ContainsRune("=+-@＝＋－＠", first)
}

func validateStatementCSV(ctx context.Context, data []byte) string {
	if !utf8.Valid(data) {
		return "INVALID_CSV"
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	r := csv.NewReader(bytes.NewReader(data))
	rows, cells := 0, 0
	for {
		if ctx.Err() != nil {
			return "CHECK_CANCELLED"
		}
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "INVALID_CSV"
		}
		rows++
		cells += len(record)
		if rows > statementRows || len(record) > statementColumns || cells > statementCellLimit {
			return "SPREADSHEET_LIMIT"
		}
		for _, field := range record {
			if len(field) > 8192 {
				return "SPREADSHEET_LIMIT"
			}
			for _, c := range field {
				if unicode.IsControl(c) && c != '\r' && c != '\n' {
					return "INVALID_CSV"
				}
			}
			if activeCSVField(field) {
				return "ACTIVE_SPREADSHEET_CONTENT"
			}
		}
	}
	if rows == 0 {
		return "INVALID_CSV"
	}
	return ""
}

func statementPartAllowed(name string) bool {
	switch name {
	case "[Content_Types].xml", "_rels/.rels", "docProps/app.xml", "docProps/core.xml", "xl/workbook.xml", "xl/_rels/workbook.xml.rels", "xl/styles.xml", "xl/sharedStrings.xml":
		return true
	}
	return statementSheetPart.MatchString(name) || statementThemePart.MatchString(name)
}

type statementRelationship struct{ ID, Type, Target string }
type statementXMLProfile struct {
	Cells, Tokens int
	Sheets        []string
	WorkbookType  bool
	Roots         map[string]string
	Namespaces    map[string]string
	SheetIDs      map[string]bool
	Relations     map[string][]statementRelationship
}

func xmlStatementAttribute(start xml.StartElement, name string) string {
	for _, a := range start.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func statementReferenceBounded(value string) bool {
	m := statementCellRef.FindStringSubmatch(value)
	if len(m) != 3 {
		return false
	}
	row, err := strconv.Atoi(m[2])
	if err != nil || row > statementRows {
		return false
	}
	column := 0
	for _, c := range m[1] {
		column = column*26 + int(c-'A') + 1
	}
	return column <= statementColumns
}

func statementFormulaAllowed(value string) bool {
	if len(value) > 4096 || strings.ContainsAny(value, "[]{}|\\") {
		return false
	}
	allowed := map[string]bool{"SUM": true, "SUMIF": true, "SUMIFS": true, "AVERAGE": true, "AVERAGEIF": true, "AVERAGEIFS": true, "MIN": true, "MAX": true, "COUNT": true, "COUNTA": true, "COUNTIF": true, "COUNTIFS": true, "IF": true, "IFERROR": true, "IFNA": true, "AND": true, "OR": true, "NOT": true, "ROUND": true, "ROUNDUP": true, "ROUNDDOWN": true, "ABS": true, "MOD": true, "INT": true, "SUMPRODUCT": true, "INDEX": true, "MATCH": true, "VLOOKUP": true, "HLOOKUP": true, "ISNUMBER": true, "ISTEXT": true, "ISBLANK": true, "ISERROR": true, "VALUE": true, "TEXT": true, "LEFT": true, "RIGHT": true, "MID": true, "LEN": true, "TRIM": true, "CONCATENATE": true, "DATE": true, "YEAR": true, "MONTH": true, "DAY": true}
	withoutLiterals := statementQuotedText.ReplaceAllString(value, `""`)
	for _, match := range statementFunction.FindAllStringSubmatch(withoutLiterals, -1) {
		if !allowed[strings.ToUpper(match[1])] {
			return false
		}
	}
	return true
}

func inspectStatementXML(ctx context.Context, name string, data []byte, profile *statementXMLProfile) string {
	if !utf8.Valid(data) {
		return "INVALID_WORKBOOK"
	}
	d := xml.NewDecoder(bytes.NewReader(data))
	depth, partTokens := 0, 0
	var capture strings.Builder
	captureDepth := 0
	for {
		if ctx.Err() != nil {
			return "CHECK_CANCELLED"
		}
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "INVALID_WORKBOOK"
		}
		partTokens++
		profile.Tokens++
		if partTokens > 300000 || profile.Tokens > 1000000 {
			return "SPREADSHEET_LIMIT"
		}
		switch t := token.(type) {
		case xml.Directive:
			return "ACTIVE_SPREADSHEET_CONTENT"
		case xml.ProcInst:
			if t.Target != "xml" || partTokens != 1 {
				return "ACTIVE_SPREADSHEET_CONTENT"
			}
		case xml.StartElement:
			depth++
			if depth > 64 {
				return "SPREADSHEET_LIMIT"
			}
			if depth == 1 {
				if profile.Roots[name] != "" {
					return "INVALID_WORKBOOK"
				}
				profile.Roots[name] = t.Name.Local
				profile.Namespaces[name] = t.Name.Space
			}
			switch t.Name.Local {
			case "Relationship":
				if xmlStatementAttribute(t, "TargetMode") != "" && xmlStatementAttribute(t, "TargetMode") != "Internal" {
					return "ACTIVE_SPREADSHEET_CONTENT"
				}
				relation := statementRelationship{xmlStatementAttribute(t, "Id"), xmlStatementAttribute(t, "Type"), xmlStatementAttribute(t, "Target")}
				if relation.ID == "" || relation.Type == "" || relation.Target == "" || strings.ContainsAny(relation.Target, "\\%?#:") {
					return "INVALID_WORKBOOK"
				}
				for _, prior := range profile.Relations[name] {
					if prior.ID == relation.ID {
						return "INVALID_WORKBOOK"
					}
				}
				profile.Relations[name] = append(profile.Relations[name], relation)
			case "Override", "Default":
				contentType := xmlStatementAttribute(t, "ContentType")
				lower := strings.ToLower(contentType)
				if strings.Contains(lower, "macro") || strings.Contains(lower, "vba") || strings.Contains(lower, "external") {
					return "ACTIVE_SPREADSHEET_CONTENT"
				}
				if xmlStatementAttribute(t, "PartName") == "/xl/workbook.xml" && contentType == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml" {
					profile.WorkbookType = true
				}
			case "sheet":
				if name == "xl/workbook.xml" {
					id := xmlStatementAttribute(t, "id")
					sheetID := xmlStatementAttribute(t, "sheetId")
					if sheetID == "" || profile.SheetIDs[sheetID] {
						return "INVALID_WORKBOOK"
					}
					profile.SheetIDs[sheetID] = true
					if id == "" || len(profile.Sheets) >= 32 {
						return "SPREADSHEET_LIMIT"
					}
					profile.Sheets = append(profile.Sheets, id)
				}
			case "row":
				if statementSheetPart.MatchString(name) {
					row, e := strconv.Atoi(xmlStatementAttribute(t, "r"))
					if e != nil || row < 1 || row > statementRows {
						return "SPREADSHEET_LIMIT"
					}
				}
			case "c":
				if statementSheetPart.MatchString(name) {
					profile.Cells++
					if profile.Cells > statementCellLimit || !statementReferenceBounded(xmlStatementAttribute(t, "r")) {
						return "SPREADSHEET_LIMIT"
					}
				}
			case "f", "definedName":
				capture.Reset()
				captureDepth = depth
			case "externalLink", "externalReferences", "oleObjects", "legacyDrawing", "drawing", "webPublishItems", "connections", "customWorkbookViews", "sheetProtection":
				return "ACTIVE_SPREADSHEET_CONTENT"
			}
		case xml.CharData:
			if len(t) > 8192 {
				return "SPREADSHEET_LIMIT"
			}
			if captureDepth > 0 {
				capture.Write(t)
				if capture.Len() > 4096 {
					return "SPREADSHEET_LIMIT"
				}
			}
		case xml.EndElement:
			if depth == captureDepth {
				if !statementFormulaAllowed(capture.String()) {
					return "ACTIVE_SPREADSHEET_CONTENT"
				}
				captureDepth = 0
			}
			depth--
		}
	}
	if depth != 0 || profile.Roots[name] == "" {
		return "INVALID_WORKBOOK"
	}
	return ""
}

func validateStatementWorkbook(ctx context.Context, data []byte) string {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "INVALID_WORKBOOK"
	}
	if len(z.File) < 5 || len(z.File) > 256 {
		return "SPREADSHEET_LIMIT"
	}
	parts := map[string]bool{}
	profile := statementXMLProfile{Roots: map[string]string{}, Namespaces: map[string]string{}, SheetIDs: map[string]bool{}, Relations: map[string][]statementRelationship{}}
	expanded := 0
	for _, file := range z.File {
		if ctx.Err() != nil {
			return "CHECK_CANCELLED"
		}
		if parts[file.Name] || path.Clean(file.Name) != file.Name || !statementPartAllowed(file.Name) || file.Flags&1 != 0 || (file.Method != zip.Store && file.Method != zip.Deflate) {
			return "UNSUPPORTED_WORKBOOK_PART"
		}
		parts[file.Name] = true
		if file.UncompressedSize64 > statementPartBytes || file.UncompressedSize64 > uint64(statementExpandedBytes-expanded) {
			return "SPREADSHEET_LIMIT"
		}
		reader, e := file.Open()
		if e != nil {
			return "INVALID_WORKBOOK"
		}
		part, e := io.ReadAll(io.LimitReader(reader, statementPartBytes+1))
		closeErr := reader.Close()
		if e != nil || closeErr != nil || uint64(len(part)) != file.UncompressedSize64 {
			return "INVALID_WORKBOOK"
		}
		expanded += len(part)
		if len(part) > statementPartBytes || expanded > statementExpandedBytes {
			return "SPREADSHEET_LIMIT"
		}
		if code := inspectStatementXML(ctx, file.Name, part, &profile); code != "" {
			return code
		}
	}
	for name := range parts {
		root, namespace := "", ""
		switch name {
		case "[Content_Types].xml":
			root, namespace = "Types", "http://schemas.openxmlformats.org/package/2006/content-types"
		case "_rels/.rels", "xl/_rels/workbook.xml.rels":
			root, namespace = "Relationships", "http://schemas.openxmlformats.org/package/2006/relationships"
		case "docProps/app.xml":
			root, namespace = "Properties", "http://schemas.openxmlformats.org/officeDocument/2006/extended-properties"
		case "docProps/core.xml":
			root, namespace = "coreProperties", "http://schemas.openxmlformats.org/package/2006/metadata/core-properties"
		case "xl/workbook.xml":
			root = "workbook"
		case "xl/styles.xml":
			root = "styleSheet"
		case "xl/sharedStrings.xml":
			root = "sst"
		default:
			if statementSheetPart.MatchString(name) {
				root = "worksheet"
			} else if statementThemePart.MatchString(name) {
				root, namespace = "theme", "http://schemas.openxmlformats.org/drawingml/2006/main"
			}
		}
		if namespace == "" {
			namespace = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
		}
		if root == "" || profile.Roots[name] != root || profile.Namespaces[name] != namespace {
			return "INVALID_WORKBOOK"
		}
	}
	if !profile.WorkbookType || profile.Roots["[Content_Types].xml"] != "Types" || profile.Roots["_rels/.rels"] != "Relationships" || profile.Roots["xl/workbook.xml"] != "workbook" || len(profile.Sheets) == 0 {
		return "INVALID_WORKBOOK"
	}
	workbookReferenced := false
	for name, relations := range profile.Relations {
		base := ""
		if name == "xl/_rels/workbook.xml.rels" {
			base = "xl"
		}
		for _, relation := range relations {
			target := strings.TrimPrefix(relation.Target, "/")
			if !strings.HasPrefix(relation.Target, "/") {
				target = path.Join(base, target)
			}
			if !parts[target] || strings.Contains(relation.Type, "external") || strings.Contains(relation.Target, "..") {
				return "UNSUPPORTED_WORKBOOK_PART"
			}
			if name == "_rels/.rels" && target == "xl/workbook.xml" && strings.HasSuffix(relation.Type, "/officeDocument") {
				workbookReferenced = true
			}
		}
	}
	if !workbookReferenced {
		return "INVALID_WORKBOOK"
	}
	seen := map[string]bool{}
	for _, id := range profile.Sheets {
		if seen[id] {
			return "INVALID_WORKBOOK"
		}
		seen[id] = true
		found := false
		for _, relation := range profile.Relations["xl/_rels/workbook.xml.rels"] {
			if relation.ID == id && strings.HasSuffix(relation.Type, "/worksheet") {
				target := strings.TrimPrefix(relation.Target, "/")
				if !strings.HasPrefix(relation.Target, "/") {
					target = path.Join("xl", target)
				}
				found = statementSheetPart.MatchString(target) && profile.Roots[target] == "worksheet"
			}
		}
		if !found {
			return "INVALID_WORKBOOK"
		}
	}
	return ""
}
