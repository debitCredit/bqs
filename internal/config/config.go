package config

import "time"

// Cache TTL configuration
const (
	TableListTTL = 5 * time.Minute  // Table lists change infrequently
	MetadataTTL  = 15 * time.Minute // Table metadata changes moderately  
	SchemaTTL    = 30 * time.Minute // Schemas change rarely
)

// UI configuration
const (
	DefaultTableHeight = 20
	MinTableHeight     = 5
	
	// Table column widths
	CacheColumnWidth   = 5
	TableColumnWidth   = 35
	TypeColumnWidth    = 8
	CreatedColumnWidth = 20
	
	// UI spacing and timing
	HeaderFooterPadding = 8  // Account for header, footer, padding in table height
	StatusMessageTTL    = 3 * time.Second // How long status messages are shown
	
	// UI styling constants  
	ModalBackgroundColor = "237" // Dark background for modals/overlays
)

// Default cache initialization TTL
const DefaultCacheTTL = MetadataTTL