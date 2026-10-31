package migrations

import (
	"fmt"

	"gorm.io/gorm"
)

// A nullable staging column permits upgrading a populated table before imposing NOT NULL.
// Do not add default:true to the live bool models: GORM could then replace explicit false on insert.
type nullableActive struct {
	Active *bool
}

func backfillActive(tx *gorm.DB, table string) error {
	if !tx.Migrator().HasTable(table) {
		return nil
	}
	if !tx.Migrator().HasColumn(table, "active") {
		if err := tx.Table(table).Migrator().AddColumn(&nullableActive{}, "Active"); err != nil {
			return fmt.Errorf("add %s.active staging column: %w", table, err)
		}
	}
	var value any = false
	if tx.Migrator().HasColumn(table, "is_active") {
		value = gorm.Expr("COALESCE(is_active, ?)", false)
	}
	// Table rather than Model includes soft-deleted rows, which must also satisfy the constraint.
	// Existing true/false values are authoritative; unknown states fail closed.
	if err := tx.Table(table).Where("active IS NULL").UpdateColumn("active", value).Error; err != nil {
		return fmt.Errorf("backfill %s.active: %w", table, err)
	}
	return nil
}

func enforceActiveNotNull(tx *gorm.DB, model any) error {
	if !tx.Migrator().HasTable(model) {
		return nil
	}
	columns, err := tx.Migrator().ColumnTypes(model)
	if err != nil {
		return fmt.Errorf("inspect auth active constraint: %w", err)
	}
	for _, column := range columns {
		if column.Name() != "active" {
			continue
		}
		if nullable, known := column.Nullable(); !known || nullable {
			if err := tx.Migrator().AlterColumn(model, "Active"); err != nil {
				return fmt.Errorf("enforce auth active constraint: %w", err)
			}
		}
		return nil
	}
	return fmt.Errorf("auth active column missing after backfill")
}
