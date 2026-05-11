package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "ghtop", "pins.json")
}

func loadPins() ([]Pin, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return []Pin{}, err
	}
	var pins []Pin
	return pins, json.Unmarshal(data, &pins)
}

func savePins(pins []Pin) error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(pins, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
