package migration

import "gorm.io/gorm"

func refreshMenuPaymentCashAsset(db *gorm.DB) error {
	return seedMenuOfferings(db)
}
