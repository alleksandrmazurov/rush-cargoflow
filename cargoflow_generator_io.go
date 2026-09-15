package rush

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteGenerationBatch writes accepted candidates, solutions, and BatchManifest.json.
func WriteGenerationBatch(dir string, result CargoFlowGenerationResult) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, c := range result.Accepted {
		levelPath := filepath.Join(dir, c.CandidateID+".json")
		solPath := filepath.Join(dir, c.CandidateID+".solution.json")
		if err := WriteJSONFile(levelPath, c.Level); err != nil {
			return fmt.Errorf("write level: %w", err)
		}
		if err := WriteJSONFile(solPath, c.SolutionDoc); err != nil {
			return fmt.Errorf("write solution: %w", err)
		}
	}
	manifest := result.ToManifest()
	return WriteJSONFile(filepath.Join(dir, "BatchManifest.json"), manifest)
}
