package backend

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

type TableInspectOptions struct {
	Sheet      string
	Head       int
	MaxSheets  int
	MaxColumns int
}

type TableWorkbook struct {
	FileType        string       `json:"file_type"`
	Path            string       `json:"path,omitempty"`
	SheetCount      int          `json:"sheet_count"`
	Sheets          []TableSheet `json:"sheets"`
	SheetsTruncated bool         `json:"sheets_truncated,omitempty"`
}

type TableSheet struct {
	Name               string     `json:"name"`
	Rows               int        `json:"rows"`
	Columns            int        `json:"columns"`
	Preview            [][]string `json:"preview,omitempty"`
	PreviewTruncated   bool       `json:"preview_truncated,omitempty"`
	ColumnsTruncated   bool       `json:"columns_truncated,omitempty"`
	HeaderRow          int        `json:"header_row,omitempty"`
	HeaderRowHeuristic bool       `json:"header_row_heuristic,omitempty"`
	Kind               string     `json:"kind,omitempty"`
}

func InspectTableFile(path string, opts TableInspectOptions) (TableWorkbook, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".xlsx", ".xlsm", ".xltx", ".xltm":
		return inspectXLSXFile(path, opts)
	default:
		return TableWorkbook{}, fmt.Errorf("unsupported table file type %q; currently only .xlsx/.xlsm are supported", ext)
	}
}

func inspectXLSXFile(path string, opts TableInspectOptions) (TableWorkbook, error) {
	opts = normalizeTableInspectOptions(opts)
	file, err := excelize.OpenFile(path, excelize.Options{RawCellValue: true})
	if err != nil {
		return TableWorkbook{}, err
	}
	defer file.Close()

	sheetNames := file.GetSheetList()
	selected, truncated, err := selectSheets(sheetNames, opts)
	if err != nil {
		return TableWorkbook{}, err
	}

	workbook := TableWorkbook{
		FileType:        "xlsx",
		Path:            path,
		SheetCount:      len(sheetNames),
		Sheets:          make([]TableSheet, 0, len(selected)),
		SheetsTruncated: truncated,
	}
	for _, sheetName := range selected {
		sheet, err := inspectXLSXSheet(file, sheetName, opts.Head, opts.MaxColumns)
		if err != nil {
			return TableWorkbook{}, err
		}
		workbook.Sheets = append(workbook.Sheets, sheet)
	}
	return workbook, nil
}

func normalizeTableInspectOptions(opts TableInspectOptions) TableInspectOptions {
	if opts.Head <= 0 {
		opts.Head = 5
	}
	if opts.MaxSheets <= 0 {
		opts.MaxSheets = 5
	}
	if opts.MaxColumns <= 0 {
		opts.MaxColumns = 12
	}
	return opts
}

func selectSheets(sheetNames []string, opts TableInspectOptions) ([]string, bool, error) {
	if strings.TrimSpace(opts.Sheet) != "" {
		for _, name := range sheetNames {
			if name == opts.Sheet {
				return []string{name}, false, nil
			}
		}
		for _, name := range sheetNames {
			if strings.EqualFold(name, opts.Sheet) {
				return []string{name}, false, nil
			}
		}
		return nil, false, fmt.Errorf("sheet not found: %s", opts.Sheet)
	}
	if len(sheetNames) <= opts.MaxSheets {
		return append([]string(nil), sheetNames...), false, nil
	}
	return append([]string(nil), sheetNames[:opts.MaxSheets]...), true, nil
}

func inspectXLSXSheet(file *excelize.File, sheetName string, head int, maxColumns int) (TableSheet, error) {
	rows, err := file.Rows(sheetName)
	if err != nil {
		return TableSheet{}, err
	}
	defer rows.Close()

	sheet := TableSheet{Name: sheetName, Kind: classifySheetName(sheetName)}
	firstRows := make([][]string, 0, 20)
	for rows.Next() {
		columns, err := rows.Columns()
		if err != nil {
			return TableSheet{}, err
		}
		trimmed := trimTrailingEmptyCells(columns)
		if len(trimmed) > sheet.Columns {
			sheet.Columns = len(trimmed)
		}
		sheet.Rows++
		if len(firstRows) < 20 {
			firstRows = append(firstRows, trimmed)
		}
		if len(sheet.Preview) < head && rowHasValue(trimmed) {
			previewRow := trimmed
			if len(previewRow) > maxColumns {
				previewRow = append([]string(nil), previewRow[:maxColumns]...)
				sheet.ColumnsTruncated = true
			}
			sheet.Preview = append(sheet.Preview, previewRow)
		} else if len(sheet.Preview) >= head && rowHasValue(trimmed) {
			sheet.PreviewTruncated = true
		}
	}
	if err := rows.Error(); err != nil {
		return TableSheet{}, err
	}
	sheet.HeaderRow = guessHeaderRow(firstRows)
	sheet.HeaderRowHeuristic = sheet.HeaderRow > 0
	return sheet, nil
}

func trimTrailingEmptyCells(cells []string) []string {
	end := len(cells)
	for end > 0 && strings.TrimSpace(cells[end-1]) == "" {
		end--
	}
	return append([]string(nil), cells[:end]...)
}

func rowHasValue(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func guessHeaderRow(rows [][]string) int {
	for i, row := range rows {
		nonEmpty := countNonEmptyCells(row)
		if nonEmpty < 2 {
			continue
		}
		nextNonEmpty := 0
		for j := i + 1; j < len(rows); j++ {
			nextNonEmpty = countNonEmptyCells(rows[j])
			if nextNonEmpty > 0 {
				break
			}
		}
		if nextNonEmpty >= 2 {
			return i + 1
		}
	}
	return 0
}

func countNonEmptyCells(row []string) int {
	count := 0
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			count++
		}
	}
	return count
}

func classifySheetName(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.Contains(normalized, "legend"):
		return "legend"
	case strings.Contains(normalized, "index"):
		return "index"
	case strings.Contains(normalized, "table"):
		return "table"
	default:
		return ""
	}
}
