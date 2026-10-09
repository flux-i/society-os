package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"time"
)

const RegistryImportMaxBytes = 2 * 1024 * 1024

type ImportBuilding struct {
	SourceID string `json:"source_id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
}
type ImportHome struct {
	SourceID  string `json:"source_id"`
	Building  string `json:"building"`
	Number    string `json:"number"`
	Floor     int    `json:"floor"`
	Occupancy string `json:"occupancy"`
}
type ImportPerson struct {
	SourceID string `json:"source_id"`
	Name     string `json:"name"`
}
type ImportRelationship struct {
	SourceID       string `json:"source_id"`
	Home           string `json:"home"`
	Person         string `json:"person"`
	Relationship   string `json:"relationship"`
	StartDate      string `json:"start_date"`
	EndDate        string `json:"end_date"`
	PrimaryContact bool   `json:"primary_contact"`
}
type RegistryImportInput struct {
	FormatVersion int                  `json:"format_version"`
	SocietyKey    string               `json:"society_key"`
	SourceKey     string               `json:"source_key"`
	Buildings     []ImportBuilding     `json:"buildings"`
	Homes         []ImportHome         `json:"homes"`
	People        []ImportPerson       `json:"people"`
	Relationships []ImportRelationship `json:"relationships"`
}
type ImportIssue struct {
	Section string `json:"section"`
	Row     int    `json:"row"`
	Field   string `json:"field"`
	Message string `json:"message"`
}
type ImportCounts struct {
	Buildings     int `json:"buildings"`
	Homes         int `json:"homes"`
	People        int `json:"people"`
	Relationships int `json:"relationships"`
	Occupied      int `json:"occupied"`
	Vacant        int `json:"vacant"`
	Owners        int `json:"owners"`
	Tenants       int `json:"tenants"`
}
type ImportReviewMember struct {
	SourceID       string `json:"source_id"`
	Name           string `json:"name"`
	Relationship   string `json:"relationship"`
	StartDate      string `json:"start_date"`
	EndDate        string `json:"end_date"`
	PrimaryContact bool   `json:"primary_contact"`
}
type ImportReviewHome struct {
	SourceID  string               `json:"source_id"`
	Label     string               `json:"label"`
	Floor     int                  `json:"floor"`
	Occupancy string               `json:"occupancy"`
	Members   []ImportReviewMember `json:"members"`
}
type RegistryImportPreview struct {
	Digest         string                `json:"digest"`
	BaseDigest     string                `json:"base_digest"`
	EffectiveDate  string                `json:"effective_date"`
	SourceKey      string                `json:"source_key"`
	Counts         ImportCounts          `json:"counts"`
	Errors         []ImportIssue         `json:"errors"`
	ErrorCount     int                   `json:"error_count"`
	Warnings       []ImportIssue         `json:"warnings"`
	Homes          []ImportReviewHome    `json:"homes"`
	CanApply       bool                  `json:"can_apply"`
	AlreadyApplied *RegistryImportResult `json:"already_applied,omitempty"`
}
type RegistryImportResult struct {
	ID            string       `json:"id"`
	SourceKey     string       `json:"source_key"`
	Digest        string       `json:"digest"`
	EffectiveDate string       `json:"effective_date"`
	Counts        ImportCounts `json:"counts"`
	CreatedAt     int64        `json:"created_at"`
}
type RegistryImportStatus struct {
	SocietyKey             string                `json:"society_key"`
	InitialImportAvailable bool                  `json:"initial_import_available"`
	RegistryCounts         Counts                `json:"registry_counts"`
	Applied                *RegistryImportResult `json:"applied,omitempty"`
	Mode                   string                `json:"mode"`
}
type ApplyRegistryImport struct {
	InputText     string `json:"input_text"`
	Digest        string `json:"digest"`
	BaseDigest    string `json:"base_digest"`
	EffectiveDate string `json:"effective_date"`
	OperationKey  string `json:"operation_key"`
	Confirmed     bool   `json:"confirmed"`
	Note          string `json:"note"`
}

func validateRegistryImport(body string) (RegistryImportInput, RegistryImportPreview) {
	var input RegistryImportInput
	p := RegistryImportPreview{Digest: InputDigest([]byte(body)), Errors: []ImportIssue{}, Warnings: []ImportIssue{}, Homes: []ImportReviewHome{}, EffectiveDate: today()}
	issue := func(section string, row int, field, message string) {
		p.ErrorCount++
		if len(p.Errors) < 50 {
			p.Errors = append(p.Errors, ImportIssue{section, row, field, message})
		}
	}
	warn := func(section string, row int, field, message string) {
		if len(p.Warnings) < 50 {
			p.Warnings = append(p.Warnings, ImportIssue{section, row, field, message})
		}
	}
	if len(body) == 0 || len(body) > RegistryImportMaxBytes || strictJSON([]byte(body), &input) != nil {
		issue("file", 0, "format", "Supply valid version-1 JSON, at most 2 MiB, with only documented fields.")
		return input, p
	}
	p.SourceKey = input.SourceKey
	if input.FormatVersion != 1 {
		issue("file", 0, "format_version", "Use format version 1.")
	}
	if !sourceIdentity.MatchString(input.SocietyKey) {
		issue("file", 0, "society_key", "Supply a stable society key of 1–80 letters, digits, dots, underscores or hyphens.")
	}
	if !sourceIdentity.MatchString(input.SourceKey) {
		issue("file", 0, "source_key", "Supply a stable source key of 1–80 letters, digits, dots, underscores or hyphens.")
	}
	if len(input.Buildings) < 1 || len(input.Buildings) > 100 || len(input.Homes) < 1 || len(input.Homes) > 1000 || len(input.People) < 1 || len(input.People) > 4000 || len(input.Relationships) < 1 || len(input.Relationships) > 8000 {
		issue("file", 0, "rows", "Supply 1–100 buildings, 1–1,000 homes, 1–4,000 people and 1–8,000 relationships.")
		return input, p
	}
	p.Counts = ImportCounts{Buildings: len(input.Buildings), Homes: len(input.Homes), People: len(input.People), Relationships: len(input.Relationships)}
	var raw map[string]json.RawMessage
	_ = json.Unmarshal([]byte(body), &raw)
	for section, fields := range map[string][]string{
		"buildings":     {"source_id", "code", "name"},
		"homes":         {"source_id", "building", "number", "floor", "occupancy"},
		"people":        {"source_id", "name"},
		"relationships": {"source_id", "home", "person", "relationship", "start_date", "end_date", "primary_contact"},
	} {
		var rows []map[string]json.RawMessage
		_ = json.Unmarshal(raw[section], &rows)
		for i, row := range rows {
			for _, field := range fields {
				if value, ok := row[field]; !ok || string(value) == "null" {
					issue(section, i+1, field, "Supply this required field explicitly; null is not a supplied value.")
				}
			}
		}
	}
	buildings := map[string]ImportBuilding{}
	homes := map[string]ImportHome{}
	people := map[string]ImportPerson{}
	relations := map[string]bool{}
	codes := map[string]bool{}
	numbers := map[string]bool{}
	ids := func(section string, row int, id string, exists bool) {
		if !sourceIdentity.MatchString(id) {
			issue(section, row, "source_id", "Use a stable source identity of 1–80 letters, digits, dots, underscores or hyphens.")
		}
		if exists {
			issue(section, row, "source_id", "This source identity is repeated in this section.")
		}
	}
	codePattern := regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]{0,11}$`)
	for i, b := range input.Buildings {
		_, exists := buildings[b.SourceID]
		ids("buildings", i+1, b.SourceID, exists)
		buildings[b.SourceID] = b
		if !codePattern.MatchString(b.Code) {
			issue("buildings", i+1, "code", "Use 1–12 uppercase letters, digits or hyphens.")
		}
		if codes[b.Code] {
			issue("buildings", i+1, "code", "This building code is repeated.")
		}
		codes[b.Code] = true
		if !validText(b.Name, 2, 80) {
			issue("buildings", i+1, "name", "Supply a name of 2–80 characters without surrounding whitespace or control characters.")
		}
	}
	buildingHomes := map[string]int{}
	for i, h := range input.Homes {
		_, exists := homes[h.SourceID]
		ids("homes", i+1, h.SourceID, exists)
		homes[h.SourceID] = h
		if _, ok := buildings[h.Building]; !ok {
			issue("homes", i+1, "building", "Reference a building source identity from this file.")
		}
		buildingHomes[h.Building]++
		if !validText(h.Number, 1, 20) {
			issue("homes", i+1, "number", "Supply a home number of 1–20 characters without surrounding whitespace or controls.")
		}
		key := h.Building + "\x00" + h.Number
		if numbers[key] {
			issue("homes", i+1, "number", "This home number is repeated within its building.")
		}
		numbers[key] = true
		if h.Floor < 0 || h.Floor > 100 {
			issue("homes", i+1, "floor", "Supply a floor from 0 to 100.")
		}
		switch h.Occupancy {
		case "VACANT":
			p.Counts.Vacant++
		case "OWNER_OCCUPIED", "RENTED":
			p.Counts.Occupied++
		default:
			issue("homes", i+1, "occupancy", "Choose OWNER_OCCUPIED, RENTED or VACANT.")
		}
	}
	for i, b := range input.Buildings {
		if buildingHomes[b.SourceID] == 0 {
			issue("buildings", i+1, "source_id", "Every supplied building must contain a home.")
		}
	}
	for i, person := range input.People {
		_, exists := people[person.SourceID]
		ids("people", i+1, person.SourceID, exists)
		people[person.SourceID] = person
		if !validText(person.Name, 2, 120) {
			issue("people", i+1, "name", "Supply a name of 2–120 characters without surrounding whitespace or controls.")
		}
	}
	owners := map[string]bool{}
	tenants := map[string]bool{}
	usedPeople := map[string]bool{}
	activeByHome := map[string]map[string]int{}
	members := map[string][]ImportReviewMember{}
	intervals := map[string][]ImportRelationship{}
	relationshipRows := map[string]int{}
	for i, r := range input.Relationships {
		ids("relationships", i+1, r.SourceID, relations[r.SourceID])
		relations[r.SourceID] = true
		relationshipRows[r.SourceID] = i + 1
		if _, ok := homes[r.Home]; !ok {
			issue("relationships", i+1, "home", "Reference a home source identity from this file.")
		}
		person, ok := people[r.Person]
		if !ok {
			issue("relationships", i+1, "person", "Reference a person source identity from this file.")
		}
		usedPeople[r.Person] = true
		if r.Relationship != "OWNER" && r.Relationship != "TENANT" && r.Relationship != "FAMILY" && r.Relationship != "AUTHORIZED_OCCUPANT" {
			issue("relationships", i+1, "relationship", "Choose OWNER, TENANT, FAMILY or AUTHORIZED_OCCUPANT.")
		}
		if !validDate(r.StartDate) {
			issue("relationships", i+1, "start_date", "Supply a real date from 1900 through today, YYYY-MM-DD.")
		}
		if r.EndDate != "" && (!validDate(r.EndDate) || r.EndDate <= r.StartDate) {
			issue("relationships", i+1, "end_date", "Use an empty current end date, or a date after the start and on or before today.")
		}
		intervalKey := r.Home + "\x00" + r.Person + "\x00" + r.Relationship
		intervals[intervalKey] = append(intervals[intervalKey], r)
		if r.EndDate == "" {
			if activeByHome[r.Home] == nil {
				activeByHome[r.Home] = map[string]int{}
			}
			activeByHome[r.Home][r.Relationship]++
			if r.PrimaryContact {
				activeByHome[r.Home]["PRIMARY"]++
			}
			if r.Relationship == "OWNER" {
				owners[r.Person] = true
			}
			if r.Relationship == "TENANT" {
				tenants[r.Person] = true
			}
		}
		members[r.Home] = append(members[r.Home], ImportReviewMember{r.SourceID, person.Name, r.Relationship, r.StartDate, r.EndDate, r.PrimaryContact})
	}
	for _, list := range intervals {
		sort.Slice(list, func(i, j int) bool { return list[i].StartDate < list[j].StartDate })
		for i := 1; i < len(list); i++ {
			if list[i-1].EndDate == "" || list[i-1].EndDate > list[i].StartDate {
				issue("relationships", relationshipRows[list[i].SourceID], "start_date", "A person's relationships to the same home and role overlap; check the supplied dates.")
				break
			}
		}
	}
	for i, person := range input.People {
		if !usedPeople[person.SourceID] {
			issue("people", i+1, "source_id", "Every person must have a supplied current or historical relationship.")
		}
	}
	for i, h := range input.Homes {
		active := activeByHome[h.SourceID]
		if active["OWNER"] == 0 {
			issue("homes", i+1, "occupancy", "Supply at least one current owner relationship for this home.")
		}
		if h.Occupancy == "RENTED" && active["TENANT"] == 0 {
			issue("homes", i+1, "occupancy", "A rented home needs a current tenant relationship.")
		}
		if h.Occupancy == "OWNER_OCCUPIED" && active["TENANT"] > 0 {
			issue("homes", i+1, "occupancy", "An owner-occupied home cannot have a current tenant relationship.")
		}
		if h.Occupancy == "VACANT" && (active["TENANT"]+active["FAMILY"]+active["AUTHORIZED_OCCUPANT"] > 0) {
			issue("homes", i+1, "occupancy", "A vacant home cannot have a current occupant relationship other than ownership.")
		}
		if active["PRIMARY"] > 1 {
			issue("homes", i+1, "primary_contact", "Choose at most one current primary contact for this home.")
		}
		if active["PRIMARY"] == 0 {
			warn("homes", i+1, "primary_contact", "No current primary contact is supplied for this home.")
		}
		p.Homes = append(p.Homes, ImportReviewHome{h.SourceID, buildings[h.Building].Code + "-" + h.Number, h.Floor, h.Occupancy, members[h.SourceID]})
	}
	p.Counts.Owners = len(owners)
	p.Counts.Tenants = len(tenants)
	return input, p
}

// Include complete registry rows and versions. A reviewed change, even one later
// reversed to the same occupancy, invalidates a previously prepared preview.
func registryDigest(ctx context.Context, q identityReader) (string, error) {
	frame := []any{today()}
	for _, query := range []string{
		"SELECT json_array(id,code,name) FROM buildings ORDER BY id",
		"SELECT json_array(id,building_id,flat_number,floor,status,version) FROM flats ORDER BY id",
		"SELECT json_array(id,full_name) FROM residents ORDER BY id",
		"SELECT json_array(id,flat_id,resident_id,relationship,start_date,end_date,is_primary_contact,can_view_finances) FROM flat_memberships ORDER BY id",
		"SELECT json_array(id,input_digest) FROM registry_imports ORDER BY id",
	} {
		rows, err := q.QueryContext(ctx, query)
		if err != nil {
			return "", err
		}
		values := []string{}
		for rows.Next() {
			var value string
			if err = rows.Scan(&value); err != nil {
				rows.Close()
				return "", err
			}
			values = append(values, value)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return "", err
		}
		frame = append(frame, values)
	}
	body, _ := json.Marshal(frame)
	return InputDigest(body), nil
}

func requireImportOfficer(ctx context.Context, q identityReader, token string, fresh bool) (Principal, error) {
	p, err := fullPrincipal(ctx, q, TokenHash(token), time.Now())
	if err != nil {
		return p, err
	}
	if !p.CanManageRegistry {
		return p, ErrForbidden
	}
	var configured bool
	if err = q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM workspace_setup)").Scan(&configured); err != nil {
		return p, err
	}
	if !configured {
		return p, ErrForbidden
	}
	if fresh {
		err = requireFresh(p)
	}
	return p, err
}

func importResult(ctx context.Context, q identityReader, where string, args ...any) (*RegistryImportResult, error) {
	var r RegistryImportResult
	var counts string
	err := q.QueryRowContext(ctx, "SELECT id,source_key,input_digest,effective_date,counts_json,created_at FROM registry_imports WHERE "+where+" ORDER BY created_at DESC,id LIMIT 1", args...).Scan(&r.ID, &r.SourceKey, &r.Digest, &r.EffectiveDate, &counts, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal([]byte(counts), &r.Counts); err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) RegistryImportStatus(ctx context.Context, token string) (RegistryImportStatus, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return RegistryImportStatus{}, err
	}
	defer tx.Rollback()
	if _, err = requireImportOfficer(ctx, tx, token, false); err != nil {
		return RegistryImportStatus{}, err
	}
	var result RegistryImportStatus
	if err = tx.QueryRowContext(ctx, "SELECT (SELECT COUNT(*) FROM buildings),(SELECT COUNT(*) FROM flats),(SELECT COUNT(*) FROM residents),(SELECT COUNT(*) FROM flat_memberships)").Scan(&result.RegistryCounts.Buildings, &result.RegistryCounts.Flats, &result.RegistryCounts.Residents, &result.RegistryCounts.Memberships); err != nil {
		return result, err
	}
	if err = tx.QueryRowContext(ctx, "SELECT mode,society_key FROM workspace_setup WHERE id=1").Scan(&result.Mode, &result.SocietyKey); err != nil {
		return result, err
	}
	result.Applied, err = importResult(ctx, tx, "1=1")
	if err != nil {
		return result, err
	}
	result.InitialImportAvailable = result.RegistryCounts == (Counts{}) && result.Applied == nil
	return result, tx.Commit()
}

func previewImport(ctx context.Context, q identityReader, body string) (RegistryImportInput, RegistryImportPreview, error) {
	input, p := validateRegistryImport(body)
	var err error
	p.BaseDigest, err = registryDigest(ctx, q)
	if err != nil {
		return input, p, err
	}
	if p.ErrorCount != 0 {
		return input, p, nil
	}
	var society string
	if err = q.QueryRowContext(ctx, "SELECT society_key FROM workspace_setup WHERE id=1").Scan(&society); err != nil {
		return input, p, err
	}
	if input.SocietyKey != society {
		p.Errors = append(p.Errors, ImportIssue{"file", 0, "society_key", "This register belongs to a different society key."})
		p.ErrorCount++
		return input, p, nil
	}
	prior, err := importResult(ctx, q, "source_key=?", input.SourceKey)
	if err != nil {
		return input, p, err
	}
	if prior != nil {
		if prior.Digest != p.Digest {
			p.Errors = append(p.Errors, ImportIssue{"file", 0, "source_key", "This source is already retained with different contents. Use reviewed registry changes."})
			p.ErrorCount++
		} else {
			p.AlreadyApplied = prior
		}
		return input, p, nil
	}
	var occupied int
	if err = q.QueryRowContext(ctx, "SELECT (SELECT COUNT(*) FROM buildings)+(SELECT COUNT(*) FROM flats)+(SELECT COUNT(*) FROM residents)+(SELECT COUNT(*) FROM flat_memberships)+(SELECT COUNT(*) FROM registry_imports)").Scan(&occupied); err != nil {
		return input, p, err
	}
	if occupied != 0 {
		p.Errors = append(p.Errors, ImportIssue{"file", 0, "registry", "Initial import requires an empty registry. Existing homes and reviewed changes are preserved."})
		p.ErrorCount++
		return input, p, nil
	}
	p.CanApply = true
	return input, p, nil
}

func (s *Store) PreviewRegistryImport(ctx context.Context, token, body string) (RegistryImportPreview, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return RegistryImportPreview{}, err
	}
	defer tx.Rollback()
	if _, err = requireImportOfficer(ctx, tx, token, false); err != nil {
		return RegistryImportPreview{}, err
	}
	_, p, err := previewImport(ctx, tx, body)
	if err != nil {
		return p, err
	}
	return p, tx.Commit()
}

func importedID(source, kind, id string) string {
	return "supplied-" + InputDigest([]byte(source+"\x00"+kind+"\x00"+id))
}

func (s *Store) ApplyRegistryImport(ctx context.Context, token string, in ApplyRegistryImport) (RegistryImportResult, error) {
	if !sourceIdentity.MatchString(in.OperationKey) || !in.Confirmed || !validText(in.Note, 10, 300) || len(in.Digest) != 64 || len(in.BaseDigest) != 64 {
		return RegistryImportResult{}, invalid("Confirm the exact preview and supply a stable operation key and verification note of 10–300 characters.")
	}
	if len(in.InputText) > RegistryImportMaxBytes || InputDigest([]byte(in.InputText)) != in.Digest {
		return RegistryImportResult{}, ErrConflict
	}
	tx, _, err := s.beginRegistryWrite(ctx, token)
	if err != nil {
		return RegistryImportResult{}, err
	}
	defer tx.Rollback()
	p, err := requireImportOfficer(ctx, tx, token, true)
	if err != nil {
		return RegistryImportResult{}, err
	}
	prior, err := importResult(ctx, tx, "operation_key=?", in.OperationKey)
	if err != nil {
		return RegistryImportResult{}, err
	}
	if prior != nil {
		var retainedRequest string
		if err = tx.QueryRowContext(ctx, "SELECT request_digest FROM registry_imports WHERE operation_key=?", in.OperationKey).Scan(&retainedRequest); err != nil {
			return RegistryImportResult{}, err
		}
		if prior.Digest != in.Digest || retainedRequest != importRequestDigest(in) {
			return RegistryImportResult{}, ErrConflict
		}
		return *prior, tx.Commit()
	}
	input, preview, err := previewImport(ctx, tx, in.InputText)
	if err != nil {
		return RegistryImportResult{}, err
	}
	if preview.AlreadyApplied != nil {
		return *preview.AlreadyApplied, tx.Commit()
	}
	if preview.BaseDigest != in.BaseDigest || preview.EffectiveDate != in.EffectiveDate {
		return RegistryImportResult{}, ErrConflict
	}
	if !preview.CanApply || preview.ErrorCount != 0 {
		return RegistryImportResult{}, invalid("This register cannot be applied. Validate it again and resolve the listed errors.")
	}
	result := RegistryImportResult{ID: randomToken(), SourceKey: input.SourceKey, Digest: in.Digest, EffectiveDate: preview.EffectiveDate, Counts: preview.Counts, CreatedAt: time.Now().Unix()}
	counts, _ := json.Marshal(result.Counts)
	if _, err = tx.ExecContext(ctx, `INSERT INTO registry_imports VALUES (?,?,?,?,?,?,?,?,?,?,?)`, result.ID, in.OperationKey, importRequestDigest(in), input.SourceKey, in.Digest, in.BaseDigest, result.EffectiveDate, string(counts), p.ID, in.Note, result.CreatedAt); err != nil {
		return result, err
	}
	retain := func(kind, source, id string) error {
		_, e := tx.ExecContext(ctx, "INSERT INTO registry_import_entities VALUES (?,?,?,?)", result.ID, kind, source, id)
		return e
	}
	id := func(kind, source string) string { return importedID(input.SourceKey, kind, source) }
	for _, b := range input.Buildings {
		key := id("BUILDING", b.SourceID)
		if _, err = tx.ExecContext(ctx, "INSERT INTO buildings VALUES (?,?,?)", key, b.Code, b.Name); err != nil {
			return result, err
		}
		if err = retain("BUILDING", b.SourceID, key); err != nil {
			return result, err
		}
	}
	for _, h := range input.Homes {
		key := id("HOME", h.SourceID)
		if _, err = tx.ExecContext(ctx, "INSERT INTO flats(id,building_id,flat_number,floor,status) VALUES (?,?,?,?,?)", key, id("BUILDING", h.Building), h.Number, h.Floor, h.Occupancy); err != nil {
			return result, err
		}
		if err = retain("HOME", h.SourceID, key); err != nil {
			return result, err
		}
	}
	for _, person := range input.People {
		key := id("PERSON", person.SourceID)
		if _, err = tx.ExecContext(ctx, "INSERT INTO residents VALUES (?,?)", key, person.Name); err != nil {
			return result, err
		}
		if err = retain("PERSON", person.SourceID, key); err != nil {
			return result, err
		}
	}
	for _, r := range input.Relationships {
		key := id("RELATIONSHIP", r.SourceID)
		var end any
		if r.EndDate != "" {
			end = r.EndDate
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO flat_memberships VALUES (?,?,?,?,?,?,?,0)", key, id("HOME", r.Home), id("PERSON", r.Person), r.Relationship, r.StartDate, end, r.PrimaryContact); err != nil {
			return result, err
		}
		if err = retain("RELATIONSHIP", r.SourceID, key); err != nil {
			return result, err
		}
	}
	if err = appendAudit(ctx, tx, p.ID, "", "REGISTRY_IMPORTED", in.Note, map[string]any{"base_digest": in.BaseDigest}, map[string]any{"import_id": result.ID, "source_key": result.SourceKey, "input_digest": result.Digest, "counts": result.Counts}); err != nil {
		return result, fmt.Errorf("retain registry import audit: %w", err)
	}
	return result, tx.Commit()
}

func importRequestDigest(in ApplyRegistryImport) string {
	body, _ := json.Marshal(struct {
		Digest, BaseDigest, EffectiveDate, OperationKey, Note string
		Confirmed                                             bool
	}{in.Digest, in.BaseDigest, in.EffectiveDate, in.OperationKey, in.Note, in.Confirmed})
	return InputDigest(body)
}
