package runtime

type Workload struct {
	ID               string
	Image            string
	Command          []string
	Entrypoint       []string
	Env              map[string]string
	WorkingDir       string
	ContainerdNs     string
	Status           string
	RestartPolicy    string
	OverlayIP        string
	RestartCount     int32
	CpuLimit         float64
	CpuShares        int32
	MemoryLimitMiB   int32
	ImageDigest      string
	LastFailedAtUnix int64
	ErrorMessage     string
}
