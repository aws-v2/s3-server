package dto

type SecuritySummaryOutput struct {
	Score              int               `json:"score"`
	PublicBucketsCount int               `json:"publicBucketsCount"`
	UnencryptedCount   int               `json:"unencryptedCount"`
	MfaMissingCount    int               `json:"mfaMissingCount"`
	Findings           []SecurityFinding `json:"findings"`
}

type SecurityFinding struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"` // critical, high, medium, low
	BucketName  string `json:"bucket_name"`
	Description string `json:"description"`
	Remediation string `json:"remediation"`
}
