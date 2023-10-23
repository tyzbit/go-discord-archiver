package feeder

import (
    "fmt"
    "gopkg.in/yaml.v3"
    "os"
    "path/filepath"
)

// Yaml is a feeder.
// It feeds using a YAML file.
type Yaml struct {
    Path string
}

func (f Yaml) Feed(structure interface{}) error {
    file, err := os.Open(filepath.Clean(f.Path))
    if err != nil {
        return fmt.Errorf("yaml: %v", err)
    }

    if err = yaml.NewDecoder(file).Decode(structure); err != nil {
        return fmt.Errorf("yaml: %v", err)
    }

    return file.Close()
}
