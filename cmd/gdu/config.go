package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/dundee/gdu/v6/cmd/gdu/app"
)

var systemConfigPath = "/etc/gdu.yaml"

var (
	// configWarnings are what gdu tells about the config files it read
	// anyway, such as unknown keys
	configWarnings []string
	// baseline is the configuration before the user's config file and
	// command line: the defaults and the system config file
	baseline *yaml.Node
	// userConfig is the user's config file as read, nil when there is none
	userConfig *yaml.Node
)

// loadConfig applies the config file at path to af, and returns the file. A
// file which does not parse changes nothing. Unknown keys are ignored, and
// told as warnings.
func loadConfig(path string) (*yaml.Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// into a copy of the struct, not the pointer: a document that is just
	// null would set the pointer to nil, and a broken one apply in part
	next := *af
	if err := yaml.Unmarshal(data, &next); err != nil {
		return nil, err
	}
	*af = next

	if unknown := unknownKeys(data); len(unknown) > 0 {
		configWarnings = append(configWarnings,
			fmt.Sprintf("%s: ignoring unknown settings: %s", path, strings.Join(unknown, ", ")))
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

var unknownField = regexp.MustCompile(`line (\d+): field (\S+) not found in type`)

// unknownKeys returns the keys of the config file data which are no
// setting, such as a misspelt "no-delte", with their lines.
func unknownKeys(data []byte) []string {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var typeErr *yaml.TypeError
	if err := decoder.Decode(&app.Flags{}); !errors.As(err, &typeErr) {
		return nil
	}
	var unknown []string
	for _, msg := range typeErr.Errors {
		if m := unknownField.FindStringSubmatch(msg); m != nil {
			unknown = append(unknown, fmt.Sprintf("%s (line %s)", m[2], m[1]))
		}
	}
	return unknown
}

func initConfig() {
	// Load system-wide config first (ignored on Windows/Plan9 and when absent).
	if runtime.GOOS != osWindows && runtime.GOOS != osPlan9 {
		if _, err := loadConfig(systemConfigPath); err != nil && !os.IsNotExist(err) {
			configErr = errors.Join(configErr, fmt.Errorf("%s: %w", systemConfigPath, err))
		}
	}
	baseline = configNode(af)

	// Load user config; its values overwrite whatever the system config set.
	// A file named on the command line must exist.
	explicit := setConfigFilePath()
	doc, err := loadConfig(af.CfgFile)
	if err != nil {
		if explicit || !os.IsNotExist(err) {
			configErr = errors.Join(configErr, fmt.Errorf("%s: %w", af.CfgFile, err))
		}
		return
	}
	userConfig = doc
}

func setDefaults() {
	setStyleDefaults(af)
}

// setStyleDefaults gives the colours of the terminal UI not configured their
// defaults.
func setStyleDefaults(f *app.Flags) {
	for _, c := range []struct {
		color *string
		value string
	}{
		{&f.Style.Footer.BackgroundColor, "#2479D0"},
		{&f.Style.Footer.TextColor, "#000000"},
		{&f.Style.Footer.NumberColor, "#FFFFFF"},
		{&f.Style.Header.BackgroundColor, "#2479D0"},
		{&f.Style.Header.TextColor, "#000000"},
		{&f.Style.ResultRow.NumberColor, "#e67100"},
		{&f.Style.ResultRow.DirectoryColor, "#3498db"},
	} {
		if *c.color == "" {
			*c.color = c.value
		}
	}
}

// setConfigFilePath sets the user config file, and reports whether it was
// named on the command line.
func setConfigFilePath() (explicit bool) {
	// Read the arguments one by one instead of joining them, so that paths
	// containing spaces are not truncated.
	for i, arg := range os.Args {
		if value, found := strings.CutPrefix(arg, "--config-file="); found {
			af.CfgFile = value
			return true
		}
		if arg == "--config-file" && i+1 < len(os.Args) {
			af.CfgFile = os.Args[i+1]
			return true
		}
	}
	setDefaultConfigFilePath()
	return false
}

func setDefaultConfigFilePath() {
	home, err := os.UserHomeDir()
	if err != nil {
		configErr = errors.Join(configErr, err)
		return
	}

	path := filepath.Join(home, ".config", "gdu", "gdu.yaml")
	if _, err := os.Stat(path); err == nil {
		af.CfgFile = path
		return
	}

	af.CfgFile = filepath.Join(home, ".gdu.yaml")
}

// reportConfigError tells w about the config files: the ones which could not
// be read, gdu then running with the settings read before, and what was
// ignored in the others. It used to go to the log only, which by default is
// /dev/null.
func reportConfigError(w io.Writer) {
	if configErr != nil {
		slog.Error("reading config file", "err", configErr)
		for _, err := range unjoin(configErr) {
			fmt.Fprintf(w, "gdu: ignoring the config file: %s\n", err)
		}
	}
	for _, warning := range configWarnings {
		slog.Warn(warning)
		fmt.Fprintf(w, "gdu: %s\n", warning)
	}
}

// unjoin returns the errors joined in err.
func unjoin(err error) []error {
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		return joined.Unwrap()
	}
	return []error{err}
}

// oneOffSettings are what applies to a single run; given on the command line,
// --write-config does not save them.
var oneOffSettings = map[string]bool{
	"input-file":        true,
	"output-file":       true,
	"output-attrs":      true,
	"read-from-storage": true,
	"db":                true,
	"since":             true,
	"until":             true,
	"max-age":           true,
	"min-age":           true,
}

// writeConfig writes what the user configured to the user config file: the
// settings of the config file, and the command line's changes to the defaults
// and the system config, except its one-off settings.
func writeConfig() error {
	data, err := yaml.Marshal(configToSave(configNode(af), baseline, userConfig))
	if err != nil {
		return fmt.Errorf("error marshaling config file: %w", err)
	}
	if af.CfgFile == "" {
		setDefaultConfigFilePath()
	}
	if err := os.WriteFile(af.CfgFile, data, 0o600); err != nil {
		return fmt.Errorf("error writing config file %s: %w", af.CfgFile, err)
	}
	return nil
}

// configNode returns the settings of f as a YAML mapping, with the default
// colours.
func configNode(f *app.Flags) *yaml.Node {
	settings := *f
	setStyleDefaults(&settings)
	var doc yaml.Node
	if err := doc.Encode(&settings); err != nil {
		panic(err) // the settings are plain data
	}
	return &doc
}

// configToSave returns the settings of current which differ from baseline or
// are in the user's config file user, leaving out the one-off settings not in
// it.
func configToSave(current, baseline, user *yaml.Node) *yaml.Node {
	kept := keep(mapping(current), mapping(baseline), mapping(user), true)
	if kept == nil {
		return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	}
	return kept
}

// keep returns the mapping current without the entries equal to those of
// baseline and absent from user, or nil when nothing is left.
func keep(current, baseline, user *yaml.Node, top bool) *yaml.Node {
	kept := &yaml.Node{Kind: yaml.MappingNode, Tag: current.Tag, Style: current.Style}
	for i := 0; i+1 < len(current.Content); i += 2 {
		key, value := current.Content[i], current.Content[i+1]
		inUser := user != nil && entry(user, key.Value) != nil
		if top && oneOffSettings[key.Value] && !inUser {
			continue
		}
		if value.Kind == yaml.MappingNode {
			if sub := keep(value, entry(baseline, key.Value), entry(user, key.Value), false); sub != nil {
				kept.Content = append(kept.Content, key, sub)
			}
			continue
		}
		if inUser || !sameValue(value, entry(baseline, key.Value)) {
			kept.Content = append(kept.Content, key, value)
		}
	}
	if len(kept.Content) == 0 {
		return nil
	}
	return kept
}

// mapping returns the mapping of the document doc.
func mapping(doc *yaml.Node) *yaml.Node {
	if doc != nil && doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0]
	}
	if doc != nil && doc.Kind == yaml.MappingNode {
		return doc
	}
	return nil
}

// entry returns the value of key in the mapping m, or nil.
func entry(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func sameValue(a, b *yaml.Node) bool {
	if b == nil {
		return false
	}
	var va, vb any
	if a.Decode(&va) != nil || b.Decode(&vb) != nil {
		return false
	}
	return reflect.DeepEqual(va, vb)
}
