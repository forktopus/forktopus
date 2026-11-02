package model

// AppUpdate — снимок операции. URL пакета, хеш и пути остаются внутри Go;
// revision позволяет UI отбрасывать запоздалые ответы и события.
type AppUpdate struct {
	Revision       uint64 `json:"revision"`
	Status         string `json:"status"`
	CurrentVersion string `json:"currentVersion"`
	Version        string `json:"version"`
	Format         string `json:"format"`
	Filename       string `json:"filename"`
	Size           int64  `json:"size"`
	Downloaded     int64  `json:"downloaded"`
	ReleaseURL     string `json:"releaseUrl"`
	CheckedAt      string `json:"checkedAt"`
	Message        string `json:"message"`
	CanDownload    bool   `json:"canDownload"`
	Instruction    string `json:"instruction"`
}
