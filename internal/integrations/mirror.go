package integrations

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// MirrorRow writes only explicitly mapped Space-owned columns. Metadata binds
// the physical row across sorting/insertion; row numbers are enrollment hints.
// A newly enrolled row is read back and its stable key checked before writing.
func (s *Sources) MirrorRow(ctx context.Context, c SheetsConfig, connector string, item SourceItem, values map[string]string) (bool, error) {
	if err := ValidateSheets(&c); err != nil {
		return false, err
	}
	if c.KeyColumn == 0 || len(c.MirrorColumns) == 0 || strings.HasPrefix(item.Key, "derived:") {
		return false, errors.New("mirror memerlukan ID stabil dan kolom khusus Space")
	}
	token, err := s.googleToken(ctx)
	if err != nil {
		return false, err
	}
	base := "https://sheets.googleapis.com/v4/spreadsheets/" + url.PathEscape(c.SpreadsheetID)
	var sheet struct {
		Sheets []struct {
			Properties struct {
				SheetID int    `json:"sheetId"`
				Title   string `json:"title"`
			} `json:"properties"`
		} `json:"sheets"`
	}
	if err := s.request(ctx, "GET", base+"?fields=sheets.properties", token, nil, &sheet); err != nil {
		return false, err
	}
	sheetID := -1
	for _, tab := range sheet.Sheets {
		if tab.Properties.Title == c.Tab {
			sheetID = tab.Properties.SheetID
		}
	}
	if sheetID < 0 {
		return false, errors.New("tab mirror tidak ditemukan")
	}
	headerRange := "'" + strings.ReplaceAll(c.Tab, "'", "''") + "'!" + fmt.Sprint(c.HeaderRow) + ":" + fmt.Sprint(c.HeaderRow)
	var headers struct {
		Values [][]any `json:"values"`
	}
	if err := s.request(ctx, "GET", base+"/values/"+url.PathEscape(headerRange)+"?valueRenderOption=FORMULA", token, nil, &headers); err != nil {
		return false, err
	}
	if len(headers.Values) != 1 {
		return false, errors.New("header mirror tidak tersedia")
	}
	expectedHeaders := map[string]string{"progress": "space_progress", "matchStatus": "space_match_status", "evidenceUrl": "space_evidence_url"}
	for field, col := range c.MirrorColumns {
		if col > len(headers.Values[0]) || strings.ToLower(strings.TrimSpace(fmt.Sprint(headers.Values[0][col-1]))) != expectedHeaders[field] {
			return false, fmt.Errorf("kolom mirror %s harus memiliki header %s; kolom manual/formula tidak diambil alih", field, expectedHeaders[field])
		}
	}
	lookup := map[string]any{"metadataKey": "space.connector." + connector, "metadataValue": Hash(item.Key), "visibility": "DOCUMENT"}
	type metadata struct {
		MetadataID int `json:"metadataId"`
		Location   struct {
			DimensionRange struct {
				SheetID    int    `json:"sheetId"`
				Dimension  string `json:"dimension"`
				StartIndex int    `json:"startIndex"`
				EndIndex   int    `json:"endIndex"`
			} `json:"dimensionRange"`
		} `json:"location"`
	}
	var search struct {
		Matched []struct {
			Metadata metadata `json:"developerMetadata"`
		} `json:"matchedDeveloperMetadata"`
	}
	if err := s.request(ctx, "POST", base+"/developerMetadata:search", token, map[string]any{"dataFilters": []any{map[string]any{"developerMetadataLookup": lookup}}}, &search); err != nil {
		return false, err
	}
	metadataID := 0
	if len(search.Matched) > 1 {
		return false, errors.New("metadata row berulang; perlu tinjauan pemetaan")
	}
	if len(search.Matched) == 1 {
		m := search.Matched[0].Metadata
		if m.Location.DimensionRange.SheetID != sheetID || m.Location.DimensionRange.Dimension != "ROWS" || m.Location.DimensionRange.EndIndex-m.Location.DimensionRange.StartIndex != 1 {
			return false, errors.New("lokasi metadata tidak cocok dengan satu baris sumber")
		}
		metadataID = m.MetadataID
	} else {
		rows, err := s.SheetValues(ctx, c, false)
		if err != nil {
			return false, err
		}
		parsed, err := ParseRows(c, rows)
		if err != nil {
			return false, err
		}
		row := 0
		for _, candidate := range parsed {
			if candidate.Key == item.Key {
				if candidate.Fingerprint != item.Fingerprint || candidate.Issue != "" {
					return false, errors.New("baris berubah sejak pratinjau; mirror ditunda")
				}
				row = candidate.Row
			}
		}
		if row == 0 {
			return false, errors.New("baris sumber hilang; mirror tidak menghidupkan atau membuat baris otomatis")
		}
		var created struct {
			Replies []struct {
				Create struct {
					Metadata metadata `json:"developerMetadata"`
				} `json:"createDeveloperMetadata"`
			} `json:"replies"`
		}
		payload := map[string]any{"requests": []any{map[string]any{"createDeveloperMetadata": map[string]any{"developerMetadata": map[string]any{"metadataKey": lookup["metadataKey"], "metadataValue": lookup["metadataValue"], "visibility": "DOCUMENT", "location": map[string]any{"dimensionRange": map[string]any{"sheetId": sheetID, "dimension": "ROWS", "startIndex": row - 1, "endIndex": row}}}}}}}
		if err := s.request(ctx, "POST", base+":batchUpdate", token, payload, &created); err != nil {
			return false, fmt.Errorf("pendaftaran metadata belum dapat dipastikan: %w", err)
		}
		if len(created.Replies) != 1 {
			return false, errors.New("hasil pendaftaran metadata belum dapat dipastikan")
		}
		metadataID = created.Replies[0].Create.Metadata.MetadataID
	}
	if metadataID == 0 {
		return false, errors.New("metadata baris tidak valid")
	}
	filter := map[string]any{"developerMetadataLookup": map[string]any{"metadataId": metadataID}}
	readRow := func(render string) ([]any, error) {
		var out struct {
			Ranges []struct {
				ValueRange struct {
					Values [][]any `json:"values"`
				} `json:"valueRange"`
			} `json:"valueRanges"`
		}
		body := map[string]any{"dataFilters": []any{filter}, "majorDimension": "ROWS", "valueRenderOption": render, "dateTimeRenderOption": "SERIAL_NUMBER"}
		if err := s.request(ctx, "POST", base+"/values:batchGetByDataFilter", token, body, &out); err != nil {
			return nil, err
		}
		if len(out.Ranges) != 1 || len(out.Ranges[0].ValueRange.Values) != 1 {
			return nil, errors.New("metadata tidak menunjuk tepat satu baris yang tersedia")
		}
		return out.Ranges[0].ValueRange.Values[0], nil
	}
	raw, err := readRow("UNFORMATTED_VALUE")
	if err != nil {
		return false, err
	}
	check := c
	check.HeaderRow = 1
	parsed, err := ParseRows(check, [][]any{{}, raw})
	if err != nil || len(parsed) != 1 || parsed[0].Key != item.Key || parsed[0].Fingerprint != item.Fingerprint || parsed[0].Issue != "" {
		return false, errors.New("identitas atau isi baris berubah; tidak menulis sel apa pun")
	}
	formulas, err := readRow("FORMULA")
	if err != nil {
		return false, err
	}
	width := 0
	changed := false
	for field, col := range c.MirrorColumns {
		if col <= len(formulas) && strings.HasPrefix(strings.TrimSpace(fmt.Sprint(formulas[col-1])), "=") {
			return false, errors.New("kolom mirror berisi formula; penulisan dibatalkan")
		}
		old := ""
		if col <= len(raw) && raw[col-1] != nil {
			old = fmt.Sprint(raw[col-1])
		}
		if old != values[field] {
			changed = true
		}
		if col > width {
			width = col
		}
	}
	if !changed {
		return false, nil
	}
	// Null inputs leave unowned columns unchanged; empty strings deliberately
	// clear only an explicitly owned mirror column.
	cells := make([]any, width)
	for field, col := range c.MirrorColumns {
		cells[col-1] = values[field]
	}
	var result struct {
		TotalUpdatedCells int `json:"totalUpdatedCells"`
	}
	body := map[string]any{"valueInputOption": "RAW", "includeValuesInResponse": false, "data": []any{map[string]any{"dataFilter": filter, "majorDimension": "ROWS", "values": [][]any{cells}}}}
	if err := s.request(ctx, "POST", base+"/values:batchUpdateByDataFilter", token, body, &result); err != nil {
		return false, fmt.Errorf("status mirror belum dapat dipastikan; baca ulang sebelum retry: %w", err)
	}
	if result.TotalUpdatedCells == 0 {
		return false, errors.New("tidak ada sel diperbarui; baris mungkin sudah dihapus")
	}
	return true, nil
}
