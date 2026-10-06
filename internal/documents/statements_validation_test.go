package documents

import (
	"archive/zip"
	"bytes"
	"context"
	"sort"
	"strings"
	"testing"
)

func TestStatementCSVChecksIndependentFieldsAndBoundsWithoutExecutingOrChangingOriginals(t *testing.T) {
	cases := []struct{ name, contents, code string }{
		{"unicode amount and quoted text", "Description,Amount\r\n\"₹ collection, fictional\",432.19\r\nExpense,-100.00\r\n\"Two\nlines\",0.01\r\n", ""},
		{"UTF8 BOM", "\xef\xbb\xbfDescription,Amount\nOpening,100.00\n", ""},
		{"inconsistent columns", "Description,Amount\nBroken\n", "INVALID_CSV"},
		{"malformed quote", "Description,Amount\n\"Broken,1.00\n", "INVALID_CSV"},
		{"invalid UTF8", "Description,Amount\n\xff,1.00\n", "INVALID_CSV"},
		{"control byte", "Description,Amount\n\x00,1.00\n", "INVALID_CSV"},
		{"tab control", "Description,Amount\n\t=1+1,1.00\n", "INVALID_CSV"},
		{"formula", "Description,Amount\n=1+1,1.00\n", "ACTIVE_SPREADSHEET_CONTENT"},
		{"trimmed formula", "Description,Amount\n  =1+1,1.00\n", "ACTIVE_SPREADSHEET_CONTENT"},
		{"fullwidth formula", "Description,Amount\n＝1+1,1.00\n", "ACTIVE_SPREADSHEET_CONTENT"},
		{"DDE", "Description,Amount\n+cmd|' /C calc'!A0,1.00\n", "ACTIVE_SPREADSHEET_CONTENT"},
		{"at prefix", "Description,Amount\n@SUM(A1:A2),1.00\n", "ACTIVE_SPREADSHEET_CONTENT"},
		{"empty", "\r\n\n", "INVALID_CSV"},
		{"field limit", "Description,Amount\n" + strings.Repeat("a", 8193) + ",1.00\n", "SPREADSHEET_LIMIT"},
		{"row limit", strings.Repeat("1\n", 50001), "SPREADSHEET_LIMIT"},
		{"column limit", strings.Repeat("1,", 512) + "1\n", "SPREADSHEET_LIMIT"},
		{"cell limit", strings.Repeat("1,2,3\n", 33334), "SPREADSHEET_LIMIT"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data := []byte(c.contents)
			before := append([]byte{}, data...)
			mime, code := ValidateStatementOriginal(context.Background(), "prepared.CSV", data)
			if code != c.code || (code == "" && mime != "text/csv; charset=utf-8") || (code != "" && mime != "") {
				t.Fatalf("mime=%q code=%q; want code=%q", mime, code, c.code)
			}
			if !bytes.Equal(data, before) {
				t.Fatal("validation changed the original bytes")
			}
		})
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, code := ValidateStatementOriginal(cancelled, "prepared.csv", []byte("a,b\n1,2")); code != "CHECK_CANCELLED" {
		t.Fatal(code)
	}
}

// Independent minimal SpreadsheetML package from the documented workbook,
// sheet relationship and worksheet structure, with an ordinary SUM formula.
func statementWorkbookParts() map[string]string {
	return map[string]string{
		"[Content_Types].xml":        `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`,
		"_rels/.rels":                `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="root" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Income" sheetId="1" r:id="income"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="income" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml":   `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>Fictional supplied amount</t></is></c><c r="B1"><v>432.19</v></c></row><row r="2"><c r="B2"><f>SUM(B1:B1)</f><v>432.19</v></c></row></sheetData></worksheet>`,
	}
}

func zippedStatement(t *testing.T, parts map[string]string, duplicate bool) []byte {
	t.Helper()
	var buffer bytes.Buffer
	w := zip.NewWriter(&buffer)
	names := make([]string, 0, len(parts))
	for name := range parts {
		names = append(names, name)
	}
	sort.Strings(names)
	if duplicate {
		names = append(names, "xl/workbook.xml")
	}
	for _, name := range names {
		entry, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = entry.Write([]byte(parts[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestStatementWorkbooksInspectRealPackageLinksFormulasExpansionAndCellBounds(t *testing.T) {
	type mutation struct {
		name, code string
		change     func(map[string]string)
	}
	cases := []mutation{
		{"ordinary formula and exact bytes", "", func(map[string]string) {}},
		{"wrong workbook namespace", "INVALID_WORKBOOK", func(p map[string]string) {
			p["xl/workbook.xml"] = strings.ReplaceAll(p["xl/workbook.xml"], "http://schemas.openxmlformats.org/spreadsheetml/2006/main", "urn:not-a-workbook")
		}},
		{"wrong worksheet root", "INVALID_WORKBOOK", func(p map[string]string) {
			p["xl/worksheets/sheet1.xml"] = strings.ReplaceAll(p["xl/worksheets/sheet1.xml"], "worksheet", "notWorksheet")
		}},
		{"duplicate worksheet identity", "INVALID_WORKBOOK", func(p map[string]string) {
			p["xl/workbook.xml"] = strings.ReplaceAll(p["xl/workbook.xml"], "</sheets>", `<sheet name="Duplicate" sheetId="1" r:id="another"/></sheets>`)
			p["xl/_rels/workbook.xml.rels"] = strings.ReplaceAll(p["xl/_rels/workbook.xml.rels"], "</Relationships>", `<Relationship Id="another" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`)
		}},
		{"active network function", "ACTIVE_SPREADSHEET_CONTENT", func(p map[string]string) {
			p["xl/worksheets/sheet1.xml"] = strings.ReplaceAll(p["xl/worksheets/sheet1.xml"], "SUM(B1:B1)", `WEBSERVICE("https://example.invalid")`)
		}},
		{"DDE formula", "ACTIVE_SPREADSHEET_CONTENT", func(p map[string]string) {
			p["xl/worksheets/sheet1.xml"] = strings.ReplaceAll(p["xl/worksheets/sheet1.xml"], "SUM(B1:B1)", `cmd|' /C calc'!A0`)
		}},
		{"external reference", "ACTIVE_SPREADSHEET_CONTENT", func(p map[string]string) {
			p["xl/worksheets/sheet1.xml"] = strings.ReplaceAll(p["xl/worksheets/sheet1.xml"], "SUM(B1:B1)", `[other.xlsx]Sheet1!A1`)
		}},
		{"external relationship", "ACTIVE_SPREADSHEET_CONTENT", func(p map[string]string) {
			p["xl/_rels/workbook.xml.rels"] = strings.ReplaceAll(p["xl/_rels/workbook.xml.rels"], `Target="worksheets/sheet1.xml"`, `Target="https://example.invalid/secret" TargetMode="External"`)
		}},
		{"macro part", "UNSUPPORTED_WORKBOOK_PART", func(p map[string]string) { p["xl/vbaProject.bin"] = "private binary" }},
		{"macro workbook type", "ACTIVE_SPREADSHEET_CONTENT", func(p map[string]string) {
			p["[Content_Types].xml"] = strings.ReplaceAll(p["[Content_Types].xml"], "spreadsheetml.sheet.main+xml", "ms-excel.sheet.macroEnabled.main+xml")
		}},
		{"traversal part", "UNSUPPORTED_WORKBOOK_PART", func(p map[string]string) { p["../escaped.xml"] = "<escaped/>" }},
		{"missing sheet", "INVALID_WORKBOOK", func(p map[string]string) {
			p["xl/_rels/workbook.xml.rels"] = strings.ReplaceAll(p["xl/_rels/workbook.xml.rels"], `Id="income"`, `Id="other"`)
		}},
		{"missing document root relationship", "INVALID_WORKBOOK", func(p map[string]string) { p["_rels/.rels"] = `<Relationships/>` }},
		{"malformed XML", "INVALID_WORKBOOK", func(p map[string]string) { p["xl/workbook.xml"] += "<" }},
		{"DTD", "ACTIVE_SPREADSHEET_CONTENT", func(p map[string]string) {
			p["xl/workbook.xml"] = `<!DOCTYPE workbook [<!ENTITY escape SYSTEM "file:///tmp/private">]>` + p["xl/workbook.xml"]
		}},
		{"row coordinate bound", "SPREADSHEET_LIMIT", func(p map[string]string) {
			p["xl/worksheets/sheet1.xml"] = strings.ReplaceAll(p["xl/worksheets/sheet1.xml"], `r="1"`, `r="50001"`)
		}},
		{"column coordinate bound", "SPREADSHEET_LIMIT", func(p map[string]string) {
			p["xl/worksheets/sheet1.xml"] = strings.ReplaceAll(p["xl/worksheets/sheet1.xml"], `r="B1"`, `r="SS1"`)
		}},
		{"expanded byte limit", "SPREADSHEET_LIMIT", func(p map[string]string) { p["xl/sharedStrings.xml"] = strings.Repeat("a", statementPartBytes+1) }},
		{"depth limit", "SPREADSHEET_LIMIT", func(p map[string]string) {
			p["xl/styles.xml"] = strings.Repeat("<node>", 65) + strings.Repeat("</node>", 65)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			parts := statementWorkbookParts()
			c.change(parts)
			data := zippedStatement(t, parts, false)
			before := append([]byte{}, data...)
			mime, code := ValidateStatementOriginal(context.Background(), "prepared.xlsx", data)
			if code != c.code || (code == "" && mime != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet") || (code != "" && mime != "") {
				t.Fatalf("mime=%q code=%q; want code=%q", mime, code, c.code)
			}
			if !bytes.Equal(data, before) {
				t.Fatal("validation changed original bytes or cached formula values")
			}
		})
	}
	for _, filename := range []string{"prepared.xls", "prepared.xlsm", "prepared.zip", "prepared.png"} {
		if _, code := ValidateStatementOriginal(context.Background(), filename, []byte("not allowed")); code != "UNSUPPORTED_TYPE" {
			t.Fatal(filename, code)
		}
	}
	if _, code := ValidateStatementOriginal(context.Background(), "duplicate.xlsx", zippedStatement(t, statementWorkbookParts(), true)); code != "UNSUPPORTED_WORKBOOK_PART" {
		t.Fatal(code)
	}
	if _, code := ValidateStatementOriginal(context.Background(), "broken.xlsx", []byte("not a ZIP archive")); code != "INVALID_WORKBOOK" {
		t.Fatal(code)
	}
	if _, code := ValidateStatementOriginal(context.Background(), "large.xlsx", make([]byte, MaxStatementBytes+1)); code != "FILE_TOO_LARGE" {
		t.Fatal(code)
	}
	if _, code := ValidateStatementOriginal(context.Background(), "bad.pdf", []byte("not a PDF")); code != "INVALID_PDF" {
		t.Fatal(code)
	}
}
