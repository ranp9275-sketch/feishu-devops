package oa

type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type StoredJSON struct {
	ID           string                 `json:"id"`
	ReceivedAt   string                 `json:"received_at"`
	IPAddress    string                 `json:"ip_address"`
	UserAgent    string                 `json:"user_agent"`
	OriginalData map[string]interface{} `json:"original_data"`
}
