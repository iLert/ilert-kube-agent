package storage

// Storage prometheus metrics storage
type Storage struct {
	alertsCreatedCount float64
}

// Init initialize storage
func (storage *Storage) Init() {
	storage.alertsCreatedCount = 0
}

// GetAlertsCreatedCount returns created alerts count
func (storage *Storage) GetAlertsCreatedCount() float64 {
	return storage.alertsCreatedCount
}

// IncreaseAlertsCreatedCount increases created alerts count
func (storage *Storage) IncreaseAlertsCreatedCount() {
	storage.alertsCreatedCount++
}
