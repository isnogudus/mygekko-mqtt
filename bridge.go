package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

// FieldDef defines a field name and its type for parsing status values
type FieldDef struct {
	Name string
	Type string // "int", "float", "string", or "" to skip
}

// MQTTPublisher defines the interface for MQTT operations
type MQTTPublisher interface {
	Publish(topic string, value any) error
	PublishJSON(topic string, data any) error
	Subscribe(topic string, handler func(topic string, payload []byte)) error
}

// GekkoClient defines the interface for MyGEKKO API operations
type GekkoClient interface {
	GetStatus(categories []string) (map[string]any, error)
	SetValue(category, item, value string) error
	GetGekkoName() (string, error)
	GetDefinitions() (map[string]any, error)
}

type Bridge struct {
	cfg       *Config
	gekko     GekkoClient
	mqtt      MQTTPublisher
	fieldDef  map[string][]FieldDef
	gekkoName string
	history   map[string]any
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewBridge(cfg *Config, gekko GekkoClient, mqtt MQTTPublisher, fieldDefinitions map[string][]FieldDef) (*Bridge, error) {
	ctx, cancel := context.WithCancel(context.Background())

	gekkoName, err := gekko.GetGekkoName()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to get gekko name: %w", err)
	}
	slog.Info("Gekko name", "name", gekkoName)

	return &Bridge{
		cfg:       cfg,
		gekko:     gekko,
		mqtt:      mqtt,
		fieldDef:  fieldDefinitions,
		gekkoName: gekkoName,
		history:   make(map[string]any),
		ctx:       ctx,
		cancel:    cancel,
	}, nil
}

func (b *Bridge) Stop() {
	b.cancel()
}

func (b *Bridge) RunGetter() {
	slog.Info("Starting getter...")
	if err := b.mqtt.Publish(fmt.Sprintf("%s/getter_online", b.gekkoName), "true"); err != nil {
		slog.Error("Failed to publish getter_online", "error", err)
		os.Exit(6)
	}

	slog.Info("Start MyGekko polling")
	ticker := time.NewTicker(time.Duration(b.cfg.MyGekko.Interval * float64(time.Second)))
	defer ticker.Stop()

	// Poll immediately on start, then on every tick
	round := 0
	poll := func() {
		round++

		// Always poll interval_items
		if len(b.cfg.MyGekko.IntervalItems) > 0 {
			slog.Info("Polling interval items", "items", b.cfg.MyGekko.IntervalItems)
			b.pollCategories(b.cfg.MyGekko.IntervalItems)
		}

		// Poll main_items every N rounds
		if round >= b.cfg.MyGekko.IntervalRounds {
			round = 0
			if len(b.cfg.MyGekko.MainItems) > 0 {
				slog.Info("Polling main items", "items", b.cfg.MyGekko.MainItems)
				b.pollCategories(b.cfg.MyGekko.MainItems)
			}
		}
	}

	// Initial poll immediately
	poll()

	for {
		select {
		case <-b.ctx.Done():
			slog.Info("Getter stopped")
			return
		case <-ticker.C:
			poll()
		}
	}
}

func (b *Bridge) pollCategories(categories []string) {
	for _, category := range categories {
		slog.Debug("category", "category", category)

		status, err := b.gekko.GetStatus([]string{category})
		if err != nil {
			slog.Error("Can't connect MyGekko", "error", err)
			os.Exit(11)
		}

		catData, ok := status[category]
		if !ok {
			slog.Warn("Category not found in response", "category", category)
			continue
		}

		catMap, ok := catData.(map[string]any)
		if !ok {
			continue
		}

		for item, itemData := range catMap {
			if strings.HasPrefix(item, "group") {
				continue
			}

			itemMap, ok := itemData.(map[string]any)
			if !ok {
				continue
			}

			sumstate, ok := itemMap["sumstate"]
			if !ok {
				continue
			}

			b.processItem(category, item, sumstate)
		}

		// Publish timestamp for category
		if err := b.mqtt.Publish(fmt.Sprintf("%s/%s/get/time", b.gekkoName, category), time.Now().Unix()); err != nil {
			slog.Error("Failed to publish timestamp", "category", category, "error", err)
			os.Exit(6)
		}
	}
}

func (b *Bridge) processItem(category, item string, sumstate any) {
	sumstateMap, ok := sumstate.(map[string]any)
	if !ok {
		return
	}

	// Get the semicolon-separated value string
	valueStr, ok := sumstateMap["value"].(string)
	if !ok {
		return
	}

	// Get field definitions for this category
	fields, ok := b.fieldDef[category]
	if !ok {
		slog.Warn("Unknown category", "category", category)
		return
	}

	// Split value string and map to field names
	values := strings.Split(valueStr, ";")
	itemData := make(map[string]any)
	hasChanges := false

	for i, field := range fields {
		if i >= len(values) {
			break
		}

		// Skip fields with empty name (reserved)
		if field.Name == "" || field.Type == "" {
			continue
		}

		rawValue := values[i]
		if rawValue == "" {
			continue
		}

		// Convert value to appropriate type
		var value any
		var err error
		switch field.Type {
		case "int":
			value, err = strconv.Atoi(rawValue)
		case "float":
			value, err = strconv.ParseFloat(rawValue, 64)
		case "string":
			value = rawValue
		default:
			continue
		}

		if err != nil {
			slog.Error("Failed to parse value", "category", category, "item", item, "field", field.Name, "value", rawValue, "error", err)
			os.Exit(5)
		}

		// Add to item data for JSON publish
		itemData[field.Name] = value

		// Check history to avoid duplicate publishes
		histKey := fmt.Sprintf("%s/%s/%s", category, item, field.Name)
		if oldVal, exists := b.history[histKey]; exists && oldVal == value {
			continue
		}
		b.history[histKey] = value
		hasChanges = true

		// Publish individual field to MQTT
		topic := fmt.Sprintf("%s/%s/%s/get/%s", b.gekkoName, category, item, field.Name)
		if err := b.mqtt.Publish(topic, value); err != nil {
			slog.Error("Failed to publish", "topic", topic, "error", err)
			os.Exit(6)
		}
	}

	// Publish JSON with all fields if any value changed
	if hasChanges && len(itemData) > 0 {
		itemData["timestamp"] = time.Now().Unix()
		jsonTopic := fmt.Sprintf("%s/%s/%s/get/json", b.gekkoName, category, item)
		if err := b.mqtt.PublishJSON(jsonTopic, itemData); err != nil {
			slog.Error("Failed to publish JSON", "topic", jsonTopic, "error", err)
			os.Exit(6)
		}
	}
}

func (b *Bridge) RunSetter() {
	slog.Info("Starting setter...")
	if err := b.mqtt.Publish(fmt.Sprintf("%s/setter_online", b.gekkoName), "true"); err != nil {
		slog.Error("Failed to publish setter_online", "error", err)
		os.Exit(6)
	}

	// Subscribe to all set commands (deduplicated)
	allCategories := slices.Concat(b.cfg.MyGekko.IntervalItems, b.cfg.MyGekko.MainItems)
	slices.Sort(allCategories)
	allCategories = slices.Compact(allCategories)
	for _, category := range allCategories {
		topic := fmt.Sprintf("%s/%s/+/set", b.gekkoName, category)
		slog.Info("subscribe", "topic", topic)
		err := b.mqtt.Subscribe(topic, func(t string, payload []byte) {
			b.handleSetCommand(t, payload)
		})
		if err != nil {
			slog.Error("Failed to subscribe", "topic", topic, "error", err)
			os.Exit(7)
		}
	}

	slog.Info("Start MQTT")
	// Wait for shutdown
	<-b.ctx.Done()
	slog.Info("Setter stopped")
}

func (b *Bridge) handleSetCommand(topic string, payload []byte) {
	slog.Info("Incoming message...")

	// Parse topic: {root}/{category}/{item}/set
	parts := strings.Split(topic, "/")
	if len(parts) < 4 {
		slog.Error("Invalid topic format", "topic", topic)
		os.Exit(8)
	}

	// Extract category and item (skip root prefix)
	category := parts[len(parts)-3]
	item := parts[len(parts)-2]
	value := string(payload)

	slog.Info("Write command", "value", value, "category", category, "item", item)

	if err := b.gekko.SetValue(category, item, value); err != nil {
		slog.Error("MyGEKKO command error", "error", err)
		os.Exit(9)
	}
}

// parseFormatField parses a single field from the format string
// e.g. "currentState enum[...]" -> FieldDef{Name: "currentState", Type: "int"}
func parseFormatField(raw string) (FieldDef, error) {
	data := strings.TrimSpace(raw)
	if data == "" {
		return FieldDef{}, nil
	}

	// Split "fieldName type[...]" into name and rest
	parts := strings.SplitN(data, " ", 2)
	if len(parts) != 2 {
		return FieldDef{}, fmt.Errorf("invalid format definition: '%s'", data)
	}

	name := parts[0]
	typeData := parts[1]

	// Extract type name before the bracket: "enum[...]" -> "enum"
	// Must do this BEFORE checking for colon, since brackets can contain colons
	bracketIdx := strings.Index(typeData, "[")
	if bracketIdx == -1 {
		return FieldDef{}, fmt.Errorf("no type bracket found in '%s'", typeData)
	}
	typeName := typeData[:bracketIdx]

	// Handle optional prefix like "#zimmermann:enum" -> extract "enum"
	if _, after, found := strings.Cut(typeName, ":"); found {
		typeName = after
	}

	var fieldType string
	switch typeName {
	case "int", "enum":
		fieldType = "int"
	case "float":
		fieldType = "float"
	case "string":
		fieldType = "string"
	case "null":
		fieldType = ""
	default:
		return FieldDef{}, fmt.Errorf("type %s is not supported", typeName)
	}

	return FieldDef{Name: name, Type: fieldType}, nil
}

// LoadFieldDefinitions loads and parses field definitions from the MyGEKKO API
func LoadFieldDefinitions(gekko *MyGekkoClient) (map[string][]FieldDef, error) {
	slog.Info("Loading field definitions from API...")

	definitions, err := gekko.GetDefinitions()
	if err != nil {
		return nil, fmt.Errorf("failed to get definitions: %w", err)
	}

	result := make(map[string][]FieldDef)

	for category, catData := range definitions {
		catMap, ok := catData.(map[string]any)
		if !ok {
			continue
		}

		// Find first item to get format
		for itemName, itemData := range catMap {
			if !strings.HasPrefix(itemName, "item") {
				continue
			}

			itemMap, ok := itemData.(map[string]any)
			if !ok {
				continue
			}

			sumstate, ok := itemMap["sumstate"].(map[string]any)
			if !ok {
				continue
			}

			formatStr, ok := sumstate["format"].(string)
			if !ok {
				continue
			}

			// Parse the format string (semicolon-separated)
			formatParts := strings.Split(formatStr, ";")
			var fields []FieldDef
			for _, part := range formatParts {
				field, err := parseFormatField(part)
				if err != nil {
					slog.Warn("Failed to parse field", "category", category, "error", err)
					continue
				}
				if field.Name != "" {
					fields = append(fields, field)
				}
			}

			if len(fields) > 0 {
				result[category] = fields
			}

			break // Only need first item per category
		}
	}

	// Print parsed definitions at debug level
	for category, fields := range result {
		slog.Debug(fmt.Sprintf("%s:", category))
		for i, f := range fields {
			slog.Debug(fmt.Sprintf("  %d: %s (%s)", i, f.Name, f.Type))
		}
	}

	return result, nil
}
