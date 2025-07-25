// config/save.go
package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

func SaveConfig(filename string) error {
	data, err := yaml.Marshal(&Settings)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}
