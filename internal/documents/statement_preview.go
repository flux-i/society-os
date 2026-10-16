package documents

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"io"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

const previewPageSize = 50

type StatementTable struct {
	Sheets    []string   `json:"sheets"`
	Sheet     int        `json:"sheet"`
	Page      int        `json:"page"`
	PageSize  int        `json:"page_size"`
	TotalRows int        `json:"total_rows"`
	Columns   int        `json:"columns"`
	Rows      [][]string `json:"rows"`
}
type previewText struct {
	Text string `xml:"t"`
	Runs []struct {
		Text string `xml:"t"`
	} `xml:"r"`
}

func (t previewText) value() string {
	out := t.Text
	for _, r := range t.Runs {
		out += r.Text
	}
	return out
}

type previewCell struct {
	Ref     string      `xml:"r,attr"`
	Type    string      `xml:"t,attr"`
	Value   string      `xml:"v"`
	Formula *string     `xml:"f"`
	Inline  previewText `xml:"is"`
}

// PreviewStatement displays saved text and cached values. It never evaluates a formula.
// Reuse the original validation profile before decoding a bounded worksheet package.
func PreviewStatement(ctx context.Context, filename string, data []byte, sheet, pageNumber int) (StatementTable, error) {
	out := StatementTable{Sheet: sheet, Page: pageNumber, PageSize: previewPageSize, Rows: [][]string{}}
	if sheet < 1 || pageNumber < 1 || pageNumber > 1000 {
		return out, errors.New("invalid_preview_page")
	}
	if _, code := ValidateStatementOriginal(ctx, filename, data); code != "" {
		return out, errors.New(code)
	}
	start := (pageNumber - 1) * previewPageSize
	if strings.EqualFold(filepath.Ext(filename), ".csv") {
		if sheet != 1 {
			return out, errors.New("invalid_preview_sheet")
		}
		out.Sheets = []string{"Sheet 1"}
		r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})))
		for {
			if err := ctx.Err(); err != nil {
				return out, err
			}
			record, err := r.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return out, err
			}
			if out.TotalRows >= start && len(out.Rows) < previewPageSize {
				out.Rows = append(out.Rows, record)
			}
			out.TotalRows++
			out.Columns = len(record)
		}
		return out, nil
	}
	if !strings.EqualFold(filepath.Ext(filename), ".xlsx") {
		return out, errors.New("unsupported_preview_type")
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return out, err
	}
	parts := map[string]*zip.File{}
	for _, f := range z.File {
		parts[f.Name] = f
	}
	read := func(name string, target any) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		f := parts[name]
		if f == nil {
			return errors.New("missing_workbook_part")
		}
		r, err := f.Open()
		if err != nil {
			return err
		}
		defer r.Close()
		return xml.NewDecoder(io.LimitReader(r, statementPartBytes+1)).Decode(target)
	}
	var book struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			ID   string `xml:"id,attr"`
		} `xml:"sheets>sheet"`
	}
	if err = read("xl/workbook.xml", &book); err != nil {
		return out, err
	}
	for _, s := range book.Sheets {
		out.Sheets = append(out.Sheets, s.Name)
	}
	if sheet > len(book.Sheets) {
		return out, errors.New("invalid_preview_sheet")
	}
	var rels struct {
		Items []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if err = read("xl/_rels/workbook.xml.rels", &rels); err != nil {
		return out, err
	}
	target := ""
	for _, rel := range rels.Items {
		if rel.ID == book.Sheets[sheet-1].ID {
			target = path.Join("xl", rel.Target)
			if strings.HasPrefix(rel.Target, "/") {
				target = strings.TrimPrefix(rel.Target, "/")
			}
		}
	}
	if !statementSheetPart.MatchString(target) {
		return out, errors.New("invalid_preview_sheet")
	}
	var shared struct {
		Items []previewText `xml:"si"`
	}
	if parts["xl/sharedStrings.xml"] != nil {
		if err = read("xl/sharedStrings.xml", &shared); err != nil {
			return out, err
		}
	}
	var document struct {
		Rows []struct {
			Number int           `xml:"r,attr"`
			Cells  []previewCell `xml:"c"`
		} `xml:"sheetData>row"`
	}
	if err = read(target, &document); err != nil {
		return out, err
	}
	cells := map[int]map[int]string{}
	for i, row := range document.Rows {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		number := row.Number
		if number == 0 {
			number = i + 1
		}
		if number > out.TotalRows {
			out.TotalRows = number
		}
		for _, cell := range row.Cells {
			ref := statementCellRef.FindStringSubmatch(cell.Ref)
			if len(ref) != 3 {
				return out, errors.New("invalid_preview_cell")
			}
			column := 0
			for _, c := range ref[1] {
				column = column*26 + int(c-'A') + 1
			}
			if column > out.Columns {
				out.Columns = column
			}
			cellRow, _ := strconv.Atoi(ref[2])
			if cellRow > out.TotalRows {
				out.TotalRows = cellRow
			}
			if cellRow <= start || cellRow > start+previewPageSize {
				continue
			}
			value := cell.Value
			switch cell.Type {
			case "s":
				index, e := strconv.Atoi(value)
				if e != nil || index < 0 || index >= len(shared.Items) {
					return out, errors.New("invalid_shared_string")
				}
				value = shared.Items[index].value()
			case "inlineStr":
				value = cell.Inline.value()
			case "b":
				if value == "1" {
					value = "TRUE"
				} else {
					value = "FALSE"
				}
			}
			if cell.Formula != nil && value == "" {
				value = "=" + *cell.Formula + " (no cached value)"
			}
			if cells[cellRow] == nil {
				cells[cellRow] = map[int]string{}
			}
			cells[cellRow][column] = value
		}
	}
	if out.Columns == 0 {
		out.Columns = 1
	}
	end := min(out.TotalRows, start+previewPageSize)
	for n := start + 1; n <= end; n++ {
		row := make([]string, out.Columns)
		for c, value := range cells[n] {
			row[c-1] = value
		}
		out.Rows = append(out.Rows, row)
	}
	return out, nil
}
