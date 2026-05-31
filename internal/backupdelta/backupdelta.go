package backupdelta

import (
	"encoding/base64"
	"fmt"

	"github.com/sergi/go-diff/diffmatchpatch"
)

func EncodeDelta(base, next []byte) ([]byte, error) {
	dmp := diffmatchpatch.New()
	baseText := base64.StdEncoding.EncodeToString(base)
	nextText := base64.StdEncoding.EncodeToString(next)

	diffs := dmp.DiffMain(baseText, nextText, false)
	patches := dmp.PatchMake(baseText, diffs)
	patchText := dmp.PatchToText(patches)
	return []byte(patchText), nil
}

func ApplyDelta(base []byte, patchBytes []byte) ([]byte, error) {
	dmp := diffmatchpatch.New()
	baseText := base64.StdEncoding.EncodeToString(base)

	patches, err := dmp.PatchFromText(string(patchBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to parse delta patch: %w", err)
	}

	patched, results := dmp.PatchApply(patches, baseText)
	for _, ok := range results {
		if !ok {
			return nil, fmt.Errorf("delta patch did not apply cleanly")
		}
	}

	out, err := base64.StdEncoding.DecodeString(patched)
	if err != nil {
		return nil, fmt.Errorf("failed to decode patched backup: %w", err)
	}
	return out, nil
}
