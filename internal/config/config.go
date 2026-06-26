package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"bft/internal/model"
)

func Load(path string) (model.NodeConfig, error) {
	var cfg model.NodeConfig

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if err := validate(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func validate(cfg model.NodeConfig) error {
	if cfg.ID == "" {
		return fmt.Errorf("id is required")
	}
	if cfg.Address == "" {
		return fmt.Errorf("address is required")
	}
	if cfg.Byzantine {
		switch cfg.Behavior {
		case "", model.BehaviorSilent, model.BehaviorConflictingValue:
		default:
			return fmt.Errorf("unsupported byzantine behavior %q", cfg.Behavior)
		}
	}
	return nil
}

func LoadDir(dir string) ([]model.NodeConfig, error) {
	pattern := filepath.Join(dir, "*.json")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, fmt.Errorf("no config files found in %s", dir)
	}

	configs := make([]model.NodeConfig, 0, len(paths))
	for _, path := range paths {
		cfg, err := Load(path)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", path, err)
		}
		configs = append(configs, cfg)
	}
	return configs, nil
}
