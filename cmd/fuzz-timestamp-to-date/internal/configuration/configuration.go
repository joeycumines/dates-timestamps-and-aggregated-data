package configuration

import (
	"encoding/base64"
	"encoding/json"
	"errors"
)

const Variable = `github.com/joeycumines/dates-timestamps-and-aggregated-data/cmd/fuzz-timestamp-to-date/internal/configuration.optionsBase64`

type Options struct {
	Cmd  string   `json:"cmd"`
	Args []string `json:"args"`
	Dir  string   `json:"dir"`
}

var optionsBase64 string

func Skip() bool {
	return optionsBase64 == ``
}

func Encode(options Options) (string, error) {
	if options.Cmd == "" {
		return ``, errors.New("options.Cmd is empty")
	}
	b, err := json.Marshal(options)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func Decode() (options Options, err error) {
	if optionsBase64 == "" {
		err = errors.New("optionsBase64 is empty")
		return
	}
	b, err := base64.StdEncoding.DecodeString(optionsBase64)
	if err != nil {
		return
	}
	err = json.Unmarshal(b, &options)
	if options.Cmd == "" {
		err = errors.New("options.Cmd is empty")
		return
	}
	return
}
