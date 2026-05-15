package envx

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// Load loads local .env files when they exist without overriding
// environment variables that are already present in the process.
func Load() error {
	candidates := []string{
		".env",
		filepath.Join("backend", ".env"),
		filepath.Join("..", ".env"),
	}

	files := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		abs, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if _, ok := seen[abs]; ok {
			continue
		}
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		files = append(files, candidate)
		seen[abs] = struct{}{}
	}

	if len(files) == 0 {
		return nil
	}
	return godotenv.Load(files...)
}
