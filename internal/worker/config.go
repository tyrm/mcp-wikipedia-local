package worker

type Config struct {
	NumWorkers     int
	BatchSize      int
	CheckpointFile string
}
