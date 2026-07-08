package backend

import (
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestInspectTableFileXLSXPreview(t *testing.T) {
	path := filepath.Join(t.TempDir(), "supplement.xlsx")
	file := excelize.NewFile()
	index, err := file.NewSheet("Table S1")
	if err != nil {
		t.Fatal(err)
	}
	file.SetActiveSheet(index)
	if err := file.SetSheetRow("Table S1", "A1", &[]any{"Supplementary Table 1"}); err != nil {
		t.Fatal(err)
	}
	if err := file.SetSheetRow("Table S1", "A2", &[]any{"Gene", "Position", "P value"}); err != nil {
		t.Fatal(err)
	}
	if err := file.SetSheetRow("Table S1", "A3", &[]any{"ABC1", "chr1:10", "0.01"}); err != nil {
		t.Fatal(err)
	}
	if err := file.SetSheetName("Sheet1", "Legend"); err != nil {
		t.Fatal(err)
	}
	if err := file.SetSheetRow("Legend", "A1", &[]any{"Table contents:"}); err != nil {
		t.Fatal(err)
	}
	if err := file.SaveAs(path); err != nil {
		t.Fatal(err)
	}

	got, err := InspectTableFile(path, TableInspectOptions{Head: 2, MaxSheets: 1})
	if err != nil {
		t.Fatalf("InspectTableFile() error = %v", err)
	}
	if got.FileType != "xlsx" || got.SheetCount != 2 || !got.SheetsTruncated {
		t.Fatalf("unexpected workbook summary: %#v", got)
	}
	if len(got.Sheets) != 1 || got.Sheets[0].Name != "Legend" {
		t.Fatalf("unexpected sheets: %#v", got.Sheets)
	}

	got, err = InspectTableFile(path, TableInspectOptions{Sheet: "Table S1", Head: 3})
	if err != nil {
		t.Fatalf("InspectTableFile(sheet) error = %v", err)
	}
	sheet := got.Sheets[0]
	if sheet.Rows != 3 || sheet.Columns != 3 {
		t.Fatalf("unexpected dimensions: %#v", sheet)
	}
	if sheet.Kind != "table" {
		t.Fatalf("Kind = %q, want table", sheet.Kind)
	}
	if sheet.HeaderRow != 2 {
		t.Fatalf("HeaderRow = %d, want 2", sheet.HeaderRow)
	}
	if len(sheet.Preview) != 3 || sheet.Preview[1][0] != "Gene" {
		t.Fatalf("unexpected preview: %#v", sheet.Preview)
	}
}
