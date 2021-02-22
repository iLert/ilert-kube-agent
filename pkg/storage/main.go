package storage

// Storage prometheus metrics storage
type Storage struct {
	incidentsCreatedCount float64
}

// Init initialize storage
func (storage *Storage) Init() {
	storage.incidentsCreatedCount = 0
}

// GetIncidentsCreatedCount returns created incidents count
func (storage *Storage) GetIncidentsCreatedCount() float64 {
	return storage.incidentsCreatedCount
}

// IncreaseIncidentsCreatedCount increases created incidents count
func (storage *Storage) IncreaseIncidentsCreatedCount() {
	storage.incidentsCreatedCount++
}
