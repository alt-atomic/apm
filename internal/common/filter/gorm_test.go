package filter

import (
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type dryRunRow struct {
	Name        string
	Description string
	Section     string
	Size        int
}

func (dryRunRow) TableName() string { return "rows" }

func newDryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return db
}

func capturedSQL(t *testing.T, q *gorm.DB) string {
	t.Helper()
	return q.Find(&[]dryRunRow{}).Statement.SQL.String()
}

func TestApplyGroups_NoGroups(t *testing.T) {
	db := newDryRunDB(t)
	applier := &GormApplier{}
	sql := capturedSQL(t, applier.ApplyGroups(db.Model(&dryRunRow{}), nil))
	if strings.Contains(strings.ToUpper(sql), "WHERE") {
		t.Errorf("expected no WHERE clause for empty groups, got: %s", sql)
	}
}

func TestApplyGroups_SingleFilterGroupEqualsApply(t *testing.T) {
	applier := &GormApplier{}
	f := Filter{Field: "name", Op: OpEq, Value: "zip"}

	sqlGroups := capturedSQL(t, applier.ApplyGroups(newDryRunDB(t).Model(&dryRunRow{}), []FilterGroup{{f}}))
	sqlFlat := capturedSQL(t, applier.Apply(newDryRunDB(t).Model(&dryRunRow{}), []Filter{f}))

	if sqlGroups != sqlFlat {
		t.Errorf("expected single-filter group to match Apply.\nApplyGroups: %s\nApply:       %s", sqlGroups, sqlFlat)
	}
}

func TestApplyGroups_GroupIsORed(t *testing.T) {
	db := newDryRunDB(t)
	applier := &GormApplier{}

	groups := []FilterGroup{{
		{Field: "name", Op: OpLike, Value: "office"},
		{Field: "size", Op: OpGt, Value: "1000"},
	}}

	sql := capturedSQL(t, applier.ApplyGroups(db.Model(&dryRunRow{}), groups))
	upper := strings.ToUpper(sql)

	if !strings.Contains(upper, " OR ") {
		t.Errorf("expected OR inside group, got: %s", sql)
	}
	if !strings.Contains(sql, "`name`") || !strings.Contains(sql, "`size`") {
		t.Errorf("expected both fields referenced, got: %s", sql)
	}
}

func TestApplyGroups_GroupsAreANDed(t *testing.T) {
	db := newDryRunDB(t)
	applier := &GormApplier{}

	groups := MakeGroups(
		[]Filter{{Field: "section", Op: OpEq, Value: "games"}},
		[]Filter{
			{Field: "name", Op: OpLike, Value: "office"},
			{Field: "description", Op: OpLike, Value: "office"},
		},
	)

	sql := capturedSQL(t, applier.ApplyGroups(db.Model(&dryRunRow{}), groups))
	upper := strings.ToUpper(sql)

	if !strings.Contains(upper, " AND ") {
		t.Errorf("expected AND between groups, got: %s", sql)
	}
	if !strings.Contains(upper, " OR ") {
		t.Errorf("expected OR inside orFilters group, got: %s", sql)
	}
	// OR-скобка не должна протекать: AND-условие вне скобок
	orPart := sql[strings.Index(upper, " OR "):]
	if strings.Contains(orPart, "`section`") {
		t.Errorf("expected section condition outside OR bracket, got: %s", sql)
	}
}

func TestApply_MultiFieldIsORed(t *testing.T) {
	db := newDryRunDB(t)
	applier := &GormApplier{}

	sql := capturedSQL(t, applier.Apply(db.Model(&dryRunRow{}), []Filter{
		{Field: "name|description", Op: OpLike, Value: "office"},
	}))
	upper := strings.ToUpper(sql)

	if !strings.Contains(upper, " OR ") {
		t.Errorf("expected OR between fields, got: %s", sql)
	}
	if !strings.Contains(sql, "`name`") || !strings.Contains(sql, "`description`") {
		t.Errorf("expected both fields referenced, got: %s", sql)
	}
}

func TestApply_MultiFieldAndValuesCrossProduct(t *testing.T) {
	db := newDryRunDB(t)
	applier := &GormApplier{}

	sql := capturedSQL(t, applier.Apply(db.Model(&dryRunRow{}), []Filter{
		{Field: "name|description", Op: OpLike, Value: "zip|rar"},
	}))

	if got := strings.Count(strings.ToUpper(sql), " OR "); got != 3 {
		t.Errorf("expected 4 ORed conditions (2 fields x 2 values), got %d OR in: %s", got+1, sql)
	}
}

func TestApplyGroups_ValuePipeInsideGroup(t *testing.T) {
	db := newDryRunDB(t)
	applier := &GormApplier{}

	groups := []FilterGroup{{
		{Field: "name", Op: OpLike, Value: "zip|rar"},
		{Field: "size", Op: OpGt, Value: "1000"},
	}}

	sql := capturedSQL(t, applier.ApplyGroups(db.Model(&dryRunRow{}), groups))

	if got := strings.Count(strings.ToUpper(sql), " OR "); got != 2 {
		t.Errorf("expected 3 ORed conditions (2 values + size), got %d OR in: %s", got+1, sql)
	}
}

func TestApplyGroups_ExecutesAgainstSQLite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err = db.AutoMigrate(&dryRunRow{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	rows := []dryRunRow{
		{Name: "libreoffice", Description: "editor", Section: "office", Size: 100},
		{Name: "zip", Description: "office archiver", Section: "archive", Size: 10},
		{Name: "vim", Description: "editor", Section: "editors", Size: 5},
		{Name: "mc", Description: "file manager", Section: "shells", Size: 2000},
	}
	if err = db.Create(&rows).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	applier := &GormApplier{}
	query := func(groups []FilterGroup) []string {
		var got []dryRunRow
		if err := applier.ApplyGroups(db.Model(&dryRunRow{}), groups).Order("name").Find(&got).Error; err != nil {
			t.Fatalf("query: %v", err)
		}
		names := make([]string, 0, len(got))
		for _, r := range got {
			names = append(names, r.Name)
		}
		return names
	}

	// (name~office OR description~office) AND size < 1000
	names := query(MakeGroups(
		[]Filter{{Field: "size", Op: OpLt, Value: "1000"}},
		[]Filter{
			{Field: "name", Op: OpLike, Value: "office"},
			{Field: "description", Op: OpLike, Value: "office"},
		},
	))
	if strings.Join(names, ",") != "libreoffice,zip" {
		t.Errorf("expected [libreoffice zip], got %v", names)
	}

	// чистый OR по разным полям и операторам: name=zip OR size>1000
	names = query(MakeGroups(nil, []Filter{
		{Field: "name", Op: OpEq, Value: "zip"},
		{Field: "size", Op: OpGt, Value: "1000"},
	}))
	if strings.Join(names, ",") != "mc,zip" {
		t.Errorf("expected [mc zip], got %v", names)
	}

	// мульти-поле: name|description ~ editor
	names = query(AndGroups([]Filter{{Field: "name|description", Op: OpLike, Value: "editor"}}))
	if strings.Join(names, ",") != "libreoffice,vim" {
		t.Errorf("expected [libreoffice vim], got %v", names)
	}
}

func TestApplyGroups_CustomApplierInsideGroup(t *testing.T) {
	db := newDryRunDB(t)
	applier := &GormApplier{
		CustomAppliers: map[string]FieldApplier{
			"section": func(q *gorm.DB, f Filter) (*gorm.DB, bool) {
				return q.Where("custom_section = ?", f.Value), true
			},
		},
	}

	groups := []FilterGroup{{
		{Field: "section", Op: OpEq, Value: "games"},
		{Field: "name", Op: OpEq, Value: "zip"},
	}}

	sql := capturedSQL(t, applier.ApplyGroups(db.Model(&dryRunRow{}), groups))

	if !strings.Contains(sql, "custom_section") {
		t.Errorf("expected custom applier used inside group, got: %s", sql)
	}
	if !strings.Contains(strings.ToUpper(sql), " OR ") {
		t.Errorf("expected OR inside group, got: %s", sql)
	}
}
