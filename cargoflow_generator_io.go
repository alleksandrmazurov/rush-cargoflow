package rush

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteGenerationBatch writes accepted candidates, solutions, signatures, and BatchManifest.json.
func WriteGenerationBatch(dir string, result CargoFlowGenerationResult) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, c := range result.Accepted {
		levelPath := filepath.Join(dir, c.CandidateID+".json")
		solPath := filepath.Join(dir, c.CandidateID+".solution.json")
		sigPath := filepath.Join(dir, c.CandidateID+".signature.json")
		if err := WriteJSONFile(levelPath, c.Level); err != nil {
			return fmt.Errorf("write level: %w", err)
		}
		if err := WriteJSONFile(solPath, c.SolutionDoc); err != nil {
			return fmt.Errorf("write solution: %w", err)
		}
		if err := WriteJSONFile(sigPath, c.Signature); err != nil {
			return fmt.Errorf("write signature: %w", err)
		}
	}
	manifest := result.ToManifest()
	return WriteJSONFile(filepath.Join(dir, "BatchManifest.json"), manifest)
}
