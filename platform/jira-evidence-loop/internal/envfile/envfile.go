package envfile

import (
	"os"
	"strings"
)

// Parse turns raw .env bytes into a key→value map: skips blank lines and lines
// starting with '#', splits each remaining line on the first '=', and trims
// surrounding whitespace from key and value. It never touches the environment.
func Parse(b []byte) map[string]string {
	result := map[string]string{}
	lines := strings.Split(string(b), "\n")
	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}
		result[key] = val
	}
	return result
}

// LoadInto reads each path in order and, for every parsed key whose getenv
// returns "", calls setenv with the parsed value. Real environment and earlier
// paths win (if-unset semantics). Missing files (os.IsNotExist) are skipped;
// any other read error is returned.
func LoadInto(setenv func(k, v string) error, getenv func(k string) string, paths ...string) error {
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		for k, v := range Parse(b) {
			if getenv(k) == "" {
				if err := setenv(k, v); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
