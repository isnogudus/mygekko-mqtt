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

	// Incoming set commands are queued here so the MQTT receive loop never
	// blocks on the (synchronous, potentially slow) MyGEKKO HTTP call. A
	// single worker drains the queues, which serializes commands and spaces
	// throttled ones by at least cmdInterval — MyGEKKO is single-threaded and
	// drops/crashes on commands that arrive too quickly after one another.
	//
	// immediateQueue has priority and bypasses the throttle: its commands
	// (e.g. a blind STOP) are sent as soon as possible, even preempting an
	// active throttle wait.
	cmdQueue         chan setCommand
	immediateQueue   chan setCommand
	cmdInterval      time.Duration
	throttlePrefixes map[string][]string // category -> payload prefixes that are throttled
}

type setCommand struct {
	topic   string
	payload []byte
}

func NewBridge(cfg *Config, gekko GekkoClient, mqtt MQTTPublisher, fieldDefinitions map[string][]FieldDef, gekkoName string) (*Bridge, error) {
	ctx, cancel := context.WithCancel(context.Background())

	return &Bridge{
		cfg:              cfg,
		gekko:            gekko,
		mqtt:             mqtt,
		fieldDef:         fieldDefinitions,
		gekkoName:        gekkoName,
		history:          make(map[string]any),
		ctx:              ctx,
		cancel:           cancel,
		cmdQueue:         make(chan setCommand, 256),
		immediateQueue:   make(chan setCommand, 64),
		cmdInterval:      time.Duration(cfg.MyGekko.CommandInterval * float64(time.Second)),
		throttlePrefixes: cfg.MyGekko.ThrottlePrefixes,
	}, nil
}

func (b *Bridge) Stop() {
	b.cancel()
}

func (b *Bridge) RunGetter() {
	slog.Info("Starting getter...")
	ticker := time.NewTicker(time.Duration(b.cfg.MyGekko.Interval * float64(time.Second)))
	defer ticker.Stop()

	// Poll immediately on start, then on every tick
	// Start at IntervalRounds so first poll() fetches everything
	round := b.cfg.MyGekko.IntervalRounds
	poll := func() {
		round++

		// Always poll interval_items
		if len(b.cfg.MyGekko.IntervalItems) > 0 {
			slog.Debug("Polling interval items", "items", b.cfg.MyGekko.IntervalItems)
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
		if err := b.mqtt.Publish(fmt.Sprintf("%s/get/time", category), time.Now().Unix()); err != nil {
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
		topic := fmt.Sprintf("%s/%s/get/%s", category, item, field.Name)
		if err := b.mqtt.Publish(topic, value); err != nil {
			slog.Error("Failed to publish", "topic", topic, "error", err)
			os.Exit(6)
		}
	}

	// Publish JSON with all fields if any value changed
	if hasChanges && len(itemData) > 0 {
		itemData["timestamp"] = time.Now().Unix()
		jsonTopic := fmt.Sprintf("%s/%s/get/json", category, item)
		if err := b.mqtt.PublishJSON(jsonTopic, itemData); err != nil {
			slog.Error("Failed to publish JSON", "topic", jsonTopic, "error", err)
			os.Exit(6)
		}
	}
}

func (b *Bridge) RunSetter() {
	slog.Info("Starting setter...")

	// Drain the command queue in a dedicated goroutine so the MQTT receive
	// loop is never blocked by a slow MyGEKKO request.
	go b.runCommandWorker()

	// Subscribe to all set commands for all known categories
	allCategories := make([]string, 0, len(b.fieldDef))
	for category := range b.fieldDef {
		allCategories = append(allCategories, category)
	}
	slices.Sort(allCategories)
	for _, category := range allCategories {
		topic := fmt.Sprintf("%s/+/set", category)
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

// categoryFromTopic extracts the category from a set topic
// ({root}/{category}/{item}/set), or "" if the topic is malformed.
func categoryFromTopic(topic string) string {
	parts := strings.Split(topic, "/")
	if len(parts) < 4 {
		return ""
	}
	return parts[len(parts)-3]
}

// isThrottled reports whether a command must be spaced by cmdInterval (true) or
// may be sent immediately (false). Categories without a throttle rule are
// throttled entirely; for a category with a rule, only payloads starting with
// one of its prefixes are throttled (e.g. blinds "P50"), the rest are immediate.
func (b *Bridge) isThrottled(category, value string) bool {
	prefixes, ok := b.throttlePrefixes[category]
	if !ok {
		return true
	}
	for _, p := range prefixes {
		if strings.HasPrefix(value, p) {
			return true
		}
	}
	return false
}

// handleSetCommand is the MQTT receive callback. It must not block, so it only
// copies the message and hands it to the command worker via the matching queue.
func (b *Bridge) handleSetCommand(topic string, payload []byte) {
	slog.Info("Incoming message...", "topic", topic)

	// paho may reuse the payload buffer after this callback returns, so copy it.
	p := make([]byte, len(payload))
	copy(p, payload)

	cmd := setCommand{topic: topic, payload: p}

	queue := b.cmdQueue
	if !b.isThrottled(categoryFromTopic(topic), string(p)) {
		slog.Debug("Queuing immediate command", "topic", topic)
		queue = b.immediateQueue
	}

	select {
	case queue <- cmd:
	case <-b.ctx.Done():
	}
}

// runCommandWorker drains the command queues, sending one command at a time to
// MyGEKKO. Immediate commands are sent as soon as possible; throttled commands
// are spaced by at least cmdInterval. An immediate command preempts an active
// throttle wait.
func (b *Bridge) runCommandWorker() {
	slog.Info("Starting command worker...")
	var last time.Time

	send := func(cmd setCommand) {
		b.processSetCommand(cmd.topic, cmd.payload)
		last = time.Now()
	}

	for {
		// Highest priority: a pending immediate command, sent without delay.
		select {
		case <-b.ctx.Done():
			slog.Info("Command worker stopped")
			return
		case cmd := <-b.immediateQueue:
			send(cmd)
			continue
		default:
		}

		// How long before the next throttled command may be sent.
		var delay time.Duration
		if b.cmdInterval > 0 {
			delay = b.cmdInterval - time.Since(last)
		}

		if delay <= 0 {
			// No throttle pending: send the next ready command, still
			// preferring immediate ones.
			select {
			case <-b.ctx.Done():
				slog.Info("Command worker stopped")
				return
			case cmd := <-b.immediateQueue:
				send(cmd)
			case cmd := <-b.cmdQueue:
				send(cmd)
			}
			continue
		}

		// Throttle active: wait out the interval, but let an immediate command
		// preempt the wait.
		slog.Debug("Throttling set commands", "wait", delay)
		timer := time.NewTimer(delay)
		select {
		case <-b.ctx.Done():
			timer.Stop()
			slog.Info("Command worker stopped")
			return
		case cmd := <-b.immediateQueue:
			timer.Stop()
			slog.Debug("Immediate command preempts throttle", "topic", cmd.topic)
			send(cmd)
		case <-timer.C:
			// Interval elapsed; loop to dispatch the next command.
		}
	}
}

func (b *Bridge) processSetCommand(topic string, payload []byte) {
	// Parse topic: {root}/{category}/{item}/set
	parts := strings.Split(topic, "/")
	if len(parts) < 4 {
		slog.Error("Invalid topic format", "topic", topic)
		return
	}

	// Extract category and item (skip root prefix)
	category := parts[len(parts)-3]
	item := parts[len(parts)-2]
	value := string(payload)

	slog.Info("Write command", "value", value, "category", category, "item", item)

	// A failed command must not take down the bridge: that would also drop all
	// other commands still queued behind it. Log it and carry on.
	if err := b.gekko.SetValue(category, item, value); err != nil {
		slog.Error("MyGEKKO command error", "error", err, "category", category, "item", item, "value", value)
		return
	}
	slog.Debug("Command ok", "category", category, "item", item, "value", value)
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
