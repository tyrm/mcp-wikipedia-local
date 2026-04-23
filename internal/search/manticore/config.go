package manticore

type Config struct {
	DSN       string
	Table     string
	BatchSize int
	EmbedDims int
}
