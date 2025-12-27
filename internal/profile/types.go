package profile

// Info represents profile information for listing and display.
// Note: LoggedIn status is not part of profile info - check keyring separately.
type Info struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	Type      string `json:"type"`
	Namespace string `json:"namespace,omitempty"`
	Current   bool   `json:"current"`
}

// Status represents comprehensive profile status information.
type Status struct {
	Name          string `json:"name"`
	Address       string `json:"address"`
	Type          string `json:"type"`
	Namespace     string `json:"namespace,omitempty"`
	Binary        string `json:"binary"`
	BinaryPath    string `json:"binary_path,omitempty"`
	TLSSkipVerify bool   `json:"tls_skip_verify,omitempty"`
	CACert        string `json:"ca_cert,omitempty"`
	CAPath        string `json:"ca_path,omitempty"`
	ClientCert    string `json:"client_cert,omitempty"`
	ClientKey     string `json:"client_key,omitempty"`
	Active        bool   `json:"active"`
}
