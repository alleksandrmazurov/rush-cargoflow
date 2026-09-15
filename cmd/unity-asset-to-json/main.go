package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/fogleman/rush"
)

// Converts Unity LevelDefinition .asset YAML into Cargo Flow JSON schema v1.
// Development helper — mirrors Editor exporter field mapping.

var (
	reLevelID    = regexp.MustCompile(`^\s*LevelId:\s*(.+)\s*$`)
	reWidth      = regexp.MustCompile(`^\s*Width:\s*(\d+)\s*$`)
	reHeight     = regexp.MustCompile(`^\s*Height:\s*(\d+)\s*$`)
	reExit       = regexp.MustCompile(`^\s*ExitColumn:\s*(\d+)\s*$`)
	reObjID      = regexp.MustCompile(`^\s*-\s*Id:\s*(.+)\s*$`)
	reType       = regexp.MustCompile(`^\s*Type:\s*(\d+)\s*$`)
	reGridX      = regexp.MustCompile(`^\s*GridX:\s*(-?\d+)\s*$`)
	reGridY      = regexp.MustCompile(`^\s*GridY:\s*(-?\d+)\s*$`)
	reObjW       = regexp.MustCompile(`^\s*Width:\s*(\d+)\s*$`)
	reObjH       = regexp.MustCompile(`^\s*Height:\s*(\d+)\s*$`)
)

// Unity CargoObjectType enum ordinals.
const (
	unityTarget = 0
	unity1x1    = 1
	unity1x2    = 2 // vertical
	unity2x1    = 3 // horizontal
	unity1x3    = 4
	unity3x1    = 5
	unityStatic = 6
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "usage: %s <Level_XX.asset> <out.json> [canonicalGestures]\n", filepath.Base(os.Args[0]))
		os.Exit(2)
	}
	inPath, outPath := os.Args[1], os.Args[2]
	var canon *int
	if len(os.Args) >= 4 {
		v, err := strconv.Atoi(os.Args[3])
		if err != nil {
			fatal(err)
		}
		canon = &v
	}
	level, err := parseUnityLevelAsset(inPath)
	if err != nil {
		fatal(err)
	}
	level.CanonicalGestures = canon
	level.Source = "unity-asset:" + filepath.Base(inPath)
	if err := level.Validate(); err != nil {
		fatal(err)
	}
	if err := rush.WriteJSONFile(outPath, level); err != nil {
		fatal(err)
	}
	fmt.Printf("wrote %s (%d pieces) exitCol=%d\n", outPath, len(level.Pieces), level.Exit.Column)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func parseUnityLevelAsset(path string) (*rush.LevelJSON, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	level := &rush.LevelJSON{
		SchemaVersion:   rush.CargoFlowJSONSchemaVersion,
		CoordinateSpace: rush.CoordinateSpaceUnity,
		Exit:            rush.ExitJSON{Side: "top"},
	}

	sc := bufio.NewScanner(f)
	inObjects := false
	var cur *rush.PieceJSON
	flush := func() {
		if cur == nil {
			return
		}
		level.Pieces = append(level.Pieces, *cur)
		cur = nil
	}

	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "Objects:") {
			inObjects = true
			continue
		}
		if !inObjects {
			if m := reLevelID.FindStringSubmatch(line); m != nil {
				level.LevelID = strings.TrimSpace(m[1])
			} else if m := reWidth.FindStringSubmatch(line); m != nil {
				level.Width, _ = strconv.Atoi(m[1])
			} else if m := reHeight.FindStringSubmatch(line); m != nil {
				level.Height, _ = strconv.Atoi(m[1])
			} else if m := reExit.FindStringSubmatch(line); m != nil {
				level.Exit.Column, _ = strconv.Atoi(m[1])
			}
			continue
		}

		if m := reObjID.FindStringSubmatch(line); m != nil {
			flush()
			cur = &rush.PieceJSON{ID: strings.TrimSpace(m[1]), Movable: true}
			continue
		}
		if cur == nil {
			continue
		}
		if m := reType.FindStringSubmatch(line); m != nil {
			t, _ := strconv.Atoi(m[1])
			cur.Type, cur.Movable = unityTypeToJSON(t)
		} else if m := reGridX.FindStringSubmatch(line); m != nil {
			cur.X, _ = strconv.Atoi(m[1])
		} else if m := reGridY.FindStringSubmatch(line); m != nil {
			cur.Y, _ = strconv.Atoi(m[1])
		} else if m := reObjW.FindStringSubmatch(line); m != nil {
			// Inside object block, Width/Height are footprint (not board).
			cur.Width, _ = strconv.Atoi(m[1])
		} else if m := reObjH.FindStringSubmatch(line); m != nil {
			cur.Height, _ = strconv.Atoi(m[1])
		}
	}
	flush()
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return level, nil
}

func unityTypeToJSON(t int) (string, bool) {
	switch t {
	case unityTarget:
		return "target", true
	case unity1x1:
		return "movable1x1", true
	case unity1x2, unity2x1:
		return "movable1x2", true
	case unity1x3, unity3x1:
		return "movable1x3", true
	case unityStatic:
		return "static1x1", false
	default:
		return fmt.Sprintf("unknown_%d", t), true
	}
}
