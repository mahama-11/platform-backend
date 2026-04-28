package migration

import "gorm.io/gorm"

func refreshMenuOfferingsLandingCopy(db *gorm.DB) error {
	return seedMenuOfferings(db)
}
