package main

import (
	"testing"
)

// MockMQTT implements MQTTPublisher for testing
type MockMQTT struct {
	published     []PublishedMessage
	jsonPublished []PublishedJSON
	subscriptions []string
}

type PublishedMessage struct {
	Topic string
	Value any
}

type PublishedJSON struct {
	Topic string
	Data  any
}

func NewMockMQTT() *MockMQTT {
	return &MockMQTT{
		published:     []PublishedMessage{},
		jsonPublished: []PublishedJSON{},
		subscriptions: []string{},
	}
}

func (m *MockMQTT) Publish(topic string, value any) error {
	m.published = append(m.published, PublishedMessage{Topic: topic, Value: value})
	return nil
}

func (m *MockMQTT) PublishJSON(topic string, data any) error {
	m.jsonPublished = append(m.jsonPublished, PublishedJSON{Topic: topic, Data: data})
	return nil
}

func (m *MockMQTT) Subscribe(topic string, handler func(string, []byte)) error {
	m.subscriptions = append(m.subscriptions, topic)
	return nil
}

// MockGekko implements GekkoClient for testing
type MockGekko struct {
	name        string
	status      map[string]any
	definitions map[string]any
	setValue    func(category, item, value string) error
}

func NewMockGekko(name string) *MockGekko {
	return &MockGekko{
		name:        name,
		status:      make(map[string]any),
		definitions: make(map[string]any),
	}
}

func (m *MockGekko) GetGekkoName() (string, error) {
	return m.name, nil
}

func (m *MockGekko) GetStatus(categories []string) (map[string]any, error) {
	return m.status, nil
}

func (m *MockGekko) SetValue(category, item, value string) error {
	if m.setValue != nil {
		return m.setValue(category, item, value)
	}
	return nil
}

func (m *MockGekko) GetDefinitions() (map[string]any, error) {
	return m.definitions, nil
}

func TestParseFormatField_Int(t *testing.T) {
	field, err := parseFormatField("currentState int[0,1,2]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if field.Name != "currentState" {
		t.Errorf("expected name 'currentState', got '%s'", field.Name)
	}
	if field.Type != "int" {
		t.Errorf("expected type 'int', got '%s'", field.Type)
	}
}

func TestParseFormatField_Enum(t *testing.T) {
	field, err := parseFormatField("status enum[off,on,auto]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if field.Name != "status" {
		t.Errorf("expected name 'status', got '%s'", field.Name)
	}
	if field.Type != "int" {
		t.Errorf("expected type 'int' for enum, got '%s'", field.Type)
	}
}

func TestParseFormatField_Float(t *testing.T) {
	field, err := parseFormatField("temperature float[0.0:100.0]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if field.Name != "temperature" {
		t.Errorf("expected name 'temperature', got '%s'", field.Name)
	}
	if field.Type != "float" {
		t.Errorf("expected type 'float', got '%s'", field.Type)
	}
}

func TestParseFormatField_String(t *testing.T) {
	field, err := parseFormatField("name string[max:100]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if field.Name != "name" {
		t.Errorf("expected name 'name', got '%s'", field.Name)
	}
	if field.Type != "string" {
		t.Errorf("expected type 'string', got '%s'", field.Type)
	}
}

func TestParseFormatField_Null(t *testing.T) {
	field, err := parseFormatField("reserved null[]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if field.Name != "reserved" {
		t.Errorf("expected name 'reserved', got '%s'", field.Name)
	}
	if field.Type != "" {
		t.Errorf("expected empty type for null, got '%s'", field.Type)
	}
}

func TestParseFormatField_WithPrefix(t *testing.T) {
	// Real example from MyGEKKO API: "#zimmermann:enum[...]"
	field, err := parseFormatField("mode #zimmermann:enum[off,on,auto]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if field.Name != "mode" {
		t.Errorf("expected name 'mode', got '%s'", field.Name)
	}
	if field.Type != "int" {
		t.Errorf("expected type 'int' for prefixed enum, got '%s'", field.Type)
	}
}

func TestParseFormatField_ColonInBrackets(t *testing.T) {
	// Real example: "energyUnit string[xh:x=kW,ml,l3,...](I.S.)"
	field, err := parseFormatField("energyUnit string[xh:x=kW,ml,l3]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if field.Name != "energyUnit" {
		t.Errorf("expected name 'energyUnit', got '%s'", field.Name)
	}
	if field.Type != "string" {
		t.Errorf("expected type 'string', got '%s'", field.Type)
	}
}

func TestParseFormatField_Empty(t *testing.T) {
	field, err := parseFormatField("")
	if err != nil {
		t.Fatalf("unexpected error for empty string: %v", err)
	}
	if field.Name != "" || field.Type != "" {
		t.Errorf("expected empty field for empty string, got %+v", field)
	}
}

func TestParseFormatField_Whitespace(t *testing.T) {
	field, err := parseFormatField("   ")
	if err != nil {
		t.Fatalf("unexpected error for whitespace: %v", err)
	}
	if field.Name != "" || field.Type != "" {
		t.Errorf("expected empty field for whitespace, got %+v", field)
	}
}

func TestParseFormatField_InvalidFormat(t *testing.T) {
	_, err := parseFormatField("noTypeHere")
	if err == nil {
		t.Error("expected error for invalid format (no space)")
	}
}

func TestParseFormatField_NoBracket(t *testing.T) {
	_, err := parseFormatField("name int")
	if err == nil {
		t.Error("expected error for missing bracket")
	}
}

func TestParseFormatField_UnsupportedType(t *testing.T) {
	_, err := parseFormatField("data blob[binary]")
	if err == nil {
		t.Error("expected error for unsupported type")
	}
}

func TestParseFormatField_ComplexBracketContent(t *testing.T) {
	// Test with complex content inside brackets
	field, err := parseFormatField("value float[-100.0:100.0](unit:°C)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if field.Type != "float" {
		t.Errorf("expected type 'float', got '%s'", field.Type)
	}
}

// Integration tests using mocks

func TestNewBridge(t *testing.T) {
	cfg := &Config{
		MyGekko: MyGekkoConfig{
			Host:           "test.example.com",
			Username:       "user",
			Password:       "pass",
			Interval:       5.0,
			IntervalRounds: 4,
			IntervalItems:  []string{"blinds"},
		},
		MQTT: MQTTConfig{
			URL:  "tcp://mqtt.example.com:1883",
			Root: "test",
		},
	}

	mockGekko := NewMockGekko("TestGekko")
	mockMQTT := NewMockMQTT()
	fieldDefs := map[string][]FieldDef{
		"blinds": {{Name: "position", Type: "int"}},
	}

	bridge, err := NewBridge(cfg, mockGekko, mockMQTT, fieldDefs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bridge.gekkoName != "TestGekko" {
		t.Errorf("expected gekkoName 'TestGekko', got '%s'", bridge.gekkoName)
	}
}

func TestProcessItem_PublishesValues(t *testing.T) {
	cfg := &Config{}
	mockGekko := NewMockGekko("TestGekko")
	mockMQTT := NewMockMQTT()
	fieldDefs := map[string][]FieldDef{
		"blinds": {
			{Name: "position", Type: "int"},
			{Name: "angle", Type: "float"},
		},
	}

	bridge, err := NewBridge(cfg, mockGekko, mockMQTT, fieldDefs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Simulate sumstate data
	sumstate := map[string]any{
		"value": "50;45.5",
	}

	bridge.processItem("blinds", "item0", sumstate)

	// Check individual field publishes
	if len(mockMQTT.published) != 2 {
		t.Errorf("expected 2 published messages, got %d", len(mockMQTT.published))
	}

	// Check JSON publish
	if len(mockMQTT.jsonPublished) != 1 {
		t.Errorf("expected 1 JSON published, got %d", len(mockMQTT.jsonPublished))
	}

	// Verify topics
	expectedTopic1 := "TestGekko/blinds/item0/get/position"
	expectedTopic2 := "TestGekko/blinds/item0/get/angle"
	found1, found2 := false, false
	for _, msg := range mockMQTT.published {
		if msg.Topic == expectedTopic1 {
			found1 = true
			if msg.Value != 50 {
				t.Errorf("expected position value 50, got %v", msg.Value)
			}
		}
		if msg.Topic == expectedTopic2 {
			found2 = true
			if msg.Value != 45.5 {
				t.Errorf("expected angle value 45.5, got %v", msg.Value)
			}
		}
	}
	if !found1 {
		t.Errorf("position topic not found in published messages")
	}
	if !found2 {
		t.Errorf("angle topic not found in published messages")
	}
}

func TestProcessItem_HistoryDeduplication(t *testing.T) {
	cfg := &Config{}
	mockGekko := NewMockGekko("TestGekko")
	mockMQTT := NewMockMQTT()
	fieldDefs := map[string][]FieldDef{
		"blinds": {{Name: "position", Type: "int"}},
	}

	bridge, err := NewBridge(cfg, mockGekko, mockMQTT, fieldDefs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sumstate := map[string]any{
		"value": "50",
	}

	// First call should publish
	bridge.processItem("blinds", "item0", sumstate)
	if len(mockMQTT.published) != 1 {
		t.Errorf("first call: expected 1 published, got %d", len(mockMQTT.published))
	}

	// Second call with same value should NOT publish
	bridge.processItem("blinds", "item0", sumstate)
	if len(mockMQTT.published) != 1 {
		t.Errorf("second call: expected still 1 published (no duplicate), got %d", len(mockMQTT.published))
	}

	// Third call with different value should publish
	sumstate["value"] = "75"
	bridge.processItem("blinds", "item0", sumstate)
	if len(mockMQTT.published) != 2 {
		t.Errorf("third call: expected 2 published, got %d", len(mockMQTT.published))
	}
}

func TestProcessItem_JSONContainsTimestamp(t *testing.T) {
	cfg := &Config{}
	mockGekko := NewMockGekko("TestGekko")
	mockMQTT := NewMockMQTT()
	fieldDefs := map[string][]FieldDef{
		"blinds": {{Name: "position", Type: "int"}},
	}

	bridge, err := NewBridge(cfg, mockGekko, mockMQTT, fieldDefs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sumstate := map[string]any{
		"value": "50",
	}

	bridge.processItem("blinds", "item0", sumstate)

	if len(mockMQTT.jsonPublished) != 1 {
		t.Fatalf("expected 1 JSON published, got %d", len(mockMQTT.jsonPublished))
	}

	jsonData, ok := mockMQTT.jsonPublished[0].Data.(map[string]any)
	if !ok {
		t.Fatalf("expected JSON data to be map[string]any")
	}

	if _, exists := jsonData["timestamp"]; !exists {
		t.Error("JSON data should contain timestamp")
	}

	if _, exists := jsonData["position"]; !exists {
		t.Error("JSON data should contain position field")
	}
}

func TestProcessItem_SkipsEmptyValues(t *testing.T) {
	cfg := &Config{}
	mockGekko := NewMockGekko("TestGekko")
	mockMQTT := NewMockMQTT()
	fieldDefs := map[string][]FieldDef{
		"blinds": {
			{Name: "position", Type: "int"},
			{Name: "angle", Type: "float"},
		},
	}

	bridge, err := NewBridge(cfg, mockGekko, mockMQTT, fieldDefs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second value is empty
	sumstate := map[string]any{
		"value": "50;",
	}

	bridge.processItem("blinds", "item0", sumstate)

	// Should only publish position, not angle
	if len(mockMQTT.published) != 1 {
		t.Errorf("expected 1 published (empty value skipped), got %d", len(mockMQTT.published))
	}
}

func TestProcessItem_SkipsNullFields(t *testing.T) {
	cfg := &Config{}
	mockGekko := NewMockGekko("TestGekko")
	mockMQTT := NewMockMQTT()
	fieldDefs := map[string][]FieldDef{
		"blinds": {
			{Name: "position", Type: "int"},
			{Name: "", Type: ""},        // null/reserved field
			{Name: "angle", Type: "float"},
		},
	}

	bridge, err := NewBridge(cfg, mockGekko, mockMQTT, fieldDefs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sumstate := map[string]any{
		"value": "50;reserved;45.5",
	}

	bridge.processItem("blinds", "item0", sumstate)

	// Should publish position and angle, skip reserved
	if len(mockMQTT.published) != 2 {
		t.Errorf("expected 2 published (reserved skipped), got %d", len(mockMQTT.published))
	}
}

func TestProcessItem_UnknownCategory(t *testing.T) {
	cfg := &Config{}
	mockGekko := NewMockGekko("TestGekko")
	mockMQTT := NewMockMQTT()
	fieldDefs := map[string][]FieldDef{
		"blinds": {{Name: "position", Type: "int"}},
	}

	bridge, err := NewBridge(cfg, mockGekko, mockMQTT, fieldDefs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sumstate := map[string]any{
		"value": "50",
	}

	// Process unknown category - should not panic, just skip
	bridge.processItem("unknown", "item0", sumstate)

	if len(mockMQTT.published) != 0 {
		t.Errorf("expected 0 published for unknown category, got %d", len(mockMQTT.published))
	}
}

func TestHandleSetCommand(t *testing.T) {
	cfg := &Config{}
	mockMQTT := NewMockMQTT()

	var capturedCategory, capturedItem, capturedValue string
	mockGekko := NewMockGekko("TestGekko")
	mockGekko.setValue = func(category, item, value string) error {
		capturedCategory = category
		capturedItem = item
		capturedValue = value
		return nil
	}

	fieldDefs := map[string][]FieldDef{}

	bridge, err := NewBridge(cfg, mockGekko, mockMQTT, fieldDefs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Simulate incoming MQTT message
	bridge.handleSetCommand("root/blinds/item0/set", []byte("P50"))

	if capturedCategory != "blinds" {
		t.Errorf("expected category 'blinds', got '%s'", capturedCategory)
	}
	if capturedItem != "item0" {
		t.Errorf("expected item 'item0', got '%s'", capturedItem)
	}
	if capturedValue != "P50" {
		t.Errorf("expected value 'P50', got '%s'", capturedValue)
	}
}
