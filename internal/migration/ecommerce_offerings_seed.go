package migration

import (
	commercial "platform-service/internal/modules/commercial"

	"gorm.io/gorm"
)

func seedEcommerceOfferings(db *gorm.DB) error {
	return commercial.SeedEcommerceVisibleBaseline(db)
}
