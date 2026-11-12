package sheet

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// The workbook as Google publishes it, or as Space writes it: tabs by name,
// each a grid of cell texts (numbers as written, formulas as their value).

// Tab is one sheet of a workbook.
type Tab struct {
	Name   string
	Rows   [][]string
	Hidden map[int]bool // 0-based columns
}

const maxXLSX = 8 << 20

var cellRef = regexp.MustCompile(`^([A-Z]+)([0-9]+)$`)

// ReadXLSX reads every tab of an xlsx file.
func ReadXLSX(data []byte) (map[string]*Tab, error) {
	if len(data) > maxXLSX {
		return nil, errors.New("berkas sheet terlalu besar")
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	files := map[string]*zip.File{}
	for _, f := range z.File {
		files[f.Name] = f
	}
	read := func(name string) ([]byte, error) {
		f := files[name]
		if f == nil {
			return nil, fmt.Errorf("%s tidak ada", name)
		}
		r, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return io.ReadAll(io.LimitReader(r, 32<<20))
	}
	var shared []string
	if b, err := read("xl/sharedStrings.xml"); err == nil {
		var sst struct {
			SI []struct {
				T string `xml:"t"`
				R []struct {
					T string `xml:"t"`
				} `xml:"r"`
			} `xml:"si"`
		}
		if err := xml.Unmarshal(b, &sst); err != nil {
			return nil, err
		}
		for _, si := range sst.SI {
			s := si.T
			for _, r := range si.R {
				s += r.T
			}
			shared = append(shared, s)
		}
	}
	b, err := read("xl/_rels/workbook.xml.rels")
	if err != nil {
		return nil, err
	}
	var rels struct {
		R []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if err := xml.Unmarshal(b, &rels); err != nil {
		return nil, err
	}
	target := map[string]string{}
	for _, r := range rels.R {
		t := strings.TrimPrefix(r.Target, "/")
		if !strings.HasPrefix(t, "xl/") {
			t = "xl/" + t
		}
		target[r.ID] = t
	}
	if b, err = read("xl/workbook.xml"); err != nil {
		return nil, err
	}
	var wb struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			RID  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sheets>sheet"`
	}
	if err := xml.Unmarshal(b, &wb); err != nil {
		return nil, err
	}
	tabs := map[string]*Tab{}
	for _, s := range wb.Sheets {
		b, err := read(target[s.RID])
		if err != nil {
			return nil, err
		}
		var ws struct {
			Cols []struct {
				Min    int    `xml:"min,attr"`
				Max    int    `xml:"max,attr"`
				Hidden string `xml:"hidden,attr"`
			} `xml:"cols>col"`
			Rows []struct {
				R int `xml:"r,attr"`
				C []struct {
					R  string `xml:"r,attr"`
					T  string `xml:"t,attr"`
					V  string `xml:"v"`
					IS struct {
						T string `xml:"t"`
						R []struct {
							T string `xml:"t"`
						} `xml:"r"`
					} `xml:"is"`
				} `xml:"c"`
			} `xml:"sheetData>row"`
		}
		if err := xml.Unmarshal(b, &ws); err != nil {
			return nil, err
		}
		tab := &Tab{Name: s.Name, Hidden: map[int]bool{}}
		for _, c := range ws.Cols {
			if c.Hidden == "1" || c.Hidden == "true" {
				for i := c.Min; i <= c.Max && i-c.Min < 1000; i++ {
					tab.Hidden[i-1] = true
				}
			}
		}
		for _, row := range ws.Rows {
			if row.R < 1 || row.R > 100000 {
				continue
			}
			for len(tab.Rows) < row.R {
				tab.Rows = append(tab.Rows, nil)
			}
			cells := tab.Rows[row.R-1]
			for i, c := range row.C {
				col := i
				if m := cellRef.FindStringSubmatch(c.R); m != nil {
					col = colIndex(m[1])
				}
				if col < 0 || col >= 1000 {
					continue
				}
				var v string
				switch c.T {
				case "s":
					n, err := strconv.Atoi(strings.TrimSpace(c.V))
					if err == nil && n >= 0 && n < len(shared) {
						v = shared[n]
					}
				case "inlineStr":
					v = c.IS.T
					for _, r := range c.IS.R {
						v += r.T
					}
				default:
					v = c.V
				}
				for len(cells) <= col {
					cells = append(cells, "")
				}
				cells[col] = v
			}
			tab.Rows[row.R-1] = cells
		}
		tabs[s.Name] = tab
	}
	return tabs, nil
}

func colIndex(letters string) int {
	n := 0
	for _, ch := range letters {
		n = n*26 + int(ch-'A') + 1
	}
	return n - 1
}

func colName(i int) string {
	s := ""
	for i++; i > 0; i = (i - 1) / 26 {
		s = string(rune('A'+(i-1)%26)) + s
	}
	return s
}

// Cell is a cell Space writes: text, or a formula.
type Cell struct {
	Text    string
	Formula string // without "="
	Style   int    // styleText, styleHead, styleID, styleClock
}

const (
	styleText  = 0
	styleHead  = 1
	styleID    = 2
	styleClock = 3
	styleIDHd  = 4
)

// WriteTab is a tab Space writes.
type WriteTab struct {
	Name   string
	Rows   [][]Cell
	Widths []float64 // per column, 0 for default
	Hidden []int     // 0-based columns hidden
	// Lists are dropdowns: a column and its choices, for rows 2..ListRows.
	Lists    map[int][]string
	ListRows int
}

func esc(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// WriteXLSX writes tabs as an xlsx file Google Sheets imports.
func WriteXLSX(tabs []WriteTab) ([]byte, error) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	put := func(name, body string) error {
		w, err := z.Create(name)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+"\n"+body)
		return err
	}
	var shared []string
	index := map[string]int{}
	str := func(s string) int {
		if i, ok := index[s]; ok {
			return i
		}
		index[s] = len(shared)
		shared = append(shared, s)
		return len(shared) - 1
	}
	types := `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/><Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>`
	sheets, wbRels := "", ""
	for i := range tabs {
		n := strconv.Itoa(i + 1)
		types += `<Override PartName="/xl/worksheets/sheet` + n + `.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`
		sheets += `<sheet name="` + esc(tabs[i].Name) + `" sheetId="` + n + `" r:id="rId` + n + `"/>`
		wbRels += `<Relationship Id="rId` + n + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet` + n + `.xml"/>`
	}
	k := strconv.Itoa(len(tabs) + 1)
	wbRels += `<Relationship Id="rId` + k + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`
	k2 := strconv.Itoa(len(tabs) + 2)
	wbRels += `<Relationship Id="rId` + k2 + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/>`
	types += `</Types>`
	for i, tab := range tabs {
		var b strings.Builder
		b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>`)
		hidden := map[int]bool{}
		for _, c := range tab.Hidden {
			hidden[c] = true
		}
		cols := 0
		for _, r := range tab.Rows {
			cols = max(cols, len(r))
		}
		if cols > 0 {
			b.WriteString(`<cols>`)
			for c := 0; c < cols; c++ {
				w := 16.0
				if c < len(tab.Widths) && tab.Widths[c] > 0 {
					w = tab.Widths[c]
				}
				attr := ""
				if hidden[c] {
					attr = ` hidden="1"`
				}
				fmt.Fprintf(&b, `<col min="%d" max="%d" width="%g" customWidth="1"%s/>`, c+1, c+1, w, attr)
			}
			b.WriteString(`</cols>`)
		}
		b.WriteString(`<sheetData>`)
		for r, row := range tab.Rows {
			fmt.Fprintf(&b, `<row r="%d">`, r+1)
			for c, cell := range row {
				ref := colName(c) + strconv.Itoa(r+1)
				s := ""
				if cell.Style != 0 {
					s = ` s="` + strconv.Itoa(cell.Style) + `"`
				}
				switch {
				case cell.Formula != "":
					fmt.Fprintf(&b, `<c r="%s"%s><f>%s</f></c>`, ref, s, esc(cell.Formula))
				case cell.Text != "":
					fmt.Fprintf(&b, `<c r="%s"%s t="s"><v>%d</v></c>`, ref, s, str(cell.Text))
				case cell.Style != 0:
					fmt.Fprintf(&b, `<c r="%s"%s/>`, ref, s)
				}
			}
			b.WriteString(`</row>`)
		}
		b.WriteString(`</sheetData>`)
		if len(tab.Lists) > 0 && tab.ListRows > 1 {
			fmt.Fprintf(&b, `<dataValidations count="%d">`, len(tab.Lists))
			for c := 0; c < cols+len(tab.Lists); c++ {
				items, ok := tab.Lists[c]
				if !ok {
					continue
				}
				col := colName(c)
				fmt.Fprintf(&b, `<dataValidation type="list" allowBlank="1" showErrorMessage="1" errorTitle="Progres" error="Pilih dari daftar" sqref="%s2:%s%d"><formula1>"%s"</formula1></dataValidation>`, col, col, tab.ListRows, esc(strings.Join(items, ",")))
			}
			b.WriteString(`</dataValidations>`)
		}
		b.WriteString(`</worksheet>`)
		if err := put("xl/worksheets/sheet"+strconv.Itoa(i+1)+".xml", b.String()); err != nil {
			return nil, err
		}
	}
	var sst strings.Builder
	fmt.Fprintf(&sst, `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="%d" uniqueCount="%d">`, len(shared), len(shared))
	for _, s := range shared {
		sst.WriteString(`<si><t xml:space="preserve">` + esc(s) + `</t></si>`)
	}
	sst.WriteString(`</sst>`)
	// Styles: 0 text (@), 1 header (bold), 2 ID (grey, text), 3 the sheet
	// clock (date and time), 4 an ID's header (grey, bold).
	styles := `<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><numFmts count="1"><numFmt numFmtId="164" formatCode="yyyy-mm-dd hh:mm:ss"/></numFmts><fonts count="2"><font><sz val="11"/><name val="Arial"/></font><font><b/><sz val="11"/><name val="Arial"/></font></fonts><fills count="3"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FFD9D9D9"/><bgColor indexed="64"/></patternFill></fill></fills><borders count="1"><border/></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="5"><xf numFmtId="49" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/><xf numFmtId="49" fontId="1" fillId="0" borderId="0" xfId="0" applyNumberFormat="1" applyFont="1"/><xf numFmtId="49" fontId="0" fillId="2" borderId="0" xfId="0" applyNumberFormat="1" applyFill="1"/><xf numFmtId="164" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/><xf numFmtId="49" fontId="1" fillId="2" borderId="0" xfId="0" applyNumberFormat="1" applyFont="1" applyFill="1"/></cellXfs></styleSheet>`
	for _, p := range [][2]string{
		{"[Content_Types].xml", types},
		{"_rels/.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{"xl/workbook.xml", `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>` + sheets + `</sheets><calcPr calcId="191029" fullCalcOnLoad="1"/></workbook>`},
		{"xl/_rels/workbook.xml.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` + wbRels + `</Relationships>`},
		{"xl/styles.xml", styles},
		{"xl/sharedStrings.xml", sst.String()},
	} {
		if err := put(p[0], p[1]); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
