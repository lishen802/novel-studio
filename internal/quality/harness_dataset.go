package quality

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed testdata/creative_quality_v1.json
var creativeQualityV1 []byte

const creativeQualityV1SHA256 = "421645b78a06f4178ff909ae2e8661f4aa39b0a6debb7bb38f5f1b61a3987fbb"

// BuiltinHarnessDatasetV1 返回随应用发布的只读原创创作质量数据集。
func BuiltinHarnessDatasetV1() (HarnessDataset, error) {
	var dataset HarnessDataset
	if err := json.Unmarshal(creativeQualityV1, &dataset); err != nil {
		return HarnessDataset{}, fmt.Errorf("decode built-in harness dataset: %w", err)
	}
	hash, err := harnessDatasetHash(dataset)
	if err != nil {
		return HarnessDataset{}, fmt.Errorf("hash built-in harness dataset: %w", err)
	}
	dataset.SHA256 = hash
	if dataset.Version != "1.0.0" || hash != creativeQualityV1SHA256 {
		return HarnessDataset{}, ErrDatasetHashMismatch
	}
	if err := dataset.Validate(); err != nil {
		return HarnessDataset{}, fmt.Errorf("validate built-in harness dataset: %w", err)
	}
	return dataset, nil
}
