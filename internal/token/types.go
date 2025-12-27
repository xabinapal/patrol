package token

// Status represents the status of a token.
type Status struct {
	Stored      bool     `json:"stored"`
	Valid       bool     `json:"valid"`
	DisplayName string   `json:"display_name,omitempty"`
	TTL         int      `json:"ttl,omitempty"`
	Renewable   bool     `json:"renewable,omitempty"`
	Policies    []string `json:"policies,omitempty"`
	AuthPath    string   `json:"auth_path,omitempty"`
	EntityID    string   `json:"entity_id,omitempty"`
	Accessor    string   `json:"accessor,omitempty"`
	Error       string   `json:"error,omitempty"`
}
