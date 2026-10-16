package documents

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestStatementPreviewPreservesExactTextAndPagesWithoutEvaluating(t *testing.T) {
	text := "Description,Amount\n"
	for i := 1; i <= 62; i++ {
		text += fmt.Sprintf("Item %d,99999999999999.01\n", i)
	}
	out, err := PreviewStatement(context.Background(), "income.csv", []byte(text), 1, 2)
	if err != nil || out.TotalRows != 63 || out.Columns != 2 || len(out.Rows) != 13 || out.Rows[0][0] != "Item 50" || out.Rows[0][1] != "99999999999999.01" {
		t.Fatalf("CSV: %#v %v", out, err)
	}
	parts := statementWorkbookParts()
	parts["xl/worksheets/sheet1.xml"] = strings.Replace(parts["xl/worksheets/sheet1.xml"], "<v>432.19</v></c></row></sheetData>", "</c></row></sheetData>", 1)
	out, err = PreviewStatement(context.Background(), "income.xlsx", zippedStatement(t, parts, false), 1, 1)
	if err != nil || out.Sheets[0] != "Income" || out.TotalRows != 2 || out.Rows[0][0] != "Fictional supplied amount" || out.Rows[0][1] != "432.19" || out.Rows[1][0] != "" || out.Rows[1][1] != "=SUM(B1:B1) (no cached value)" {
		t.Fatalf("workbook: %#v %v", out, err)
	}
	for _, bad := range []struct{ sheet, page int }{{0, 1}, {1, 0}, {2, 1}, {1, 1001}} {
		if _, err := PreviewStatement(context.Background(), "income.csv", []byte(text), bad.sheet, bad.page); err == nil {
			t.Fatal("invalid bounds accepted", bad)
		}
	}
	if _, err := PreviewStatement(context.Background(), "income.csv", []byte("x\n=1+1\n"), 1, 1); err == nil {
		t.Fatal("active source accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := PreviewStatement(ctx, "income.csv", []byte(text), 1, 1); err == nil {
		t.Fatal("cancellation ignored")
	}
}

func TestStatementPreviewWorksheetRelationsSharedStringsAndSparseRows(t *testing.T) {
	parts := statementWorkbookParts()
	parts["[Content_Types].xml"] = strings.Replace(parts["[Content_Types].xml"], "</Types>", `<Override PartName="/xl/worksheets/sheet2.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`, 1)
	parts["xl/workbook.xml"] = strings.Replace(parts["xl/workbook.xml"], "</sheets>", `<sheet name="Balance" sheetId="2" r:id="balance"/></sheets>`, 1)
	parts["xl/_rels/workbook.xml.rels"] = strings.Replace(parts["xl/_rels/workbook.xml.rels"], "</Relationships>", `<Relationship Id="balance" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet2.xml"/></Relationships>`, 1)
	parts["xl/worksheets/sheet2.xml"] = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="51"><c r="C51" t="inlineStr"><is><r><t>&lt;script&gt;</t></r><r><t> literal</t></r></is></c><c r="D51"><v>0.01</v></c></row></sheetData></worksheet>`
	out, err := PreviewStatement(context.Background(), "balance.xlsx", zippedStatement(t, parts, false), 2, 2)
	if err != nil || len(out.Sheets) != 2 || out.Sheets[1] != "Balance" || out.TotalRows != 51 || out.Columns != 4 || len(out.Rows) != 1 || out.Rows[0][2] != "<script> literal" || out.Rows[0][3] != "0.01" {
		t.Fatalf("second sheet: %#v %v", out, err)
	}
}
