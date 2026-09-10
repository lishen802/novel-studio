package storyasset

import "time"

// Revision 保存正式资产某一版本的完整规范化载荷及审计信息。
type Revision struct {
	AssetID       string
	Revision      int64
	PayloadJSON   string
	ContentSHA256 string
	CreatedBy     string
	CreatedAt     time.Time
}
