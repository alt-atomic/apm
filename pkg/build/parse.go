package build

import (
	"encoding/json"
	"io"
	"net/http"
	"os"

	"altlinux.space/alt-atomic/apm/pkg/osutils"

	"github.com/goccy/go-yaml"
)

func ParseYamlConfigData(data []byte) (Config, error) {
	return parseConfigData(data, yamlCodec{})
}

func ParseJsonConfigData(data []byte) (Config, error) {
	return parseConfigData(data, jsonCodec{})
}

func parseConfigData(data []byte, codec Codec) (Config, error) {
	var cfg Config
	if err := codec.DecodeStrict(data, &cfg); err != nil {
		return cfg, err
	}
	if err := cfg.checkModules(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// parseConfigEnvData parses env vars; unknown keys stay allowed (lenient).
func parseConfigEnvData(data []byte, codec Codec, version Version) (Envs, error) {
	var envs Envs
	var err error
	switch codec.(type) {
	case jsonCodec:
		err = json.Unmarshal(data, &envs)
	default:
		err = yaml.Unmarshal(data, &envs)
	}
	if err != nil {
		return envs, err
	}

	resolved, err := ResolveExprMap(envs.Env, ExprData{
		Env:     osutils.GetEnvMap(),
		Version: version,
	})
	if err != nil {
		return Envs{}, err
	}
	envs.Env = resolved

	return envs, nil
}

// ReadAndParseConfig reads a config from a file or URL, codec by extension.
func ReadAndParseConfig(target string) (Config, error) {
	data, err := readSource(target)
	if err != nil {
		return Config{}, err
	}
	return parseConfigData(data, codecForPath(target))
}

// ReadAndParseConfigEnv reads env vars from a file or URL, codec by extension.
func ReadAndParseConfigEnv(target string, version Version) (Envs, error) {
	data, err := readSource(target)
	if err != nil {
		return Envs{}, err
	}
	return parseConfigEnvData(data, codecForPath(target), version)
}

// ReadAndParseModules reads modules from a file or URL, codec by extension.
func ReadAndParseModules(target string) (*[]Module, error) {
	data, err := readSource(target)
	if err != nil {
		return nil, err
	}
	cfg, err := parseConfigData(data, codecForPath(target))
	if err != nil {
		return nil, err
	}
	return &cfg.Modules, nil
}

// readSource reads raw bytes from a local file or an HTTP(S) URL.
func readSource(target string) ([]byte, error) {
	if osutils.IsURL(target) {
		resp, err := http.Get(target)
		if err != nil {
			return nil, err
		}
		defer func() { _ = resp.Body.Close() }()
		return io.ReadAll(resp.Body)
	}
	return os.ReadFile(target)
}
