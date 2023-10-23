package feeder

import (
    "fmt"
    "github.com/BurntSushi/toml"
    "os"
    "path/filepath"
)

// Toml is a feeder.
// It feeds using a TOML file.
type Toml struct {
    Path string
}

func (f Toml) Feed(structure interface{}) error {
    file, err := os.Open(filepath.Clean(f.Path))
    if err != nil {
        return fmt.Errorf("toml: %v", err)
    }

    if _, err = toml.NewDecoder(file).Decode(structure); err != nil {
        return fmt.Errorf("toml: %v", err)
    }

    return file.Close()
}
