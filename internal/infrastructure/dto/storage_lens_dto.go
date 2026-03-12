package dto

type StorageLensOutput struct {
	Summary        StorageLensSummary         `json:"summary"`
	TimeSeries     []StorageLensSnapshotDTO   `json:"timeSeries"`
	StorageClasses []StorageClassDistribution `json:"storageClasses"`
	TopBuckets     []TopBucketInfo            `json:"topBuckets"`
}

type StorageLensSummary struct {
	TotalStorage  int64   `json:"totalStorage"`
	ObjectCount   int64   `json:"objectCount"`
	ActiveBuckets int     `json:"activeBuckets"`
	AvgObjectSize int64   `json:"avgObjectSize"`
	Growth        float64 `json:"growth"`
}

type StorageLensSnapshotDTO struct {
	Date    string `json:"date"`
	Storage int64  `json:"storage"`
}

type StorageClassDistribution struct {
	Name       string  `json:"name"`
	Size       int64   `json:"size"`
	Percentage float64 `json:"percentage"`
	Color      string  `json:"color"`
}

type TopBucketInfo struct {
	Name        string  `json:"name"`
	Region      string  `json:"region"`
	Size        int64   `json:"size"`
	ObjectCount int64   `json:"objectCount"`
	Growth      float64 `json:"growth"`
}
