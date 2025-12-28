package notify

// mockBackend is a mock implementation of Backend for testing.
type mockBackend struct {
	notifyFunc  func(title, message, iconPath string) error
	alertFunc   func(title, message, iconPath string) error
	notifyCalls []notifyCall
	alertCalls  []notifyCall
}

type notifyCall struct {
	title    string
	message  string
	iconPath string
}

// Notify implements Backend.
func (m *mockBackend) Notify(title, message, iconPath string) error {
	m.notifyCalls = append(m.notifyCalls, notifyCall{title, message, iconPath})
	if m.notifyFunc != nil {
		return m.notifyFunc(title, message, iconPath)
	}
	return nil
}

// Alert implements Backend.
func (m *mockBackend) Alert(title, message, iconPath string) error {
	m.alertCalls = append(m.alertCalls, notifyCall{title, message, iconPath})
	if m.alertFunc != nil {
		return m.alertFunc(title, message, iconPath)
	}
	return nil
}
