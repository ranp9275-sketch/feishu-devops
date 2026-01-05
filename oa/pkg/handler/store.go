package oa

import (
	"devops/feishu/config"
	log "devops/tools/logger"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

var (
	Logger = log.NewLogger("ERROR")
)

// OARequest defines the GORM model for OA requests
type OARequest struct {
	ID        string    `gorm:"primaryKey;size:64"`
	Data      string    `gorm:"type:longtext"` // Stores the JSON content
	Processed bool      `gorm:"column:processed;default:false"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (OARequest) TableName() string {
	return "oa_requests"
}

// Helper to get DB connection
func getDB() (*gorm.DB, error) {
	c, err := config.LoadConfig()
	if err != nil {
		Logger.Error("Failed to load config: %v", err)
		return nil, err
	}
	db := c.GetDB()
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}
	return db, nil
}

// ensureTableExists ensures the table exists in the database
func ensureTableExists(db *gorm.DB) {
	if !db.Migrator().HasTable(&OARequest{}) {
		db.AutoMigrate(&OARequest{})
	}
}

// SaveToDB saves request data to the database.
func SaveToDB(id string, req interface{}) error {
	db, err := getDB()
	if err != nil {
		return err
	}
	ensureTableExists(db)

	// Marshal data to JSON string
	jsonBytes, err := json.Marshal(req)
	if err != nil {
		Logger.Error("Failed to marshal request data: %v", err)
		return err
	}

	oaReq := OARequest{
		ID:        id,
		Data:      string(jsonBytes),
		Processed: false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Use Save to update if exists (though ID is usually unique per request)
	if result := db.Save(&oaReq); result.Error != nil {
		Logger.Error("Failed to save request data to DB: %v", result.Error)
		return result.Error
	}

	return nil
}

// LoadFromDB loads request data from DB.
func LoadFromDB(id string) (map[string]interface{}, error) {
	db, err := getDB()
	if err != nil {
		return nil, err
	}
	ensureTableExists(db)

	var oaReq OARequest
	if result := db.First(&oaReq, "id = ?", id); result.Error != nil {
		Logger.Error("Failed to load request data from DB for id %s: %v", id, result.Error)
		return nil, result.Error
	}

	var req map[string]interface{}
	if err := json.Unmarshal([]byte(oaReq.Data), &req); err != nil {
		Logger.Error("Failed to unmarshal request data for id %s: %v", id, err)
		return nil, err
	}

	return req, nil
}

// LoadAllJsonFromDB loads all request data from DB.
func LoadAllJsonFromDB() ([]map[string]interface{}, error) {
	db, err := getDB()
	if err != nil {
		return nil, err
	}
	ensureTableExists(db)

	var oaReqs []OARequest
	if result := db.Order("created_at desc").Find(&oaReqs); result.Error != nil {
		Logger.Error("Failed to load all requests from DB: %v", result.Error)
		return nil, result.Error
	}

	var reqs []map[string]interface{}
	for _, oaReq := range oaReqs {
		var req map[string]interface{}
		if err := json.Unmarshal([]byte(oaReq.Data), &req); err == nil {
			req["id"] = oaReq.ID
			req["processed"] = oaReq.Processed
			reqs = append(reqs, req)
		}
	}
	return reqs, nil
}

// GetLatestJsonFromDB gets the latest request data from DB.
func GetLatestJsonFromDB() (map[string]interface{}, error) {
	db, err := getDB()
	if err != nil {
		return nil, err
	}
	ensureTableExists(db)

	var oaReq OARequest
	// Get latest by CreatedAt
	if result := db.Order("created_at desc").First(&oaReq); result.Error != nil {
		Logger.Error("Failed to get latest request from DB: %v", result.Error)
		return nil, result.Error
	}

	var req map[string]interface{}
	if err := json.Unmarshal([]byte(oaReq.Data), &req); err != nil {
		return nil, err
	}
	return req, nil
}

// GetUnprocessedRequestsFromDB gets all unprocessed request data from DB.
func GetUnprocessedRequestsFromDB() ([]map[string]interface{}, error) {
	db, err := getDB()
	if err != nil {
		return nil, err
	}
	ensureTableExists(db)

	var oaReqs []OARequest
	if result := db.Where("processed = ?", false).Order("created_at asc").Find(&oaReqs); result.Error != nil {
		Logger.Error("Failed to load unprocessed requests from DB: %v", result.Error)
		return nil, result.Error
	}

	var reqs []map[string]interface{}
	for _, oaReq := range oaReqs {
		var req map[string]interface{}
		if err := json.Unmarshal([]byte(oaReq.Data), &req); err == nil {
			// Ensure ID is present in the map, just in case
			req["id"] = oaReq.ID
			reqs = append(reqs, req)
		}
	}
	return reqs, nil
}

// MarkRequestAsProcessed marks a request as processed in DB.
func MarkRequestAsProcessed(id string) error {
	db, err := getDB()
	if err != nil {
		return err
	}
	ensureTableExists(db)

	if result := db.Model(&OARequest{}).Where("id = ?", id).Update("processed", true); result.Error != nil {
		Logger.Error("Failed to mark request %s as processed: %v", id, result.Error)
		return result.Error
	} else if result.RowsAffected == 0 {
		Logger.Error("MarkRequestAsProcessed: No rows affected for ID %s. ID mismatch?", id)
	} else {
		Logger.Info("MarkRequestAsProcessed: Successfully updated processed status for ID %s", id)
	}
	return nil
}
